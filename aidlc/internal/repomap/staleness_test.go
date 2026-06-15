package repomap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
)

func TestCheckStalenessFreshAfterIndexWrite(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "main.go", "package main\n")

	shards, err := ScanDir(root)
	if err != nil {
		t.Fatalf("ScanDir() error = %v", err)
	}
	if err := WriteIndex(filepath.Join(root, model.MapDir), *shards); err != nil {
		t.Fatalf("WriteIndex() error = %v", err)
	}

	status, err := CheckStaleness(root)
	if err != nil {
		t.Fatalf("CheckStaleness() error = %v", err)
	}
	if !status.Fresh {
		t.Fatalf("Fresh = false, status = %#v", status)
	}
}

func TestCheckStalenessDetectsChangedMissingAndAddedFiles(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "main.go", "package main\n")
	testutil.WriteFile(t, root, "delete.go", "package main\n")

	shards, err := ScanDir(root)
	if err != nil {
		t.Fatalf("ScanDir() error = %v", err)
	}
	if err := WriteIndex(filepath.Join(root, model.MapDir), *shards); err != nil {
		t.Fatalf("WriteIndex() error = %v", err)
	}

	testutil.WriteFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	testutil.WriteFile(t, root, "added.go", "package main\n")
	if err := os.Remove(filepath.Join(root, "delete.go")); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	status, err := CheckStaleness(root)
	if err != nil {
		t.Fatalf("CheckStaleness() error = %v", err)
	}
	if status.Fresh {
		t.Fatal("Fresh = true, want stale")
	}
	assertPaths(t, "Changed", status.Changed, []string{"main.go"})
	assertPaths(t, "Missing", status.Missing, []string{"delete.go"})
	assertPaths(t, "Added", status.Added, []string{"added.go"})
}

func TestCheckStalenessMissingIndexIsStale(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "main.go", "package main\n")

	status, err := CheckStaleness(root)
	if err != nil {
		t.Fatalf("CheckStaleness() error = %v", err)
	}
	if status.Fresh || !status.MissingIndex {
		t.Fatalf("status = %#v, want missing stale index", status)
	}
}

func assertPaths(t testing.TB, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s = %#v, want %#v", label, got, want)
		}
	}
}
