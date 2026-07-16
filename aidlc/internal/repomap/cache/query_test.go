package cache

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

func TestQuerierReturnsBM25RankedResults(t *testing.T) {
	t.Parallel()

	mapDir := filepath.Join(t.TempDir(), filepath.FromSlash(model.MapDir))
	writeShard(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/auth.md", Kind: "spec", Title: "Auth", Text: "auth auth auth login token"},
		{Path: "docs/spec/billing.md", Kind: "spec", Title: "Billing", Text: "invoice payment"},
		{Path: "docs/blueprints/auth.md", Kind: "blueprint", Title: "Auth blueprint", Text: "auth login supporting module notes"},
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

func TestQuerierUsesSignificantTermsForQuestionInput(t *testing.T) {
	t.Parallel()

	mapDir := filepath.Join(t.TempDir(), filepath.FromSlash(model.MapDir))
	writeShard(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/auth.md", Kind: "spec", Title: "Auth", Text: "token validation handled by auth service"},
		{Path: "docs/spec/billing.md", Kind: "spec", Title: "Billing", Text: "invoice payment"},
	})

	if err := NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	results, err := NewQuerier(mapDir).Query(context.Background(), "Where is token validation handled?", 10)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if got, want := cacheResultPaths(results), []string{"docs/spec/auth.md"}; !equalStrings(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}

func TestQuerierFindsSourceChunkFromNaturalSpacedIdentifierTerms(t *testing.T) {
	t.Parallel()

	mapDir := filepath.Join(t.TempDir(), filepath.FromSlash(model.MapDir))
	writeShard(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/repomap.md", Kind: "spec", Title: "Repo map source chunk extraction", Text: "source chunk extraction plan"},
	})
	writeShard(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{
			Path:      "aidlc/internal/repomap/sourcechunks.go",
			Language:  "go",
			StartLine: 20,
			EndLine:   26,
			Text:      "func ExtractSourceChunks(path, language, content string) []model.SourceChunkRecord",
		},
		{
			Path:      "aidlc/internal/repomap/sourcechunks_test.go",
			Language:  "go",
			StartLine: 9,
			EndLine:   13,
			Text:      "func TestExtractSourceChunksCoversSourceChunkExtraction(t *testing.T)",
		},
	})

	if err := NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	for _, query := range []string{"source chunk extraction", "extract source chunks"} {
		t.Run(query, func(t *testing.T) {
			results, err := NewQuerier(mapDir).Query(context.Background(), query, 5)
			if err != nil {
				t.Fatalf("Query() error = %v", err)
			}
			if len(results) == 0 || results[0].Path != "aidlc/internal/repomap/sourcechunks.go" {
				t.Fatalf("top result for %q = %#v, want sourcechunks.go first", query, results)
			}
		})
	}
}

func TestQuerierFindsSymbolRecords(t *testing.T) {
	t.Parallel()

	mapDir := filepath.Join(t.TempDir(), filepath.FromSlash(model.MapDir))
	writeShard(t, mapDir, model.SymbolsShard, []model.SymbolRecord{
		{
			Path:      "internal/auth/principal.go",
			Language:  "go",
			Kind:      "method",
			Name:      "NormalizePrincipal",
			Receiver:  "Service",
			Container: "auth",
			StartLine: 14,
			EndLine:   20,
		},
	})
	writeShard(t, mapDir, model.FilesShard, []model.FileRecord{
		{Path: "internal/auth/principal.go", Language: "go", ContentHash: "hash"},
		{Path: "internal/billing/invoice.go", Language: "go", ContentHash: "hash"},
	})

	if err := NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	for _, query := range []string{"NormalizePrincipal", "normalize principal"} {
		t.Run(query, func(t *testing.T) {
			results, err := NewQuerier(mapDir).Query(context.Background(), query, 5)
			if err != nil {
				t.Fatalf("Query() error = %v", err)
			}
			if len(results) == 0 || results[0].Path != "internal/auth/principal.go" {
				t.Fatalf("top result for %q = %#v, want symbol path first", query, results)
			}
			if !strings.Contains(results[0].Snippet, "NormalizePrincipal") {
				t.Fatalf("top result snippet = %q, want symbol name", results[0].Snippet)
			}
		})
	}
}

func TestBuilderSchemaDoesNotCreateVectorEmbeddingOrModelTables(t *testing.T) {
	t.Parallel()

	mapDir := filepath.Join(t.TempDir(), filepath.FromSlash(model.MapDir))
	writeShard(t, mapDir, model.SymbolsShard, []model.SymbolRecord{
		{Path: "internal/auth/principal.go", Language: "go", Kind: "function", Name: "NormalizePrincipal"},
	})

	if err := NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	db, err := sql.Open(sqliteDriver, filepath.Join(mapDir, model.SQLiteFilename))
	if err != nil {
		t.Fatalf("open sqlite cache: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT name
		FROM sqlite_master
		WHERE type IN ('table', 'view')
			AND (
				lower(name) LIKE '%vector%'
				OR lower(name) LIKE '%embedding%'
				OR lower(name) LIKE '%model%'
			)
		ORDER BY name`)
	if err != nil {
		t.Fatalf("query sqlite schema: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan sqlite schema row: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite schema rows: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("unexpected vector/embedding/model schema objects: %#v", names)
	}
}

func TestFTSMatchQueryNormalizesPunctuationAndStopwords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "punctuation",
			query: "token,validation;cache(query)",
			want:  `"token" "validation" "cache" "query"`,
		},
		{
			name:  "question stopwords",
			query: "Where is token validation handled?",
			want:  `"token" "validation" "handled"`,
		},
		{
			name:  "stopwords only",
			query: "where is the and or",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ftsMatchQuery(tt.query); got != tt.want {
				t.Fatalf("ftsMatchQuery() = %q, want %q", got, tt.want)
			}
		})
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

	results, err = NewQuerier(mapDir).Query(context.Background(), "where is the and or", 10)
	if err != nil {
		t.Fatalf("Query(stopwords) error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Query(stopwords) len = %d, want 0", len(results))
	}
}

func cacheResultPaths(results []model.QueryResult) []string {
	paths := make([]string, 0, len(results))
	for _, result := range results {
		paths = append(paths, result.Path)
	}
	return paths
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
