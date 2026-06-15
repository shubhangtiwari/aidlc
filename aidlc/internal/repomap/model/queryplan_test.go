package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSearchPlanV1JSONFieldOrder(t *testing.T) {
	includeTests := false
	plan := SearchPlanV1{
		Version:           1,
		Question:          "Where is auth checked?",
		Terms:             []string{"auth", "checked"},
		Phrases:           []string{"token validation"},
		Symbols:           []string{"Authorize"},
		Paths:             []string{"aidlc/internal/auth/auth.go"},
		Globs:             []string{"aidlc/internal/**/*.go"},
		Languages:         []string{"go"},
		Shards:            []string{SourceChunksShard},
		IncludeTests:      &includeTests,
		RelationshipDepth: 2,
		Limit:             25,
	}
	got, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `{"version":1,"question":"Where is auth checked?","terms":["auth","checked"],"phrases":["token validation"],"symbols":["Authorize"],"paths":["aidlc/internal/auth/auth.go"],"globs":["aidlc/internal/**/*.go"],"languages":["go"],"shards":["source_chunks.jsonl"],"include_tests":false,"relationship_depth":2,"limit":25}`
	if string(got) != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
}

func TestSearchPlanNormalizeDefaultsAndCleans(t *testing.T) {
	plan, err := (SearchPlanV1{
		Version:  1,
		Question: "  Where is Auth checked?  ",
		Terms:    []string{" Auth ", "auth", "Token"},
		Phrases:  []string{" token validation ", "token validation"},
		Symbols:  []string{" Authorize ", "Authorize"},
		Paths:    []string{"b.go", "a.go", "b.go"},
		Globs:    []string{"aidlc/**/*.go"},
		Languages: []string{
			" Go ",
			"go",
		},
		Shards: []string{"source_chunks", "SOURCE_CHUNKS.JSONL"},
		Limit:  100,
	}).Normalize()
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if plan.Question != "Where is Auth checked?" {
		t.Fatalf("Question = %q", plan.Question)
	}
	assertStrings(t, "Terms", plan.Terms, []string{"auth", "token"})
	assertStrings(t, "Phrases", plan.Phrases, []string{"token validation"})
	assertStrings(t, "Symbols", plan.Symbols, []string{"Authorize"})
	assertStrings(t, "Paths", plan.Paths, []string{"b.go", "a.go"})
	assertStrings(t, "Globs", plan.Globs, []string{"aidlc/**/*.go"})
	assertStrings(t, "Languages", plan.Languages, []string{"go"})
	assertStrings(t, "Shards", plan.Shards, []string{SourceChunksShard})
	if plan.RelationshipDepth != DefaultRelationshipDepth {
		t.Fatalf("RelationshipDepth = %d, want %d", plan.RelationshipDepth, DefaultRelationshipDepth)
	}
	if plan.Limit != MaxSearchLimit {
		t.Fatalf("Limit = %d, want %d", plan.Limit, MaxSearchLimit)
	}
}

func TestCompileRawSearchPlan(t *testing.T) {
	plan, err := CompileRawSearchPlan("Where is token validation handled?", 0, "docs")
	if err != nil {
		t.Fatalf("CompileRawSearchPlan() error = %v", err)
	}
	if plan.Version != SearchPlanVersion {
		t.Fatalf("Version = %d, want %d", plan.Version, SearchPlanVersion)
	}
	if plan.Question != "Where is token validation handled?" {
		t.Fatalf("Question = %q", plan.Question)
	}
	assertStrings(t, "Terms", plan.Terms, []string{"token", "validation", "handled"})
	assertStrings(t, "Shards", plan.Shards, []string{DocsShard})
	if plan.Limit != DefaultSearchLimit {
		t.Fatalf("Limit = %d, want %d", plan.Limit, DefaultSearchLimit)
	}
}

func TestSearchPlanValidationRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		plan SearchPlanV1
		want string
	}{
		{
			name: "missing version",
			plan: SearchPlanV1{},
			want: "unsupported search plan version",
		},
		{
			name: "unsupported version",
			plan: SearchPlanV1{Version: 2},
			want: "unsupported search plan version",
		},
		{
			name: "absolute path",
			plan: SearchPlanV1{Version: 1, Paths: []string{"/tmp/a.go"}},
			want: "slash-relative",
		},
		{
			name: "parent path",
			plan: SearchPlanV1{Version: 1, Paths: []string{"../a.go"}},
			want: "parent path segments",
		},
		{
			name: "backslash glob",
			plan: SearchPlanV1{Version: 1, Globs: []string{`aidlc\*.go`}},
			want: "slash-relative",
		},
		{
			name: "bad glob",
			plan: SearchPlanV1{Version: 1, Globs: []string{"aidlc/[.go"}},
			want: "invalid",
		},
		{
			name: "shard",
			plan: SearchPlanV1{Version: 1, Shards: []string{"missing"}},
			want: "unknown repo-map shard",
		},
		{
			name: "relationship depth",
			plan: SearchPlanV1{Version: 1, RelationshipDepth: 3},
			want: "relationship_depth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.plan.Normalize()
			if err == nil {
				t.Fatal("Normalize() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Normalize() error = %q, want containing %q", err, tt.want)
			}
		})
	}
}

func TestKnownShardNamesIncludesSymbols(t *testing.T) {
	want := []string{
		FilesShard,
		ImportsShard,
		TestsShard,
		BlueprintsShard,
		DocsShard,
		ChangesShard,
		SourceChunksShard,
		SymbolsShard,
	}
	assertStrings(t, "KnownShardNames", KnownShardNames(), want)
}

func assertStrings(t testing.TB, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s len = %d, want %d: %#v", name, len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %q, want %q: %#v", name, i, got[i], want[i], got)
		}
	}
}
