package repomap

import (
	"context"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type fakeQuerier struct {
	results []model.QueryResult
}

func (q fakeQuerier) Query(context.Context, string, int) ([]model.QueryResult, error) {
	return q.results, nil
}

func TestQueryEngineFormatsRankedResultsDeterministically(t *testing.T) {
	t.Parallel()

	engine := NewQueryEngine(fakeQuerier{results: []model.QueryResult{
		{Path: "internal/auth/service.go", Score: 3.25, Snippet: "auth\nservice"},
		{Path: "docs/spec/auth.md", Score: 1, Snippet: "spec   auth"},
	}})

	text, err := engine.QueryText(context.Background(), "auth", 10, "")
	if err != nil {
		t.Fatalf("QueryText() error = %v", err)
	}
	want := "internal/auth/service.go\t3.250000\tauth service\n" +
		"docs/spec/auth.md\t1.000000\tspec auth\n"
	if text != want {
		t.Fatalf("QueryText() = %q, want %q", text, want)
	}
}

func TestQueryEngineReturnsEmptyForEmptyQueryAndLimit(t *testing.T) {
	t.Parallel()

	engine := NewQueryEngine(fakeQuerier{results: []model.QueryResult{
		{Path: "internal/auth/service.go", Score: 1, Snippet: "auth"},
	}})
	for _, tc := range []struct {
		name  string
		query string
		limit int
	}{
		{name: "empty query", query: "  ", limit: 10},
		{name: "zero limit", query: "auth", limit: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			results, err := engine.Query(context.Background(), tc.query, tc.limit, "")
			if err != nil {
				t.Fatalf("Query() error = %v", err)
			}
			if len(results) != 0 {
				t.Fatalf("Query() len = %d, want 0", len(results))
			}
		})
	}
}

func TestQueryEngineTruncatesInjectedQuerierResults(t *testing.T) {
	t.Parallel()

	engine := NewQueryEngine(fakeQuerier{results: []model.QueryResult{
		{Path: "a.go", Score: 3, Snippet: "a"},
		{Path: "b.go", Score: 2, Snippet: "b"},
		{Path: "c.go", Score: 1, Snippet: "c"},
	}})
	results, err := engine.Query(context.Background(), "auth", 2, "")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if got, want := len(results), 2; got != want {
		t.Fatalf("Query() len = %d, want %d", got, want)
	}
	if results[0].Path != "a.go" || results[1].Path != "b.go" {
		t.Fatalf("Query() order = %#v, want first two injected results", results)
	}
}

func TestQueryEngineRejectsShardFilterWhenQuerierCannotFilter(t *testing.T) {
	t.Parallel()

	engine := NewQueryEngine(fakeQuerier{})
	_, err := engine.Query(context.Background(), "auth", 10, model.DocsShard)
	if err == nil {
		t.Fatal("Query() error = nil, want shard filtering error")
	}
}
