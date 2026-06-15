package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestQueryTerms(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "question noise",
			query: "Where is token validation handled?",
			want:  []string{"token", "validation", "handled"},
		},
		{
			name:  "connector noise",
			query: "files and imports for the repo-map cache",
			want:  []string{"files", "imports", "repo-map", "cache"},
		},
		{
			name:  "code terms",
			query: `model.QueryResult "source_chunks.jsonl" --check aidlc/internal/repomap/cache`,
			want:  []string{"model.queryresult", "source_chunks.jsonl", "check", "aidlc/internal/repomap/cache"},
		},
		{
			name:  "dedupe input order",
			query: "auth AUTH token auth",
			want:  []string{"auth", "token"},
		},
		{
			name:  "empty after stopwords",
			query: "where is the and or",
			want:  nil,
		},
		{
			name:  "punctuation boundaries",
			query: "token,validation;cache(query)",
			want:  []string{"token", "validation", "cache", "query"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := QueryTerms(tt.query); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("QueryTerms() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSearchTextExpandsCodeIdentifiers(t *testing.T) {
	t.Parallel()

	text := SearchText(
		"aidlc/internal/repomap/sourcechunks.go",
		"func ExtractSourceChunks(path string) []model.SourceChunkRecord",
		"ftsMatchQuery",
		"source_chunks.jsonl",
		"repo-map-cache",
	)
	for _, want := range []string{
		"sourcechunks",
		"source",
		"chunk",
		"chunks",
		"extract",
		"extraction",
		"fts",
		"match",
		"query",
		"repo",
		"map",
		"cache",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("SearchText() = %q, want substring %q", text, want)
		}
	}
}
