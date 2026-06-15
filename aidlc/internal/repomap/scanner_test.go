package repomap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
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
