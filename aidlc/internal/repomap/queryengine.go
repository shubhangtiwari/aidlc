package repomap

import (
	"context"
	"fmt"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type QueryEngine struct {
	querier model.Querier
}

type shardQuerier interface {
	QueryShard(ctx context.Context, query string, limit int, shard string) ([]model.QueryResult, error)
}

type planQuerier interface {
	QueryPlan(ctx context.Context, plan model.SearchPlanV1) ([]model.QueryResult, error)
}

func NewQueryEngine(querier model.Querier) QueryEngine {
	return QueryEngine{querier: querier}
}

func (e QueryEngine) Query(ctx context.Context, query string, limit int, shard string) ([]model.QueryResult, error) {
	if e.querier == nil {
		return nil, fmt.Errorf("repo-map querier is required")
	}
	if limit <= 0 || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if shard != "" {
		filtered, ok := e.querier.(shardQuerier)
		if !ok {
			return nil, fmt.Errorf("repo-map querier does not support shard filtering")
		}
		results, err := filtered.QueryShard(ctx, query, limit, shard)
		return topK(results, limit), err
	}
	results, err := e.querier.Query(ctx, query, limit)
	return topK(results, limit), err
}

func (e QueryEngine) QueryPlan(ctx context.Context, plan model.SearchPlanV1) ([]model.QueryResult, error) {
	if e.querier == nil {
		return nil, fmt.Errorf("repo-map querier is required")
	}
	normalized, err := plan.Normalize()
	if err != nil {
		return nil, err
	}
	if normalized.Limit <= 0 || planIsEmpty(normalized) {
		return nil, nil
	}
	if planned, ok := e.querier.(planQuerier); ok {
		results, err := planned.QueryPlan(ctx, normalized)
		return topK(results, normalized.Limit), err
	}

	var lists []rankedList
	for _, query := range planQueries(normalized) {
		results, err := queryPlanTextChannel(ctx, e.querier, query, normalized)
		if err != nil {
			return nil, err
		}
		lists = append(lists, rankedList{weight: 1.0, results: results})
	}
	return fuseRankedLists(normalized.Limit, lists...), nil
}

func (e QueryEngine) QueryText(ctx context.Context, query string, limit int, shard string) (string, error) {
	results, err := e.Query(ctx, query, limit, shard)
	if err != nil {
		return "", err
	}
	return FormatQueryResults(results), nil
}

func FormatQueryResults(results []model.QueryResult) string {
	var b strings.Builder
	for _, result := range results {
		fmt.Fprintf(&b, "%s\t%.6f\t%s\n", result.Path, result.Score, oneLine(result.Snippet))
	}
	return b.String()
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func topK(results []model.QueryResult, limit int) []model.QueryResult {
	if len(results) <= limit {
		return results
	}
	return results[:limit]
}

func planIsEmpty(plan model.SearchPlanV1) bool {
	return strings.TrimSpace(plan.Question) == "" &&
		len(plan.Terms) == 0 &&
		len(plan.Phrases) == 0 &&
		len(plan.Symbols) == 0 &&
		len(plan.Paths) == 0 &&
		len(plan.Globs) == 0
}

func planQueries(plan model.SearchPlanV1) []string {
	queries := make([]string, 0, 1+len(plan.Phrases)+len(plan.Symbols)+len(plan.Paths))
	combined := strings.Join(append(append([]string{}, plan.Terms...), plan.Question), " ")
	if strings.TrimSpace(combined) != "" {
		queries = append(queries, combined)
	}
	queries = append(queries, plan.Phrases...)
	queries = append(queries, plan.Symbols...)
	queries = append(queries, plan.Paths...)
	return compactStrings(queries)
}

func queryPlanTextChannel(ctx context.Context, querier model.Querier, query string, plan model.SearchPlanV1) ([]model.QueryResult, error) {
	limit := candidateLimitForPlan(plan.Limit)
	if len(plan.Shards) == 0 {
		return querier.Query(ctx, query, limit)
	}
	filtered, ok := querier.(shardQuerier)
	if !ok {
		return nil, fmt.Errorf("repo-map querier does not support shard filtering")
	}
	var lists []rankedList
	for _, shard := range plan.Shards {
		results, err := filtered.QueryShard(ctx, query, limit, shard)
		if err != nil {
			return nil, err
		}
		lists = append(lists, rankedList{weight: 1.0, results: results})
	}
	return fuseRankedLists(limit, lists...), nil
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}
