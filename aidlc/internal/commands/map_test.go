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
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
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
	testutil.WriteFile(t, root, "src/main.go", "package main\n")

	var stdout, stderr bytes.Buffer
	code := RunMapCLI(context.Background(), []string{"--dir", root, "--include", "src"}, &stdout, &stderr, mapTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("map code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "repo map: built\n") {
		t.Fatalf("stdout missing build summary:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "include: src\n") {
		t.Fatalf("stdout missing include summary:\n%s", stdout.String())
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

	testutil.WriteFile(t, root, "src/main.go", "package main\n\nfunc main() {}\n")
	stdout.Reset()
	stderr.Reset()
	code = RunMapCLI(context.Background(), []string{"--dir", root, "--check"}, &stdout, &stderr, mapTestDependencies())
	if code != contract.ExitConflict {
		t.Fatalf("stale check code = %d, want conflict; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "repo map: stale\n") || !strings.Contains(stdout.String(), "changed: src/main.go\n") {
		t.Fatalf("stale stdout missing changed file:\n%s", stdout.String())
	}
}

func TestMapCLIContinuesWhenCacheBuildFails(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "src/main.go", "package main\n")

	var stdout, stderr bytes.Buffer
	deps := mapTestDependencies()
	deps.CacheBuilder = failingCacheBuilder{err: errors.New("fts unavailable")}
	code := RunMapCLI(context.Background(), []string{"--dir", root, "--include", "src"}, &stdout, &stderr, deps)
	if code != contract.ExitOK {
		t.Fatalf("map code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "cache: unavailable: fts unavailable\n") {
		t.Fatalf("stdout missing cache failure status:\n%s", stdout.String())
	}
	assertMapFileExists(t, root, filepath.Join(model.MapDir, model.FilesShard))
	assertMapFileExists(t, root, filepath.Join(model.MapDir, model.IndexFilename))
}

func TestMapCLIExplicitIncludeSavesWhitelistAndBuildsSubset(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "go.mod", "module example.com/maptest\n")
	testutil.WriteFile(t, root, "src/main.go", "package main\n")
	testutil.WriteFile(t, root, "tmp/ignored.go", "package tmp\n")

	var stdout, stderr bytes.Buffer
	code := RunMapCLI(context.Background(), []string{"--dir", root, "--include", "src"}, &stdout, &stderr, mapTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("map code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	include, ok, err := templatesync.ReadManifestMapInclude(root)
	if err != nil {
		t.Fatalf("ReadManifestMapInclude() error = %v", err)
	}
	if !ok || strings.Join(include, ",") != "src" {
		t.Fatalf("saved include = %v, ok = %v; want [src], true", include, ok)
	}
	files := testutil.ReadFile(t, root, filepath.Join(model.MapDir, model.FilesShard))
	if !strings.Contains(files, `"path":"src/main.go"`) || strings.Contains(files, `"path":"tmp/ignored.go"`) {
		t.Fatalf("files shard did not honor include:\n%s", files)
	}
}

func TestMapCLIReusesSavedWhitelistWithoutPrompting(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "src/main.go", "package main\n")
	if err := templatesync.WriteManifestMapInclude(root, []string{"src"}); err != nil {
		t.Fatalf("WriteManifestMapInclude() error = %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunMapCLI(context.Background(), []string{"--dir", root}, &stdout, &stderr, MapDependencies{
		CacheBuilder:  cache.NewBuilder(),
		Stdin:         strings.NewReader("no\n"),
		IsInteractive: func() bool { return true },
	})
	if code != contract.ExitOK {
		t.Fatalf("map code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "repo map include candidates:") {
		t.Fatalf("saved whitelist should not prompt:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "include: src\n") {
		t.Fatalf("stdout missing saved include:\n%s", stdout.String())
	}
}

func TestMapCLIInteractiveFirstRunConfirmsSavesAndBuilds(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "src/main.go", "package main\n")
	testutil.WriteFile(t, root, "docs/readme.md", "# Docs\n")
	testutil.WriteFile(t, root, ".venv/ignored.py", "print('ignore')\n")

	var stdout, stderr bytes.Buffer
	code := RunMapCLI(context.Background(), []string{"--dir", root}, &stdout, &stderr, MapDependencies{
		CacheBuilder:  cache.NewBuilder(),
		Stdin:         strings.NewReader("yes\n"),
		IsInteractive: func() bool { return true },
	})
	if code != contract.ExitOK {
		t.Fatalf("map code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "repo map include candidates:\n  - docs\n  - src\n") {
		t.Fatalf("stdout missing candidates:\n%s", stdout.String())
	}
	include, ok, err := templatesync.ReadManifestMapInclude(root)
	if err != nil {
		t.Fatalf("ReadManifestMapInclude() error = %v", err)
	}
	if !ok || strings.Join(include, ",") != "docs,src" {
		t.Fatalf("saved include = %v, ok = %v; want [docs src], true", include, ok)
	}
}

func TestMapCLIFirstRunNonInteractiveRequiresInclude(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "src/main.go", "package main\n")

	var stdout, stderr bytes.Buffer
	code := RunMapCLI(context.Background(), []string{"--dir", root}, &stdout, &stderr, mapTestDependencies())
	if code != contract.ExitUsage {
		t.Fatalf("map code = %d, want usage; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "repo-map include whitelist is not configured") {
		t.Fatalf("stderr missing deterministic guidance:\n%s", stderr.String())
	}
	if _, ok, err := templatesync.ReadManifestMapInclude(root); err != nil || ok {
		t.Fatalf("first run should not write whitelist; ok = %v, err = %v", ok, err)
	}
}

func TestMapCLICheckRequiresSavedWhitelistAndDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "src/main.go", "package main\n")

	var stdout, stderr bytes.Buffer
	code := RunMapCLI(context.Background(), []string{"--dir", root, "--check"}, &stdout, &stderr, mapTestDependencies())
	if code != contract.ExitUsage {
		t.Fatalf("check code = %d, want usage; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "repo-map include whitelist is not configured") {
		t.Fatalf("stderr missing whitelist guidance:\n%s", stderr.String())
	}
	if _, ok, err := templatesync.ReadManifestMapInclude(root); err != nil || ok {
		t.Fatalf("--check should not write whitelist; ok = %v, err = %v", ok, err)
	}
}

func TestMapCLICheckReportsIncludeMismatch(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "src/main.go", "package main\n")
	testutil.WriteFile(t, root, "docs/readme.md", "# Docs\n")

	var stdout, stderr bytes.Buffer
	code := RunMapCLI(context.Background(), []string{"--dir", root, "--include", "src"}, &stdout, &stderr, mapTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("map code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if err := templatesync.WriteManifestMapInclude(root, []string{"docs"}); err != nil {
		t.Fatalf("WriteManifestMapInclude() error = %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = RunMapCLI(context.Background(), []string{"--dir", root, "--check"}, &stdout, &stderr, mapTestDependencies())
	if code != contract.ExitConflict {
		t.Fatalf("check code = %d, want conflict; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "include: mismatch\n") {
		t.Fatalf("stale stdout missing include mismatch:\n%s", stdout.String())
	}
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
	return MapDependencies{CacheBuilder: cache.NewBuilder(), IsInteractive: func() bool { return false }}
}

type failingCacheBuilder struct {
	err error
}

func (b failingCacheBuilder) Build(context.Context, string) error {
	return b.err
}
