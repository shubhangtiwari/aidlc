package repomap

import (
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

func TestFuseRankedListsDeduplicatesByPathDeterministically(t *testing.T) {
	t.Parallel()

	results := fuseRankedLists(10,
		rankedList{weight: 1, results: []model.QueryResult{
			{Path: "b.go", Score: 1, Snippet: "longer snippet"},
			{Path: "a.go", Score: 1, Snippet: "a"},
		}},
		rankedList{weight: 2, results: []model.QueryResult{
			{Path: "a.go", Score: 1, Snippet: "alpha"},
			{Path: "b.go", Score: 1, Snippet: "b"},
		}},
	)

	got := resultPaths(results)
	want := []string{"a.go", "b.go"}
	if !equalStrings(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
	if results[1].Snippet != "b" {
		t.Fatalf("deduped snippet = %q, want shortest snippet", results[1].Snippet)
	}
}

func TestPathScorePrefersExactPath(t *testing.T) {
	t.Parallel()

	exact := pathScore("internal/auth/service.go", []string{"internal/auth/service.go"}, nil)
	segment := pathScore("internal/auth/service.go", []string{"auth"}, nil)
	if exact <= segment {
		t.Fatalf("exact path score = %v, segment score = %v, want exact higher", exact, segment)
	}
}
