package model

import (
	"fmt"
	"path"
	"strings"
)

const (
	SearchPlanVersion        = 1
	DefaultSearchLimit       = 10
	MaxSearchLimit           = 50
	DefaultRelationshipDepth = 1
	MaxRelationshipDepth     = 2
)

type SearchPlanV1 struct {
	Version           int      `json:"version"`
	Question          string   `json:"question,omitempty"`
	Terms             []string `json:"terms,omitempty"`
	Phrases           []string `json:"phrases,omitempty"`
	Symbols           []string `json:"symbols,omitempty"`
	Paths             []string `json:"paths,omitempty"`
	Globs             []string `json:"globs,omitempty"`
	Languages         []string `json:"languages,omitempty"`
	Shards            []string `json:"shards,omitempty"`
	IncludeTests      *bool    `json:"include_tests,omitempty"`
	RelationshipDepth int      `json:"relationship_depth,omitempty"`
	Limit             int      `json:"limit,omitempty"`
}

func CompileRawSearchPlan(query string, limit int, shards ...string) (SearchPlanV1, error) {
	plan := SearchPlanV1{
		Version:  SearchPlanVersion,
		Question: strings.TrimSpace(query),
		Terms:    QueryTerms(query),
		Shards:   shards,
		Limit:    limit,
	}
	return plan.Normalize()
}

func (p SearchPlanV1) Normalize() (SearchPlanV1, error) {
	p.Question = strings.TrimSpace(p.Question)
	if p.Version != SearchPlanVersion {
		return SearchPlanV1{}, fmt.Errorf("unsupported search plan version %d", p.Version)
	}

	p.Terms = cleanList(p.Terms, strings.ToLower)
	p.Phrases = cleanList(p.Phrases, nil)
	p.Symbols = cleanList(p.Symbols, nil)
	p.Languages = cleanList(p.Languages, strings.ToLower)

	var err error
	if p.Paths, err = cleanPathList("path", p.Paths); err != nil {
		return SearchPlanV1{}, err
	}
	if p.Globs, err = cleanPathList("glob", p.Globs); err != nil {
		return SearchPlanV1{}, err
	}
	if p.Shards, err = cleanShardList(p.Shards); err != nil {
		return SearchPlanV1{}, err
	}

	if p.RelationshipDepth == 0 {
		p.RelationshipDepth = DefaultRelationshipDepth
	}
	if p.RelationshipDepth < 0 || p.RelationshipDepth > MaxRelationshipDepth {
		return SearchPlanV1{}, fmt.Errorf("relationship_depth must be between 0 and %d", MaxRelationshipDepth)
	}
	if p.Limit <= 0 {
		p.Limit = DefaultSearchLimit
	}
	if p.Limit > MaxSearchLimit {
		p.Limit = MaxSearchLimit
	}
	return p, nil
}

func (p SearchPlanV1) Validate() error {
	_, err := p.Normalize()
	return err
}

func cleanList(values []string, normalize func(string) string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if normalize != nil {
			value = normalize(value)
		}
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func cleanPathList(kind string, values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if err := validateSlashRelative(kind, value); err != nil {
			return nil, err
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func validateSlashRelative(kind, value string) error {
	if strings.HasPrefix(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, ":") {
		return fmt.Errorf("%s %q must be slash-relative", kind, value)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("%s %q must not contain empty, current, or parent path segments", kind, value)
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return fmt.Errorf("%s %q must not traverse parents", kind, value)
	}
	if kind == "glob" {
		if _, err := path.Match(value, "placeholder"); err != nil {
			return fmt.Errorf("glob %q is invalid: %w", value, err)
		}
	}
	return nil
}

func cleanShardList(values []string) ([]string, error) {
	known := make(map[string]struct{}, len(KnownShardNames()))
	for _, name := range KnownShardNames() {
		known[name] = struct{}{}
	}

	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = normalizeShardName(value)
		if value == "" {
			continue
		}
		if _, ok := known[value]; !ok {
			return nil, fmt.Errorf("unknown repo-map shard %q", value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func normalizeShardName(shard string) string {
	shard = strings.TrimSpace(strings.ToLower(shard))
	if shard == "" || strings.HasSuffix(shard, ".jsonl") {
		return shard
	}
	return shard + ".jsonl"
}
