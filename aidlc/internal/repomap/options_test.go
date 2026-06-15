package repomap

import (
	"strings"
	"testing"
)

func TestNormalizeIncludeCanonicalizesSortsAndDeduplicates(t *testing.T) {
	got, err := NormalizeInclude([]string{" docs/./architecture ", "aidlc\\internal", "docs/architecture"})
	if err != nil {
		t.Fatalf("NormalizeInclude() error = %v", err)
	}
	assertPaths(t, "include", got, []string{"aidlc/internal", "docs/architecture"})
}

func TestNormalizeIncludeRejectsInvalidPaths(t *testing.T) {
	for _, item := range []string{"", "/absolute", "../outside", "docs/map", "docs/map/cache"} {
		t.Run(item, func(t *testing.T) {
			_, err := NormalizeInclude([]string{item})
			if err == nil {
				t.Fatal("NormalizeInclude() succeeded, want error")
			}
		})
	}
}

func TestNormalizeIncludeRejectsGeneratedAndDependencyRoots(t *testing.T) {
	for _, item := range []string{
		".claude",
		".codex",
		".cursor/state",
		".venv/lib",
		"build",
		"cache/tmp",
		"dist",
		"node_modules/pkg",
		"out",
		"target/debug",
		"vendor",
	} {
		t.Run(item, func(t *testing.T) {
			_, err := NormalizeInclude([]string{item})
			if err == nil {
				t.Fatal("NormalizeInclude() succeeded, want error")
			}
			if !strings.Contains(err.Error(), "generated or dependency directories") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
