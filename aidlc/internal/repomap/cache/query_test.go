package cache

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

func TestQuerierReturnsBM25RankedResults(t *testing.T) {
	t.Parallel()

	mapDir := filepath.Join(t.TempDir(), filepath.FromSlash(model.MapDir))
	writeShard(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/auth.md", Kind: "spec", Title: "Auth", Text: "auth auth auth login token"},
		{Path: "docs/spec/billing.md", Kind: "spec", Title: "Billing", Text: "invoice payment"},
		{Path: "docs/blueprints/auth.md", Kind: "blueprint", Title: "Auth blueprint", Text: "auth login"},
	})
	writeShard(t, mapDir, model.FilesShard, []model.FileRecord{
		{Path: "internal/auth/service.go", Language: "go", ContentHash: "hash"},
	})

	if err := NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	results, err := NewQuerier(mapDir).Query(context.Background(), "auth login", 2)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Query() len = %d, want 2: %#v", len(results), results)
	}
	if results[0].Path != "docs/spec/auth.md" {
		t.Fatalf("top result path = %q, want docs/spec/auth.md: %#v", results[0].Path, results)
	}
	if results[0].Score > results[1].Score {
		t.Fatalf("results not ordered by ascending bm25 score: %#v", results)
	}
	if results[0].Snippet == "" {
		t.Fatalf("top result snippet is empty")
	}
}

func TestQuerierHandlesEmptyQueryAndLimit(t *testing.T) {
	t.Parallel()

	mapDir := filepath.Join(t.TempDir(), filepath.FromSlash(model.MapDir))
	if err := NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	results, err := NewQuerier(mapDir).Query(context.Background(), "auth", 0)
	if err != nil {
		t.Fatalf("Query(limit 0) error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Query(limit 0) len = %d, want 0", len(results))
	}

	results, err = NewQuerier(mapDir).Query(context.Background(), "   ", 10)
	if err != nil {
		t.Fatalf("Query(empty) error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Query(empty) len = %d, want 0", len(results))
	}
}
