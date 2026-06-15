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
