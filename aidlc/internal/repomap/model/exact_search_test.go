package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExactSearchPlanJSONShape(t *testing.T) {
	plan := ExactSearchPlan{
		Enabled:    true,
		Literals:   []string{"Generate("},
		Paths:      []string{"aidlc/internal/generator"},
		MaxResults: 20,
		MaxBytes:   12000,
		TimeoutMS:  750,
	}
	got, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `{"enabled":true,"literals":["Generate("],"paths":["aidlc/internal/generator"],"max_results":20,"max_bytes":12000,"timeout_ms":750}`
	if string(got) != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
}

func TestExactSearchPlanNormalizeDefaultsAndClamps(t *testing.T) {
	plan, err := (ExactSearchPlan{
		Enabled:    true,
		Literals:   []string{" Generate( ", "Generate("},
		Paths:      []string{"aidlc/internal/generator", "aidlc/internal/generator"},
		MaxResults: 500,
		MaxBytes:   999999,
		TimeoutMS:  9999,
	}).Normalize()
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	assertStrings(t, "Literals", plan.Literals, []string{"Generate("})
	assertStrings(t, "Paths", plan.Paths, []string{"aidlc/internal/generator"})
	if plan.MaxResults != MaxExactSearchResults {
		t.Fatalf("MaxResults = %d, want %d", plan.MaxResults, MaxExactSearchResults)
	}
	if plan.MaxBytes != MaxExactSearchBytes {
		t.Fatalf("MaxBytes = %d, want %d", plan.MaxBytes, MaxExactSearchBytes)
	}
	if plan.TimeoutMS != MaxExactSearchTimeoutMS {
		t.Fatalf("TimeoutMS = %d, want %d", plan.TimeoutMS, MaxExactSearchTimeoutMS)
	}
}

func TestExactSearchPlanNormalizeRejectsUnsafeInput(t *testing.T) {
	tests := []struct {
		name string
		plan ExactSearchPlan
		want string
	}{
		{name: "too many literals", plan: ExactSearchPlan{Enabled: true, Literals: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}}, want: "literals"},
		{name: "literal newline", plan: ExactSearchPlan{Enabled: true, Literals: []string{"a\nb"}}, want: "single line"},
		{name: "absolute path", plan: ExactSearchPlan{Enabled: true, Paths: []string{"/tmp/a.go"}}, want: "slash-relative"},
		{name: "parent path", plan: ExactSearchPlan{Enabled: true, Paths: []string{"../a.go"}}, want: "parent path"},
		{name: "shell metacharacter", plan: ExactSearchPlan{Enabled: true, Paths: []string{"aidlc/internal/*.go"}}, want: "metacharacters"},
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

func TestShouldEnableExactSearchForRawQuery(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{query: "where is Generate used", want: true},
		{query: "aidlc/internal/generator render", want: true},
		{query: "Generate(", want: true},
		{query: "where is guidance rendered", want: false},
	}

	for _, tt := range tests {
		if got := ShouldEnableExactSearchForRawQuery(tt.query); got != tt.want {
			t.Fatalf("ShouldEnableExactSearchForRawQuery(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}
