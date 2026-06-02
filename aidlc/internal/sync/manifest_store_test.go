package sync_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aidlc/ai-dlc-template/aidlc/internal/contract"
	"github.com/aidlc/ai-dlc-template/aidlc/internal/source"
	templatesync "github.com/aidlc/ai-dlc-template/aidlc/internal/sync"
)

func TestManifestStoreRoundTrip(t *testing.T) {
	target := t.TempDir()
	manifest := contract.TargetManifest{
		Upstream: contract.UpstreamRef{Source: "local", Ref: "main", Commit: "abc123"},
		Generated: contract.GenerationRecord{
			IDE:     contract.IDECodex,
			Version: "test",
		},
		Workspace: contract.WorkspaceRecord{
			IDEs: []contract.IDE{contract.IDECursor, contract.IDECodex, contract.IDECursor},
		},
		Files: []contract.ManifestFile{
			{Path: ".ai/README.md", Checksum: "sha256:b"},
			{Path: "LICENSE", Checksum: "sha256:a"},
		},
		Metadata: map[string]string{"fixture": "true"},
	}
	if err := templatesync.WriteManifest(target, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	read, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if read == nil {
		t.Fatal("manifest missing")
	}
	if read.SchemaVersion != contract.TargetManifestVersion {
		t.Fatalf("schema version = %d", read.SchemaVersion)
	}
	if read.Upstream.Commit != "abc123" {
		t.Fatalf("commit = %q", read.Upstream.Commit)
	}
	assertIDESelection(t, read.Workspace.IDEs, []contract.IDE{contract.IDECodex, contract.IDECursor})
	if len(read.Files) != 2 || read.Files[0].Path != ".ai/README.md" || read.Files[1].Path != "LICENSE" {
		t.Fatalf("files not sorted/persisted: %#v", read.Files)
	}
	assertExists(t, target, contract.TargetManifestPath)
	assertMissing(t, target, contract.LegacyTargetManifestPath)
}

func TestWriteManifestDoesNotFallBackToGeneratedIDE(t *testing.T) {
	target := t.TempDir()
	manifest := contract.TargetManifest{
		Upstream:  contract.UpstreamRef{Source: "local", Ref: "main", Commit: "abc123"},
		Generated: contract.GenerationRecord{IDE: contract.IDEAll, Version: "test"},
	}

	if err := templatesync.WriteManifest(target, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	content := testutilReadFile(t, target, contract.TargetManifestPath)
	var raw map[string]any
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		t.Fatalf("parse written manifest: %v", err)
	}
	workspace, ok := raw["workspace"].(map[string]any)
	if !ok {
		t.Fatalf("workspace = %#v, want object", raw["workspace"])
	}
	if _, exists := workspace["ides"]; exists {
		t.Fatalf("workspace.ides was written from generated.ide: %s", content)
	}

	read, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	assertIDESelection(t, read.Workspace.IDEs, nil)
}

func TestReadManifestTreatsMissingAsNil(t *testing.T) {
	read, err := templatesync.ReadManifest(t.TempDir())
	if err != nil {
		t.Fatalf("read missing manifest: %v", err)
	}
	if read != nil {
		t.Fatalf("read = %#v, want nil", read)
	}
}

func TestReadManifestBackfillsSchemaVersion(t *testing.T) {
	target := t.TempDir()
	path := filepath.Join(target, filepath.FromSlash(contract.TargetManifestPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"upstream":{"source":"local"},"files":[]}`), 0o644); err != nil {
		t.Fatalf("write legacy manifest: %v", err)
	}
	read, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read legacy manifest: %v", err)
	}
	if read.SchemaVersion != contract.TargetManifestVersion {
		t.Fatalf("schema version = %d", read.SchemaVersion)
	}
}

func TestReadManifestPrefersRootLockWhenLegacyManifestExists(t *testing.T) {
	target := t.TempDir()
	writeManifestJSON(t, target, contract.LegacyTargetManifestPath, contract.TargetManifest{
		Upstream:  contract.UpstreamRef{Commit: "legacy"},
		Generated: contract.GenerationRecord{IDE: contract.IDECodex},
		Workspace: contract.WorkspaceRecord{
			IDEs: []contract.IDE{contract.IDEClaude},
		},
	})
	writeManifestJSON(t, target, contract.TargetManifestPath, contract.TargetManifest{
		Upstream:  contract.UpstreamRef{Commit: "root"},
		Generated: contract.GenerationRecord{IDE: contract.IDECursor},
		Workspace: contract.WorkspaceRecord{
			IDEs: []contract.IDE{contract.IDEWindsurf, contract.IDECursor, contract.IDEWindsurf},
		},
	})

	read, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if read.Upstream.Commit != "root" {
		t.Fatalf("commit = %q", read.Upstream.Commit)
	}
	assertIDESelection(t, read.Workspace.IDEs, []contract.IDE{contract.IDECursor, contract.IDEWindsurf})
}

func TestReadManifestRootLockDoesNotFallBackToGeneratedIDE(t *testing.T) {
	target := t.TempDir()
	writeManifestJSON(t, target, contract.LegacyTargetManifestPath, contract.TargetManifest{
		Upstream:  contract.UpstreamRef{Commit: "legacy"},
		Generated: contract.GenerationRecord{IDE: contract.IDEClaude},
		Workspace: contract.WorkspaceRecord{
			IDEs: []contract.IDE{contract.IDEClaude},
		},
	})
	writeManifestJSON(t, target, contract.TargetManifestPath, contract.TargetManifest{
		Upstream:  contract.UpstreamRef{Commit: "root"},
		Generated: contract.GenerationRecord{IDE: contract.IDEAll},
	})

	read, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if read.Upstream.Commit != "root" {
		t.Fatalf("commit = %q", read.Upstream.Commit)
	}
	assertIDESelection(t, read.Workspace.IDEs, nil)
}

func TestReadManifestFallsBackToLegacyWorkspaceIDEs(t *testing.T) {
	target := t.TempDir()
	writeManifestJSON(t, target, contract.LegacyTargetManifestPath, contract.TargetManifest{
		Generated: contract.GenerationRecord{IDE: contract.IDECodex},
		Workspace: contract.WorkspaceRecord{
			IDEs: []contract.IDE{contract.IDEWindsurf, contract.IDEClaude, contract.IDEClaude},
		},
	})

	read, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	assertIDESelection(t, read.Workspace.IDEs, []contract.IDE{contract.IDEClaude, contract.IDEWindsurf})
}

func TestReadManifestFallsBackToLegacyGeneratedIDE(t *testing.T) {
	target := t.TempDir()
	writeManifestJSON(t, target, contract.LegacyTargetManifestPath, contract.TargetManifest{
		Generated: contract.GenerationRecord{IDE: contract.IDEAll},
	})

	read, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	assertIDESelection(t, read.Workspace.IDEs, contract.ConcreteIDEs())
}

func TestManifestFromPlanPersistsSourceChecksums(t *testing.T) {
	plan := templatesync.Plan{
		Upstream: contract.UpstreamRef{Source: "local", Ref: "main"},
		Files: map[string]source.File{
			"LICENSE":       {Path: "LICENSE", Content: []byte("license"), Mode: 0o644},
			".ai/README.md": {Path: ".ai/README.md", Content: []byte("readme"), Mode: 0o644},
		},
	}
	manifest := templatesync.ManifestFromPlan(plan, contract.GenerationRecord{IDE: contract.IDEAll}, nil)
	if len(manifest.Files) != 2 {
		t.Fatalf("files = %d", len(manifest.Files))
	}
	if manifest.Files[0].Path != ".ai/README.md" {
		t.Fatalf("first path = %q", manifest.Files[0].Path)
	}
	if manifest.Files[0].Checksum != templatesync.BytesChecksum([]byte("readme")) {
		t.Fatalf("checksum = %q", manifest.Files[0].Checksum)
	}
	assertIDESelection(t, manifest.Workspace.IDEs, contract.ConcreteIDEs())
}

func writeManifestJSON(t *testing.T, target, relativePath string, manifest contract.TargetManifest) {
	t.Helper()
	content, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	path := filepath.Join(target, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func assertIDESelection(t *testing.T, got, want []contract.IDE) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("workspace IDEs = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("workspace IDEs = %#v, want %#v", got, want)
		}
	}
}

func assertExists(t *testing.T, target, relativePath string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(relativePath))); err != nil {
		t.Fatalf("%s missing: %v", relativePath, err)
	}
}

func testutilReadFile(t *testing.T, target, relativePath string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(relativePath)))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return string(content)
}

func assertMissing(t *testing.T, target, relativePath string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(relativePath))); !os.IsNotExist(err) {
		t.Fatalf("%s exists or stat failed unexpectedly: %v", relativePath, err)
	}
}
