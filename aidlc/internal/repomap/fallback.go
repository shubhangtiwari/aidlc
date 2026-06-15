package repomap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type FallbackQuerier struct {
	mapDir string
}

var _ model.Querier = (*FallbackQuerier)(nil)

func NewFallbackQuerier(mapDir string) *FallbackQuerier {
	return &FallbackQuerier{mapDir: mapDir}
}

func (q *FallbackQuerier) Query(ctx context.Context, query string, limit int) ([]model.QueryResult, error) {
	return q.QueryShard(ctx, query, limit, "")
}

func (q *FallbackQuerier) QueryShard(ctx context.Context, query string, limit int, shard string) ([]model.QueryResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, nil
	}
	shards, err := selectedShards(shard)
	if err != nil {
		return nil, err
	}
	terms := queryTerms(query)
	if len(terms) == 0 {
		return nil, nil
	}

	matches := map[string]model.QueryResult{}
	for _, name := range shards {
		entries, err := q.loadShard(name)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			score := matchScore(entry.SearchText, terms)
			if score == 0 {
				continue
			}
			existing, ok := matches[entry.Path]
			if ok {
				if score > existing.Score {
					existing.Score = score
				}
				if existing.Snippet == "" {
					existing.Snippet = entry.Snippet
				}
				matches[entry.Path] = existing
				continue
			}
			matches[entry.Path] = model.QueryResult{
				Path:    entry.Path,
				Score:   score,
				Snippet: entry.Snippet,
			}
		}
	}

	results := make([]model.QueryResult, 0, len(matches))
	for _, result := range matches {
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

type fallbackEntry struct {
	Path       string
	SearchText string
	Snippet    string
}

func (q *FallbackQuerier) loadShard(name string) ([]fallbackEntry, error) {
	switch name {
	case model.FilesShard:
		records, err := readFallbackShard[model.FileRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: strings.Join([]string{record.Path, record.Language, record.ContentHash}, " "),
				Snippet:    strings.Join([]string{record.Language, record.ContentHash}, " "),
			})
		}
		return entries, nil
	case model.ImportsShard:
		records, err := readFallbackShard[model.ImportRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: strings.Join([]string{record.Path, record.Language, record.ImportPath}, " "),
				Snippet:    record.ImportPath,
			})
		}
		return entries, nil
	case model.TestsShard:
		records, err := readFallbackShard[model.TestRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: strings.Join([]string{record.Path, record.Language, record.TargetPath}, " "),
				Snippet:    record.TargetPath,
			})
		}
		return entries, nil
	case model.BlueprintsShard:
		records, err := readFallbackShard[model.BlueprintRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: strings.Join([]string{record.Path, record.Module, record.Section, record.Text}, " "),
				Snippet:    strings.Join([]string{record.Module, record.Section}, " "),
			})
		}
		return entries, nil
	case model.DocsShard:
		records, err := readFallbackShard[model.DocRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: strings.Join([]string{record.Path, record.Kind, record.Title, record.Text}, " "),
				Snippet:    record.Title,
			})
		}
		return entries, nil
	case model.ChangesShard:
		records, err := readFallbackShard[model.ChangeRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: strings.Join([]string{record.Path, record.Kind, record.ID, record.Title, record.Status, record.Text}, " "),
				Snippet:    strings.Join([]string{record.ID, record.Title, record.Status}, " "),
			})
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("unsupported repo-map shard %q", name)
	}
}

func readFallbackShard[T any](mapDir, name string) ([]T, error) {
	file, err := os.Open(filepath.Join(mapDir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	defer file.Close()

	records, err := model.ReadJSONL[T](file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return records, nil
}

func selectedShards(shard string) ([]string, error) {
	if shard == "" {
		return allShardNames(), nil
	}
	normalized := normalizeShard(shard)
	for _, name := range allShardNames() {
		if normalized == name {
			return []string{name}, nil
		}
	}
	return nil, fmt.Errorf("unknown repo-map shard %q", shard)
}

func allShardNames() []string {
	return []string{
		model.FilesShard,
		model.ImportsShard,
		model.TestsShard,
		model.BlueprintsShard,
		model.DocsShard,
		model.ChangesShard,
	}
}

func normalizeShard(shard string) string {
	shard = strings.TrimSpace(strings.ToLower(shard))
	if shard == "" || strings.HasSuffix(shard, ".jsonl") {
		return shard
	}
	return shard + ".jsonl"
}

func queryTerms(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, `"'`)
		if field != "" {
			terms = append(terms, field)
		}
	}
	return terms
}

func matchScore(text string, terms []string) float64 {
	text = strings.ToLower(text)
	var score float64
	for _, term := range terms {
		if strings.Contains(text, term) {
			score++
		}
	}
	return score
}
