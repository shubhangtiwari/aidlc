package model

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

const (
	DefaultExactSearchMaxResults = 20
	DefaultExactSearchMaxBytes   = 12000
	DefaultExactSearchTimeoutMS  = 750
	MaxExactSearchResults        = 50
	MaxExactSearchBytes          = 65536
	MaxExactSearchTimeoutMS      = 2000
	MaxExactSearchLiterals       = 8
	MaxExactSearchPaths          = 16
)

type ExactSearchPlan struct {
	Enabled    bool     `json:"enabled"`
	Literals   []string `json:"literals,omitempty"`
	Paths      []string `json:"paths,omitempty"`
	MaxResults int      `json:"max_results,omitempty"`
	MaxBytes   int      `json:"max_bytes,omitempty"`
	TimeoutMS  int      `json:"timeout_ms,omitempty"`
}

type ExactSearchRequest struct {
	Root       string
	Literals   []string
	Paths      []string
	MaxResults int
	MaxBytes   int
	TimeoutMS  int
}

type ExactSearchResult struct {
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Match   string `json:"match,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

type ExactSearchResponse struct {
	Results     []ExactSearchResult `json:"results,omitempty"`
	Diagnostics []string            `json:"diagnostics,omitempty"`
	Truncated   bool                `json:"truncated,omitempty"`
}

type ExactSearcher interface {
	SearchExact(ctx context.Context, request ExactSearchRequest) (ExactSearchResponse, error)
}

func (p ExactSearchPlan) Normalize() (ExactSearchPlan, error) {
	if !p.Enabled {
		return ExactSearchPlan{}, nil
	}

	p.Literals = cleanList(p.Literals, nil)
	if len(p.Literals) > MaxExactSearchLiterals {
		return ExactSearchPlan{}, fmt.Errorf("exact_search.literals must contain at most %d values", MaxExactSearchLiterals)
	}
	for _, literal := range p.Literals {
		if strings.ContainsAny(literal, "\x00\r\n") {
			return ExactSearchPlan{}, fmt.Errorf("exact_search literal must be a single line")
		}
	}

	var err error
	if p.Paths, err = cleanExactPathList(p.Paths); err != nil {
		return ExactSearchPlan{}, err
	}
	if len(p.Paths) > MaxExactSearchPaths {
		return ExactSearchPlan{}, fmt.Errorf("exact_search.paths must contain at most %d values", MaxExactSearchPaths)
	}

	if p.MaxResults <= 0 {
		p.MaxResults = DefaultExactSearchMaxResults
	}
	if p.MaxResults > MaxExactSearchResults {
		p.MaxResults = MaxExactSearchResults
	}
	if p.MaxBytes <= 0 {
		p.MaxBytes = DefaultExactSearchMaxBytes
	}
	if p.MaxBytes > MaxExactSearchBytes {
		p.MaxBytes = MaxExactSearchBytes
	}
	if p.TimeoutMS <= 0 {
		p.TimeoutMS = DefaultExactSearchTimeoutMS
	}
	if p.TimeoutMS > MaxExactSearchTimeoutMS {
		p.TimeoutMS = MaxExactSearchTimeoutMS
	}
	return p, nil
}

func cleanExactPathList(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if err := validateSlashRelative("exact_search path", value); err != nil {
			return nil, err
		}
		if strings.ContainsAny(value, "*?[]{}$`'\"!;&|<>") {
			return nil, fmt.Errorf("exact_search path %q must not contain shell metacharacters", value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func ShouldEnableExactSearchForRawQuery(query string) bool {
	for _, clue := range exactSearchClues(query) {
		if strings.Contains(clue, "/") || strings.Contains(clue, ".") {
			return true
		}
		if strings.Contains(clue, "(") || strings.Contains(clue, ")") || strings.Contains(clue, "::") {
			return true
		}
		if hasMixedCase(clue) {
			return true
		}
	}
	return false
}

func highSignalExactLiterals(query string) []string {
	clues := exactSearchClues(query)
	literals := make([]string, 0, len(clues))
	seen := make(map[string]struct{}, len(clues))
	for _, clue := range clues {
		if !ShouldEnableExactSearchForRawQuery(clue) {
			continue
		}
		if _, ok := seen[clue]; ok {
			continue
		}
		seen[clue] = struct{}{}
		literals = append(literals, clue)
		if len(literals) == MaxExactSearchLiterals {
			break
		}
	}
	return literals
}

func exactSearchClues(query string) []string {
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';'
	})
	clues := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, "\"'`[]{}<>")
		field = strings.TrimRight(field, ".?!:")
		field = strings.TrimLeft(field, ":")
		if field != "" && !isQueryStopWord(strings.ToLower(field)) {
			clues = append(clues, field)
		}
	}
	return clues
}

func hasMixedCase(value string) bool {
	hasLower := false
	hasUpper := false
	for _, r := range value {
		if unicode.IsLower(r) {
			hasLower = true
		}
		if unicode.IsUpper(r) {
			hasUpper = true
		}
	}
	return hasLower && hasUpper
}
