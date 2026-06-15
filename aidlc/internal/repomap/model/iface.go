package model

import "context"

type CacheBuilder interface {
	Build(ctx context.Context, mapDir string) error
}

type Querier interface {
	Query(ctx context.Context, query string, limit int) ([]QueryResult, error)
}

type QueryResult struct {
	Path    string  `json:"path"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}
