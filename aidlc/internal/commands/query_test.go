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

func TestRunQueryCLIAcceptsPlanJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, filepath.FromSlash(model.MapDir))
	repomapTestWriteJSONL(t, mapDir, model.SymbolsShard, []model.SymbolRecord{
		{Path: "internal/auth/auth.go", Language: "go", Kind: "function", Name: "Authorize", StartLine: 7, EndLine: 9},
	})
	repomapTestWriteJSONL(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{
			Path:      "internal/auth/auth.go",
			Language:  "go",
			StartLine: 7,
			EndLine:   9,
			Text:      "func Authorize(name string) string { return name }",
		},
	})

	plan := `{"version":1,"symbols":["Authorize"],"limit":5}`
	var stdout, stderr bytes.Buffer
	code := RunQueryCLI(context.Background(), []string{"--dir", root, "--plan-json", plan}, &stdout, &stderr, queryTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("RunQueryCLI() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "internal/auth/auth.go\t") {
		t.Fatalf("stdout = %q, want auth path first", stdout.String())
	}
}

func TestRunQueryCLIAcceptsPlanFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, filepath.FromSlash(model.MapDir))
	repomapTestWriteJSONL(t, mapDir, model.FilesShard, []model.FileRecord{
		{Path: "internal/auth/auth.go", Language: "go", ContentHash: "auth"},
		{Path: "internal/core/core.go", Language: "go", ContentHash: "core"},
	})
	planPath := filepath.Join(root, "plan.json")
	if err := os.WriteFile(planPath, []byte(`{"version":1,"paths":["internal/auth/auth.go"],"limit":5}`), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunQueryCLI(context.Background(), []string{"--dir", root, "--plan-file", planPath}, &stdout, &stderr, queryTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("RunQueryCLI() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "internal/auth/auth.go\t") {
		t.Fatalf("stdout = %q, want auth path first", stdout.String())
	}
}

func TestRunQueryCLIPlanUsesCacheTextAndFallbackPlanChannels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, filepath.FromSlash(model.MapDir))
	repomapTestWriteJSONL(t, mapDir, model.FilesShard, []model.FileRecord{
		{Path: "internal/direct.go", Language: "go", ContentHash: "direct"},
	})
	repomapTestWriteJSONL(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{
			Path:      "internal/cache_only.go",
			Language:  "go",
			StartLine: 4,
			EndLine:   8,
			Text:      "func CacheOnlyNeedle() string { return \"from sqlite fts\" }",
		},
	})
	if err := cache.NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("build cache: %v", err)
	}
	if err := os.Remove(filepath.Join(mapDir, model.SourceChunksShard)); err != nil {
		t.Fatalf("remove source chunks fallback shard: %v", err)
	}

	plan := `{"version":1,"terms":["CacheOnlyNeedle"],"paths":["internal/direct.go"],"limit":5}`
	var stdout, stderr bytes.Buffer
	code := RunQueryCLI(context.Background(), []string{"--dir", root, "--plan-json", plan}, &stdout, &stderr, queryTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("RunQueryCLI() code = %d, stderr = %q", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("stdout = %q, want cache and fallback results", stdout.String())
	}
	if !strings.HasPrefix(lines[0], "internal/direct.go\t") {
		t.Fatalf("stdout = %q, want direct fallback path first", stdout.String())
	}
	if !strings.Contains(stdout.String(), "internal/cache_only.go\t") {
		t.Fatalf("stdout = %q, want cache-backed FTS result", stdout.String())
	}
}

func TestRunQueryCLIShardFilteredPlanUsesCacheTextAndFallbackPlanChannels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, filepath.FromSlash(model.MapDir))
	repomapTestWriteJSONL(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{
			Path:      "internal/cache_only.go",
			Language:  "go",
			StartLine: 4,
			EndLine:   8,
			Text:      "func CacheOnlyNeedle() string { return \"from sqlite fts\" }",
		},
		{
			Path:      "internal/direct.go",
			Language:  "go",
			StartLine: 10,
			EndLine:   12,
			Text:      "func DirectPath() string { return \"from fallback path channel\" }",
		},
	})
	repomapTestWriteJSONL(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/outside.md", Kind: "spec", Title: "Outside shard", Text: "CacheOnlyNeedle outside requested shard"},
	})
	if err := cache.NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("build cache: %v", err)
	}
	repomapTestWriteJSONL(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{
			Path:      "internal/cache_only.go",
			Language:  "go",
			StartLine: 4,
			EndLine:   8,
			Text:      "func CacheOnly() string { return \"fallback no longer has the needle\" }",
		},
		{
			Path:      "internal/direct.go",
			Language:  "go",
			StartLine: 10,
			EndLine:   12,
			Text:      "func DirectPath() string { return \"from fallback path channel\" }",
		},
	})

	plan := `{"version":1,"terms":["CacheOnlyNeedle"],"paths":["internal/direct.go"],"shards":["source_chunks"],"limit":5}`
	var stdout, stderr bytes.Buffer
	code := RunQueryCLI(context.Background(), []string{"--dir", root, "--plan-json", plan}, &stdout, &stderr, queryTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("RunQueryCLI() code = %d, stderr = %q", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout = %q, want exactly cache and fallback results", stdout.String())
	}
	if !strings.HasPrefix(lines[0], "internal/direct.go\t") {
		t.Fatalf("stdout = %q, want direct fallback path first", stdout.String())
	}
	if !strings.HasPrefix(lines[1], "internal/cache_only.go\t") {
		t.Fatalf("stdout = %q, want cache-backed FTS result second", stdout.String())
	}
	if strings.Contains(stdout.String(), "docs/spec/outside.md\t") {
		t.Fatalf("stdout = %q, want cache results filtered to source_chunks shard", stdout.String())
	}
	if strings.Count(stdout.String(), "internal/direct.go\t") != 1 || strings.Count(stdout.String(), "internal/cache_only.go\t") != 1 {
		t.Fatalf("stdout = %q, want de-duplicated paths", stdout.String())
	}
}

func TestRunQueryCLIPlanFiltersCacheTextByLanguage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, filepath.FromSlash(model.MapDir))
	repomapTestWriteJSONL(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{
			Path:      "internal/allowed.go",
			Language:  "go",
			StartLine: 1,
			EndLine:   3,
			Text:      "func AllowedNeedle() string { return \"go\" }",
		},
		{
			Path:      "scripts/leak.py",
			Language:  "python",
			StartLine: 1,
			EndLine:   3,
			Text:      "def AllowedNeedle(): return 'python'",
		},
	})
	if err := cache.NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("build cache: %v", err)
	}
	repomapTestWriteJSONL(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{Path: "internal/allowed.go", Language: "go", StartLine: 1, EndLine: 3, Text: "func Allowed() string { return \"go\" }"},
		{Path: "scripts/leak.py", Language: "python", StartLine: 1, EndLine: 3, Text: "def allowed(): return 'python'"},
	})

	plan := `{"version":1,"terms":["AllowedNeedle"],"languages":["go"],"limit":5}`
	var stdout, stderr bytes.Buffer
	code := RunQueryCLI(context.Background(), []string{"--dir", root, "--plan-json", plan}, &stdout, &stderr, queryTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("RunQueryCLI() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "internal/allowed.go\t") {
		t.Fatalf("stdout = %q, want cache-backed go result", stdout.String())
	}
	if strings.Contains(stdout.String(), "scripts/leak.py\t") {
		t.Fatalf("stdout = %q, want python cache result excluded by language filter", stdout.String())
	}
}

func TestRunQueryCLIPlanFiltersCacheTextWhenTestsExcluded(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, filepath.FromSlash(model.MapDir))
	repomapTestWriteJSONL(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{
			Path:      "internal/allowed.go",
			Language:  "go",
			StartLine: 1,
			EndLine:   3,
			Text:      "func RuntimeNeedle() string { return \"runtime\" }",
		},
		{
			Path:      "internal/allowed_test.go",
			Language:  "go",
			StartLine: 1,
			EndLine:   3,
			Text:      "func TestRuntimeNeedle(t *testing.T) {}",
		},
	})
	if err := cache.NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("build cache: %v", err)
	}
	repomapTestWriteJSONL(t, mapDir, model.SourceChunksShard, []model.SourceChunkRecord{
		{Path: "internal/allowed.go", Language: "go", StartLine: 1, EndLine: 3, Text: "func Runtime() string { return \"runtime\" }"},
		{Path: "internal/allowed_test.go", Language: "go", StartLine: 1, EndLine: 3, Text: "func TestRuntime(t *testing.T) {}"},
	})

	plan := `{"version":1,"terms":["RuntimeNeedle"],"include_tests":false,"limit":5}`
	var stdout, stderr bytes.Buffer
	code := RunQueryCLI(context.Background(), []string{"--dir", root, "--plan-json", plan}, &stdout, &stderr, queryTestDependencies())
	if code != contract.ExitOK {
		t.Fatalf("RunQueryCLI() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "internal/allowed.go\t") {
		t.Fatalf("stdout = %q, want non-test cache result", stdout.String())
	}
	if strings.Contains(stdout.String(), "internal/allowed_test.go\t") {
		t.Fatalf("stdout = %q, want test cache result excluded by include_tests=false", stdout.String())
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

func TestRunQueryCLIPlanValidationErrorsExitUsage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "malformed json",
			args: []string{"--dir", root, "--plan-json", `{"version":1`},
			want: "malformed JSON",
		},
		{
			name: "absolute path",
			args: []string{"--dir", root, "--plan-json", `{"version":1,"paths":["/tmp/auth.go"]}`},
			want: "slash-relative",
		},
		{
			name: "bad glob",
			args: []string{"--dir", root, "--plan-json", `{"version":1,"globs":["internal/[.go"]}`},
			want: "invalid",
		},
		{
			name: "unknown shard",
			args: []string{"--dir", root, "--plan-json", `{"version":1,"shards":["missing"]}`},
			want: "unknown repo-map shard",
		},
		{
			name: "relationship depth",
			args: []string{"--dir", root, "--plan-json", `{"version":1,"relationship_depth":3}`},
			want: "relationship_depth",
		},
		{
			name: "raw args with plan",
			args: []string{"--dir", root, "--plan-json", `{"version":1}`, "auth"},
			want: "raw search terms cannot be combined",
		},
		{
			name: "shard with plan",
			args: []string{"--dir", root, "--shard", "docs", "--plan-json", `{"version":1}`},
			want: "--shard cannot be combined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunQueryCLI(context.Background(), tt.args, &stdout, &stderr, queryTestDependencies())
			if code != contract.ExitUsage {
				t.Fatalf("RunQueryCLI() code = %d, want %d; stderr = %q", code, contract.ExitUsage, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want containing %q", stderr.String(), tt.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
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
