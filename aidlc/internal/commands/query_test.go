package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/cache"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

func TestRunQueryCLIFallsBackWhenCacheIsMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, filepath.FromSlash(model.MapDir))
	repomapTestWriteJSONL(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/auth.md", Kind: "spec", Title: "Auth spec", Text: "login token"},
	})

	var stdout, stderr bytes.Buffer
	code := RunQueryCLI(context.Background(), []string{"--dir", root, "--limit", "5", "auth"}, &stdout, &stderr, queryTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("RunQueryCLI() code = %d, stderr = %q", code, stderr.String())
	}
	want := "docs/spec/auth.md\t1.000000\tAuth spec\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunQueryCLIFallsBackWhenCacheExistsButIsUnusable(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, filepath.FromSlash(model.MapDir))
	repomapTestWriteJSONL(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/auth.md", Kind: "spec", Title: "Auth spec", Text: "login token"},
	})
	if err := os.WriteFile(filepath.Join(mapDir, model.SQLiteFilename), []byte("not sqlite"), 0o644); err != nil {
		t.Fatalf("write invalid sqlite cache: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunQueryCLI(context.Background(), []string{"--dir", root, "--limit", "5", "auth"}, &stdout, &stderr, queryTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("RunQueryCLI() code = %d, stderr = %q", code, stderr.String())
	}
	want := "docs/spec/auth.md\t1.000000\tAuth spec\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunQueryCLIUsesShardFilteredFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, filepath.FromSlash(model.MapDir))
	repomapTestWriteJSONL(t, mapDir, model.FilesShard, []model.FileRecord{
		{Path: "internal/auth/service.go", Language: "go", ContentHash: "auth"},
	})
	repomapTestWriteJSONL(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/auth.md", Kind: "spec", Title: "Auth spec", Text: "auth"},
	})

	var stdout, stderr bytes.Buffer
	code := RunQueryCLI(context.Background(), []string{"--dir", root, "--shard", "docs", "auth"}, &stdout, &stderr, queryTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("RunQueryCLI() code = %d, stderr = %q", code, stderr.String())
	}
	want := "docs/spec/auth.md\t1.000000\tAuth spec\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunQueryCLIHandlesQuestionLikeSourceChunkQuery(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, filepath.FromSlash(model.MapDir))
	repomapTestWriteJSONL(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{
			Path:      "internal/auth/auth.go",
			Language:  "go",
			StartLine: 7,
			EndLine:   9,
			Text:      "func Authorize(name string) string {\n\treturn core.Greet(NormalizePrincipal(name))\n}",
		},
		{
			Path:      "internal/core/core.go",
			Language:  "go",
			StartLine: 5,
			EndLine:   7,
			Text:      "func Greet(name string) string {\n\treturn \"hello \" + NormalizeGreetingName(name)\n}",
		},
	})
	if err := cache.NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("build cache: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunQueryCLI(
		context.Background(),
		[]string{"--dir", root, "--limit", "5", "where does Authorize NormalizePrincipal Greet?"},
		&stdout,
		&stderr,
		queryTestDependencies(),
	)
	if code != contract.ExitOK {
		t.Fatalf("RunQueryCLI() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "internal/auth/auth.go\t") {
		t.Fatalf("stdout = %q, want auth source chunk first", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Authorize") {
		t.Fatalf("stdout = %q, want source chunk snippet", stdout.String())
	}
}

func TestRunQueryCLIUsageErrors(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		nil,
		{"--limit", "-1", "auth"},
		{"--unknown"},
	} {
		var stdout, stderr bytes.Buffer
		code := RunQueryCLI(context.Background(), args, &stdout, &stderr, queryTestDependencies())
		if code != contract.ExitUsage {
			t.Fatalf("RunQueryCLI(%#v) code = %d, want %d", args, code, contract.ExitUsage)
		}
	}
}

func TestRunQueryReturnsSQLiteRankedOutputWhenCacheExists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, filepath.FromSlash(model.MapDir))
	repomapTestWriteJSONL(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/auth.md", Kind: "spec", Title: "Auth spec", Text: "auth auth login"},
		{Path: "docs/spec/billing.md", Kind: "spec", Title: "Billing", Text: "invoice"},
	})
	if err := cache.NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("build cache: %v", err)
	}

	text, err := RunQuery(context.Background(), QueryOptions{TargetDir: root, Query: "auth", Limit: 1}, queryTestDependencies())
	if err != nil {
		t.Fatalf("RunQuery() error = %v", err)
	}
	if !strings.HasPrefix(text, "docs/spec/auth.md\t") {
		t.Fatalf("RunQuery() text = %q, want auth spec first", text)
	}
}

func queryTestDependencies() QueryDependencies {
	return QueryDependencies{
		NewCacheQuerier: func(mapDir string) model.Querier {
			return cache.NewQuerier(mapDir)
		},
	}
}

func repomapTestWriteJSONL[T model.SortableRecord](t testing.TB, root, name string, records []T) {
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
