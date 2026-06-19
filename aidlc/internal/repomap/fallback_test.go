package repomap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

func TestFallbackQuerierReturnsSupersetSortedByPath(t *testing.T) {
	t.Parallel()

	mapDir := t.TempDir()
	writeJSONL(t, mapDir, model.FilesShard, []model.FileRecord{
		{Path: "pkg/zeta/zeta.go", Language: "go", ContentHash: "auth-hash"},
		{Path: "internal/auth/service.go", Language: "go", ContentHash: "hash"},
	})
	writeJSONL(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/auth.md", Kind: "spec", Title: "Auth spec", Text: "login token"},
		{Path: "docs/spec/billing.md", Kind: "spec", Title: "Billing", Text: "invoice"},
	})
	writeJSONL(t, mapDir, model.BlueprintsShard, []model.BlueprintRecord{
		{Path: "docs/blueprints/auth.md", Module: "auth", Section: "State", Text: "login state"},
	})

	results, err := NewFallbackQuerier(mapDir).Query(context.Background(), "auth login", 10)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	got := resultPaths(results)
	want := []string{
		"docs/blueprints/auth.md",
		"docs/spec/auth.md",
		"internal/auth/service.go",
		"pkg/zeta/zeta.go",
	}
	if !equalStrings(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}

func TestFallbackQuerierAppliesLimitAfterPathSort(t *testing.T) {
	t.Parallel()

	mapDir := t.TempDir()
	writeJSONL(t, mapDir, model.FilesShard, []model.FileRecord{
		{Path: "c.go", Language: "go", ContentHash: "auth"},
		{Path: "a.go", Language: "go", ContentHash: "auth"},
		{Path: "b.go", Language: "go", ContentHash: "auth"},
	})

	results, err := NewFallbackQuerier(mapDir).Query(context.Background(), "auth", 2)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	want := []string{"a.go", "b.go"}
	if got := resultPaths(results); !equalStrings(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}

func TestFallbackQuerierShardFiltering(t *testing.T) {
	t.Parallel()

	mapDir := t.TempDir()
	writeJSONL(t, mapDir, model.FilesShard, []model.FileRecord{
		{Path: "internal/auth/service.go", Language: "go", ContentHash: "hash"},
	})
	writeJSONL(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/auth.md", Kind: "spec", Title: "Auth spec", Text: "login token"},
	})

	results, err := NewFallbackQuerier(mapDir).QueryShard(context.Background(), "auth", 10, "docs")
	if err != nil {
		t.Fatalf("QueryShard() error = %v", err)
	}
	want := []string{"docs/spec/auth.md"}
	if got := resultPaths(results); !equalStrings(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}

func TestFallbackQuerierUsesSignificantTermsForQuestionInput(t *testing.T) {
	t.Parallel()

	mapDir := t.TempDir()
	writeJSONL(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/auth.md", Kind: "spec", Title: "Auth spec", Text: "token validation handled by auth service"},
		{Path: "docs/spec/billing.md", Kind: "spec", Title: "Billing", Text: "invoice payment"},
	})
	writeJSONL(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{Path: "internal/auth/service.go", Language: "go", StartLine: 10, EndLine: 12, Text: "func ValidateToken handles validation"},
	})

	results, err := NewFallbackQuerier(mapDir).Query(context.Background(), "Where is token validation handled?", 10)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	want := []string{"docs/spec/auth.md", "internal/auth/service.go"}
	if got := resultPaths(results); !equalStrings(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}

func TestFallbackQuerierFindsSourceChunkFromNaturalSpacedIdentifierTerms(t *testing.T) {
	t.Parallel()

	mapDir := t.TempDir()
	writeJSONL(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/repomap.md", Kind: "spec", Title: "Repo map source chunk extraction", Text: "source chunk extraction plan"},
	})
	writeJSONL(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{
			Path:      "aidlc/internal/repomap/sourcechunks.go",
			Language:  "go",
			StartLine: 20,
			EndLine:   26,
			Text:      "func ExtractSourceChunks(path, language, content string) []model.SourceChunkRecord",
		},
	})

	results, err := NewFallbackQuerier(mapDir).Query(context.Background(), "extract source chunks", 10)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if got, want := resultPaths(results), []string{"aidlc/internal/repomap/sourcechunks.go"}; !containsAll(got, want) {
		t.Fatalf("paths = %#v, want to contain %#v", got, want)
	}
}

func TestFallbackQueryTermsNormalizePunctuationAndStopwords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "punctuation",
			query: "token,validation;cache(query)",
			want:  []string{"token", "validation", "cache", "query"},
		},
		{
			name:  "question stopwords",
			query: "Where is token validation handled?",
			want:  []string{"token", "validation", "handled"},
		},
		{
			name:  "stopwords only",
			query: "where is the and or",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := queryTerms(tt.query); !equalStrings(got, tt.want) {
				t.Fatalf("queryTerms() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func containsAll(got, want []string) bool {
	set := make(map[string]struct{}, len(got))
	for _, item := range got {
		set[item] = struct{}{}
	}
	for _, item := range want {
		if _, ok := set[item]; !ok {
			return false
		}
	}
	return true
}

func TestFallbackQuerierEmptyAndMissingShards(t *testing.T) {
	t.Parallel()

	results, err := NewFallbackQuerier(t.TempDir()).Query(context.Background(), "auth", 10)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Query() len = %d, want 0", len(results))
	}

	results, err = NewFallbackQuerier(t.TempDir()).Query(context.Background(), "where is the and or", 10)
	if err != nil {
		t.Fatalf("Query(stopwords) error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Query(stopwords) len = %d, want 0", len(results))
	}
}

func writeJSONL[T model.SortableRecord](t testing.TB, root, name string, records []T) {
	t.Helper()

	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create map dir: %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	if err := model.WriteJSONL(file, records); err != nil {
		_ = file.Close()
		t.Fatalf("write %s: %v", name, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close %s: %v", name, err)
	}
}

func resultPaths(results []model.QueryResult) []string {
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
