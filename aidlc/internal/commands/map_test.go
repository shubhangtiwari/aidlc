package commands

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/cache"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
)

func TestRunMapBuildsShardsIndexAndSQLiteCache(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "go.mod", "module example.com/maptest\n")
	testutil.WriteFile(t, root, "main.go", "package main\n\nfunc main() {}\n")

	result, err := RunMap(context.Background(), MapOptions{Dir: root}, mapTestDependencies())
	if err != nil {
		t.Fatalf("RunMap() error = %v", err)
	}
	if result.Files != 2 {
		t.Fatalf("Files = %d, want 2", result.Files)
	}

	assertMapFileExists(t, root, filepath.Join(model.MapDir, model.FilesShard))
	assertMapFileExists(t, root, filepath.Join(model.MapDir, model.IndexFilename))
	assertMapFileExists(t, root, filepath.Join(model.MapDir, model.SQLiteFilename))
	if got := testutil.ReadFile(t, root, filepath.Join(model.MapDir, model.IndexFilename)); !strings.Contains(got, `"content_hash"`) {
		t.Fatalf("index missing file hashes:\n%s", got)
	}
}

func TestMapCLICheckExitCodes(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "main.go", "package main\n")

	var stdout, stderr bytes.Buffer
	code := RunMapCLI(context.Background(), []string{"--dir", root}, &stdout, &stderr, mapTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("map code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "repo map: built\n") {
		t.Fatalf("stdout missing build summary:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = RunMapCLI(context.Background(), []string{"--dir", root, "--check"}, &stdout, &stderr, mapTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("fresh check code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if got := stdout.String(); got != "repo map: fresh\n" {
		t.Fatalf("fresh stdout = %q", got)
	}

	testutil.WriteFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	stdout.Reset()
	stderr.Reset()
	code = RunMapCLI(context.Background(), []string{"--dir", root, "--check"}, &stdout, &stderr, mapTestDependencies())
	if code != contract.ExitConflict {
		t.Fatalf("stale check code = %d, want conflict; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "repo map: stale\n") || !strings.Contains(stdout.String(), "changed: main.go\n") {
		t.Fatalf("stale stdout missing changed file:\n%s", stdout.String())
	}
}

func TestMapCLIContinuesWhenCacheBuildFails(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "main.go", "package main\n")

	var stdout, stderr bytes.Buffer
	code := RunMapCLI(context.Background(), []string{"--dir", root}, &stdout, &stderr, MapDependencies{
		CacheBuilder: failingCacheBuilder{err: errors.New("fts unavailable")},
	})
	if code != contract.ExitOK {
		t.Fatalf("map code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "cache: unavailable: fts unavailable\n") {
		t.Fatalf("stdout missing cache failure status:\n%s", stdout.String())
	}
	assertMapFileExists(t, root, filepath.Join(model.MapDir, model.FilesShard))
	assertMapFileExists(t, root, filepath.Join(model.MapDir, model.IndexFilename))
}

func TestMapCLIUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunMapCLI(context.Background(), []string{"extra"}, &stdout, &stderr, mapTestDependencies())
	if code != contract.ExitUsage {
		t.Fatalf("map usage code = %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage: aidlc map [flags]\n") {
		t.Fatalf("stderr missing usage:\n%s", stderr.String())
	}
}

func assertMapFileExists(t testing.TB, root, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
		t.Fatalf("expected %s to exist: %v", name, err)
	}
}

func mapTestDependencies() MapDependencies {
	return MapDependencies{CacheBuilder: cache.NewBuilder()}
}

type failingCacheBuilder struct {
	err error
}

func (b failingCacheBuilder) Build(context.Context, string) error {
	return b.err
}
