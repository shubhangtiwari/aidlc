package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
)

func TestDetectProjectFactsMinimal(t *testing.T) {
	root := t.TempDir()

	facts, err := DetectProjectFacts(root)
	if err != nil {
		t.Fatalf("detect facts: %v", err)
	}

	if facts.HasManifest {
		t.Fatalf("HasManifest = true, want false")
	}
	if facts.ProjectName != filepath.Base(root) {
		t.Fatalf("ProjectName = %q, want %q", facts.ProjectName, filepath.Base(root))
	}
	if facts.SourceRoot != "." {
		t.Fatalf("SourceRoot = %q, want .", facts.SourceRoot)
	}
}

func TestDetectProjectFactsNodeTypescript(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "package.json", `{
  "name": "@example/app",
  "packageManager": "pnpm@10.0.0",
  "engines": {"node": ">=22"},
  "devDependencies": {"typescript": "^5.0.0"}
}`)
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}

	facts, err := DetectProjectFacts(root)
	if err != nil {
		t.Fatalf("detect facts: %v", err)
	}

	assertEqual(t, facts.HasManifest, true)
	assertEqual(t, facts.ProjectName, "@example/app")
	assertEqual(t, facts.Language, "TypeScript / Node")
	assertEqual(t, facts.ManifestPath, "package.json")
	assertEqual(t, facts.SourceRoot, "src")
	assertEqual(t, facts.PackageName, "@example/app")
	assertEqual(t, facts.Runtime, ">=22")
	assertEqual(t, facts.BuildTool, "pnpm@10.0.0")
}

func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
