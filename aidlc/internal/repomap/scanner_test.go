package repomap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
)

func TestScanDirFixtureRepo(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "repomap", "fixture-repo")

	shards, err := ScanDir(root)
	if err != nil {
		t.Fatalf("ScanDir() error = %v", err)
	}

	assertFile(t, shards.Files, "internal/core/core.go", "go")
	assertFile(t, shards.Files, "docs/spec/1000000000-add-auth.md", "markdown")
	assertImport(t, shards.Imports, "internal/auth/auth.go", "github.com/example/fixture/internal/core")
	assertTestLink(t, shards.Tests, "internal/core/core_test.go", "internal/core/core.go")
	assertDoc(t, shards.Docs, "docs/ARCHITECTURE.md", "architecture")
	assertDoc(t, shards.Docs, "docs/spec/1000000000-add-auth.md", "spec")
	assertChange(t, shards.Changes, "spec", "1000000000-add-auth", "approved")
	assertChange(t, shards.Changes, "adr", "1000000001-use-sqlite", "accepted")
	assertBlueprint(t, shards.Blueprints, "docs/blueprints/core.md", "Integration Boundaries")
	assertSourceChunk(t, shards.SourceChunks, "internal/auth/auth.go", "Authorize")
	assertSourceChunk(t, shards.SourceChunks, "internal/core/core.go", "NormalizeGreetingName")
	assertNoSourceChunk(t, shards.SourceChunks, "docs/spec/1000000000-add-auth.md")
	assertSymbol(t, shards.Symbols, "internal/auth/auth.go", "type", "SessionPolicy")
	assertSymbol(t, shards.Symbols, "internal/auth/auth.go", "func", "Authorize")
	assertSymbol(t, shards.Symbols, "internal/core/core.go", "func", "NormalizeGreetingName")

	for _, file := range shards.Files {
		if file.Path == "internal/core/core.go" {
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if file.ContentHash != model.ContentHash(content) {
				t.Fatalf("ContentHash = %s, want %s", file.ContentHash, model.ContentHash(content))
			}
		}
	}
}

func TestWriteShardsUsesDeterministicJSONL(t *testing.T) {
	dir := t.TempDir()
	shards := Shards{
		Files: []model.FileRecord{
			{Path: "z.go", Language: "go", ContentHash: "z"},
			{Path: "a.go", Language: "go", ContentHash: "a"},
		},
	}

	if err := WriteShards(dir, shards); err != nil {
		t.Fatalf("WriteShards() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, model.FilesShard))
	if err != nil {
		t.Fatalf("read files shard: %v", err)
	}
	if !bytes.HasSuffix(content, []byte("\n")) {
		t.Fatalf("files shard does not end with newline: %q", content)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"path":"a.go"`) || !strings.Contains(lines[1], `"path":"z.go"`) {
		t.Fatalf("files shard not sorted: %q", content)
	}

	sourceChunks, err := os.ReadFile(filepath.Join(dir, model.SourceChunksShard))
	if err != nil {
		t.Fatalf("read source chunks shard: %v", err)
	}
	if string(sourceChunks) != "" {
		t.Fatalf("source chunks shard = %q, want empty", sourceChunks)
	}

	symbols, err := os.ReadFile(filepath.Join(dir, model.SymbolsShard))
	if err != nil {
		t.Fatalf("read symbols shard: %v", err)
	}
	if string(symbols) != "" {
		t.Fatalf("symbols shard = %q, want empty", symbols)
	}
}

func TestScanDirWithOptionsDescendsOnlyIncludedDirsAndRootFiles(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "README.md", "# Root\n")
	testutil.WriteFile(t, root, "aidlc/main.go", "package main\n")
	testutil.WriteFile(t, root, "docs/spec/1000000000-change.md", "---\nstatus: approved\n---\n")
	testutil.WriteFile(t, root, "extra/ignored.go", "package extra\n")
	testutil.WriteFile(t, root, ".venv/site.py", "print('skip')\n")
	testutil.WriteFile(t, root, ".cursor/state.json", "{}\n")
	testutil.WriteFile(t, root, ".codex/log.txt", "skip\n")
	testutil.WriteFile(t, root, ".claude/log.txt", "skip\n")
	testutil.WriteFile(t, root, "node_modules/pkg/index.js", "skip\n")
	testutil.WriteFile(t, root, "vendor/pkg/file.go", "package vendor\n")
	testutil.WriteFile(t, root, "docs/map/index.json", "{}\n")

	shards, err := ScanDirWithOptions(root, ScanOptions{Include: []string{"docs", "aidlc"}})
	if err != nil {
		t.Fatalf("ScanDirWithOptions() error = %v", err)
	}

	assertFile(t, shards.Files, "README.md", "markdown")
	assertFile(t, shards.Files, "aidlc/main.go", "go")
	assertFile(t, shards.Files, "docs/spec/1000000000-change.md", "markdown")
	assertNoFile(t, shards.Files, "extra/ignored.go")
	assertNoFile(t, shards.Files, ".venv/site.py")
	assertNoFile(t, shards.Files, ".cursor/state.json")
	assertNoFile(t, shards.Files, ".codex/log.txt")
	assertNoFile(t, shards.Files, ".claude/log.txt")
	assertNoFile(t, shards.Files, "node_modules/pkg/index.js")
	assertNoFile(t, shards.Files, "vendor/pkg/file.go")
	assertNoFile(t, shards.Files, "docs/map/index.json")
	assertSourceChunk(t, shards.SourceChunks, "aidlc/main.go", "package main")
	assertNoSourceChunk(t, shards.SourceChunks, "docs/map/index.json")
}

func TestDetectIncludeCandidatesExcludesGeneratedDependencyAndAgentDirs(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{
		".ai", "aidlc", "docs", "src",
		".venv", ".claude", ".cursor", ".codex",
		"node_modules", "vendor", "build", "dist", ".cache",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	testutil.WriteFile(t, root, "README.md", "# Root\n")

	got, err := DetectIncludeCandidates(root)
	if err != nil {
		t.Fatalf("DetectIncludeCandidates() error = %v", err)
	}
	assertPaths(t, "candidates", got, []string{".ai", "aidlc", "docs", "src"})
}

func assertFile(t *testing.T, records []model.FileRecord, path, language string) {
	t.Helper()
	for _, record := range records {
		if record.Path == path && record.Language == language {
			return
		}
	}
	t.Fatalf("missing file record %s/%s in %#v", path, language, records)
}

func assertNoFile(t *testing.T, records []model.FileRecord, path string) {
	t.Helper()
	for _, record := range records {
		if record.Path == path {
			t.Fatalf("unexpected file record %s in %#v", path, records)
		}
	}
}

func assertImport(t *testing.T, records []model.ImportRecord, path, importPath string) {
	t.Helper()
	for _, record := range records {
		if record.Path == path && record.ImportPath == importPath {
			return
		}
	}
	t.Fatalf("missing import record %s -> %s in %#v", path, importPath, records)
}

func assertTestLink(t *testing.T, records []model.TestRecord, path, target string) {
	t.Helper()
	for _, record := range records {
		if record.Path == path && record.TargetPath == target {
			return
		}
	}
	t.Fatalf("missing test record %s -> %s in %#v", path, target, records)
}

func assertDoc(t *testing.T, records []model.DocRecord, path, kind string) {
	t.Helper()
	for _, record := range records {
		if record.Path == path && record.Kind == kind {
			return
		}
	}
	t.Fatalf("missing doc record %s/%s in %#v", path, kind, records)
}

func assertChange(t *testing.T, records []model.ChangeRecord, kind, id, status string) {
	t.Helper()
	for _, record := range records {
		if record.Kind == kind && record.ID == id && record.Status == status {
			return
		}
	}
	t.Fatalf("missing change record %s/%s/%s in %#v", kind, id, status, records)
}

func assertBlueprint(t *testing.T, records []model.BlueprintRecord, path, section string) {
	t.Helper()
	for _, record := range records {
		if record.Path == path && record.Section == section {
			return
		}
	}
	t.Fatalf("missing blueprint record %s/%s in %#v", path, section, records)
}

func assertSourceChunk(t *testing.T, records []model.SourceChunkRecord, path, text string) {
	t.Helper()
	for _, record := range records {
		if record.Path == path && strings.Contains(record.Text, text) {
			return
		}
	}
	t.Fatalf("missing source chunk %s containing %q in %#v", path, text, records)
}

func assertNoSourceChunk(t *testing.T, records []model.SourceChunkRecord, path string) {
	t.Helper()
	for _, record := range records {
		if record.Path == path {
			t.Fatalf("unexpected source chunk %s in %#v", path, records)
		}
	}
}

func assertSymbol(t *testing.T, records []model.SymbolRecord, path, kind, name string) {
	t.Helper()
	for _, record := range records {
		if record.Path == path && record.Kind == kind && record.Name == name {
			return
		}
	}
	t.Fatalf("missing symbol %s/%s/%s in %#v", path, kind, name, records)
}
