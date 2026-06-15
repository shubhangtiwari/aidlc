package sync_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/source"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
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

func TestWriteManifestPreservesExistingMapIncludeWhenUnset(t *testing.T) {
	target := t.TempDir()
	writeRawManifestJSON(t, target, `{
		"schema_version": 1,
		"upstream": {"source":"local","ref":"main","commit":"old"},
		"workspace": {"map": {"include":[" docs/./architecture ","aidlc\\internal"]}},
		"generated": {"ide":"codex","version":"old"},
		"files": [{"path":".ai/README.md","checksum":"sha256:old"}]
	}`)

	if err := templatesync.WriteManifest(target, contract.TargetManifest{
		Upstream:  contract.UpstreamRef{Source: "local", Ref: "main", Commit: "new"},
		Generated: contract.GenerationRecord{IDE: contract.IDECodex, Version: "new"},
		Files: []contract.ManifestFile{
			{Path: ".ai/README.md", Checksum: "sha256:new"},
		},
	}); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	read, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if read.Upstream.Commit != "new" {
		t.Fatalf("commit = %q, want new", read.Upstream.Commit)
	}
	if read.Generated.Version != "new" {
		t.Fatalf("generated version = %q, want new", read.Generated.Version)
	}
	assertStrings(t, read.Workspace.Map.Include, []string{"aidlc/internal", "docs/architecture"})
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

func TestReadManifestNormalizesMapInclude(t *testing.T) {
	target := t.TempDir()
	writeManifestJSON(t, target, contract.TargetManifestPath, contract.TargetManifest{
		Workspace: contract.WorkspaceRecord{
			Map: contract.MapWorkspaceRecord{
				Include: []string{" docs/./architecture ", "aidlc\\internal", "docs/architecture"},
			},
		},
	})

	read, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	assertStrings(t, read.Workspace.Map.Include, []string{"aidlc/internal", "docs/architecture"})
}

func TestReadManifestRejectsInvalidMapInclude(t *testing.T) {
	target := t.TempDir()
	writeManifestJSON(t, target, contract.TargetManifestPath, contract.TargetManifest{
		Workspace: contract.WorkspaceRecord{
			Map: contract.MapWorkspaceRecord{Include: []string{"docs/map"}},
		},
	})

	_, err := templatesync.ReadManifest(target)
	if err == nil {
		t.Fatal("read manifest succeeded, want invalid map include error")
	}
	if !strings.Contains(err.Error(), "must not include generated map artifacts") {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteManifestMapIncludePreservesExistingLockFields(t *testing.T) {
	target := t.TempDir()
	writeRawManifestJSON(t, target, `{
		"schema_version": 1,
		"upstream": {"source":"local","ref":"main","commit":"abc123"},
		"workspace": {
			"ides": ["codex"],
			"owner": "platform",
			"map": {"include":["old"],"checksum":"sha256:old"}
		},
		"generated": {"ide":"codex","version":"test"},
		"files": [{"path":".ai/README.md","checksum":"sha256:a"}],
		"metadata": {"fixture":"true"},
		"custom": {"kept": true}
	}`)

	if err := templatesync.WriteManifestMapInclude(
		target,
		[]string{" docs/./architecture ", "aidlc\\internal", "docs/architecture"},
	); err != nil {
		t.Fatalf("write map include: %v", err)
	}

	lock := readRawManifest(t, target)
	if lock["schema_version"].(float64) != float64(contract.TargetManifestVersion) {
		t.Fatalf("schema_version = %#v", lock["schema_version"])
	}
	assertJSONPathString(t, lock, "upstream", "commit", "abc123")
	assertJSONPathString(t, lock, "generated", "version", "test")
	assertJSONPathString(t, lock, "metadata", "fixture", "true")
	if custom := lock["custom"].(map[string]any); custom["kept"] != true {
		t.Fatalf("custom field not preserved: %#v", custom)
	}
	workspace := lock["workspace"].(map[string]any)
	if workspace["owner"] != "platform" {
		t.Fatalf("workspace owner = %#v", workspace["owner"])
	}
	mapRecord := workspace["map"].(map[string]any)
	if mapRecord["checksum"] != "sha256:old" {
		t.Fatalf("workspace.map checksum = %#v", mapRecord["checksum"])
	}
	assertAnyStrings(t, mapRecord["include"], []string{"aidlc/internal", "docs/architecture"})

	include, ok, err := templatesync.ReadManifestMapInclude(target)
	if err != nil {
		t.Fatalf("read map include: %v", err)
	}
	if !ok {
		t.Fatal("map include missing")
	}
	assertStrings(t, include, []string{"aidlc/internal", "docs/architecture"})
}

func TestWriteManifestMapIncludeInitializesMapOnlyLock(t *testing.T) {
	target := t.TempDir()

	if err := templatesync.WriteManifestMapInclude(target, []string{"docs"}); err != nil {
		t.Fatalf("write map include: %v", err)
	}

	lock := readRawManifest(t, target)
	if len(lock) != 2 {
		t.Fatalf("lock keys = %#v, want only schema_version and workspace", lock)
	}
	if lock["schema_version"].(float64) != float64(contract.TargetManifestVersion) {
		t.Fatalf("schema_version = %#v", lock["schema_version"])
	}
	workspace := lock["workspace"].(map[string]any)
	if len(workspace) != 1 {
		t.Fatalf("workspace = %#v, want only map", workspace)
	}
	mapRecord := workspace["map"].(map[string]any)
	assertAnyStrings(t, mapRecord["include"], []string{"docs"})
}

func TestWriteManifestMapIncludeRejectsInvalidFolders(t *testing.T) {
	for _, include := range []string{
		"",
		"/absolute",
		"../outside",
		"docs/map",
		".claude",
		".codex/state",
		".cursor",
		".venv/lib",
		"build",
		"cache/tmp",
		"dist",
		"node_modules/pkg",
		"out",
		"target/debug",
		"vendor",
	} {
		t.Run(include, func(t *testing.T) {
			err := templatesync.WriteManifestMapInclude(t.TempDir(), []string{include})
			if err == nil {
				t.Fatal("write map include succeeded, want error")
			}
		})
	}
}

func TestReadManifestMapIncludeTreatsMissingAsUnset(t *testing.T) {
	include, ok, err := templatesync.ReadManifestMapInclude(t.TempDir())
	if err != nil {
		t.Fatalf("read missing map include: %v", err)
	}
	if ok || include != nil {
		t.Fatalf("include = %#v, ok = %v; want unset", include, ok)
	}
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

func TestManifestFromAcceptedPlanExcludesConflictedPayloads(t *testing.T) {
	files := map[string]source.File{
		".ai/created.md":   {Path: ".ai/created.md", Content: []byte("created"), Mode: 0o644},
		".ai/skipped.md":   {Path: ".ai/skipped.md", Content: []byte("skipped"), Mode: 0o644},
		".ai/updated.md":   {Path: ".ai/updated.md", Content: []byte("updated"), Mode: 0o600},
		".ai/conflict.md":  {Path: ".ai/conflict.md", Content: []byte("upstream conflict"), Mode: 0o644},
		".ai/unmatched.md": {Path: ".ai/unmatched.md", Content: []byte("unmatched"), Mode: 0o644},
		".ai/undecided.md": {Path: ".ai/undecided.md", Content: []byte("undecided"), Mode: 0o644},
	}
	plan := templatesync.Plan{
		Upstream: contract.UpstreamRef{Source: "local", Ref: "main", Commit: "abc123"},
		Files:    files,
		Decisions: []templatesync.Decision{
			{
				Path:             ".ai/created.md",
				State:            templatesync.StateCreate,
				UpstreamChecksum: templatesync.BytesChecksum(files[".ai/created.md"].Content),
				Mode:             "0644",
			},
			{
				Path:             ".ai/skipped.md",
				State:            templatesync.StateSkip,
				LocalChecksum:    templatesync.BytesChecksum(files[".ai/skipped.md"].Content),
				UpstreamChecksum: templatesync.BytesChecksum(files[".ai/skipped.md"].Content),
				Mode:             "0644",
			},
			{
				Path:             ".ai/updated.md",
				State:            templatesync.StateUpdateClean,
				UpstreamChecksum: templatesync.BytesChecksum(files[".ai/updated.md"].Content),
				Mode:             "0600",
			},
			{
				Path:             ".ai/conflict.md",
				State:            templatesync.StateConflict,
				LocalChecksum:    templatesync.BytesChecksum([]byte("local conflict")),
				UpstreamChecksum: templatesync.BytesChecksum(files[".ai/conflict.md"].Content),
				Mode:             "0644",
			},
			{
				Path:             ".ai/unmatched.md",
				State:            templatesync.StateSkip,
				LocalChecksum:    templatesync.BytesChecksum([]byte("different")),
				UpstreamChecksum: templatesync.BytesChecksum(files[".ai/unmatched.md"].Content),
				Mode:             "0644",
			},
			{
				Path:             ".ai/removed.md",
				State:            templatesync.StateRemovedUpstream,
				LocalChecksum:    templatesync.BytesChecksum([]byte("local removed")),
				PreviousChecksum: templatesync.BytesChecksum([]byte("previous removed")),
			},
		},
	}

	manifest := templatesync.ManifestFromAcceptedPlan(
		plan,
		contract.GenerationRecord{IDE: contract.IDECodex, Version: "test"},
		map[string]string{"command": "init"},
	)

	if manifest.Upstream.Commit != "abc123" {
		t.Fatalf("commit = %q", manifest.Upstream.Commit)
	}
	if manifest.Metadata["command"] != "init" {
		t.Fatalf("metadata = %#v", manifest.Metadata)
	}
	assertIDESelection(t, manifest.Workspace.IDEs, []contract.IDE{contract.IDECodex})
	assertManifestFiles(t, manifest.Files, map[string]source.File{
		".ai/created.md": files[".ai/created.md"],
		".ai/skipped.md": files[".ai/skipped.md"],
		".ai/updated.md": files[".ai/updated.md"],
	})
}

func TestManifestFromPlanStillPersistsFullUpstreamFileSet(t *testing.T) {
	plan := templatesync.Plan{
		Upstream: contract.UpstreamRef{Source: "local", Ref: "main"},
		Files: map[string]source.File{
			".ai/conflict.md": {Path: ".ai/conflict.md", Content: []byte("conflict"), Mode: 0o644},
			".ai/removed.md":  {Path: ".ai/removed.md", Content: []byte("removed"), Mode: 0o644},
		},
		Decisions: []templatesync.Decision{
			{Path: ".ai/conflict.md", State: templatesync.StateConflict},
			{Path: ".ai/removed.md", State: templatesync.StateRemovedUpstream},
		},
	}

	manifest := templatesync.ManifestFromPlan(plan, contract.GenerationRecord{IDE: contract.IDECodex}, nil)

	assertManifestFiles(t, manifest.Files, plan.Files)
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

func writeRawManifestJSON(t *testing.T, target, content string) {
	t.Helper()
	path := filepath.Join(target, filepath.FromSlash(contract.TargetManifestPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func readRawManifest(t *testing.T, target string) map[string]any {
	t.Helper()
	content := testutilReadFile(t, target, contract.TargetManifestPath)
	var raw map[string]any
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		t.Fatalf("parse written manifest: %v", err)
	}
	return raw
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

func assertManifestFiles(t *testing.T, got []contract.ManifestFile, want map[string]source.File) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("manifest files = %#v, want %d files", got, len(want))
	}
	for _, file := range got {
		sourceFile, ok := want[file.Path]
		if !ok {
			t.Fatalf("unexpected manifest file %q in %#v", file.Path, got)
		}
		if file.Checksum != templatesync.BytesChecksum(sourceFile.Content) {
			t.Fatalf("%s checksum = %q", file.Path, file.Checksum)
		}
		if file.Mode != formatMode(sourceFile.Mode) {
			t.Fatalf("%s mode = %q", file.Path, file.Mode)
		}
	}
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("strings = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("strings = %#v, want %#v", got, want)
		}
	}
}

func assertAnyStrings(t *testing.T, got any, want []string) {
	t.Helper()
	values, ok := got.([]any)
	if !ok {
		t.Fatalf("value = %#v, want []any", got)
	}
	if len(values) != len(want) {
		t.Fatalf("strings = %#v, want %#v", values, want)
	}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("strings = %#v, want %#v", values, want)
		}
	}
}

func assertJSONPathString(t *testing.T, raw map[string]any, objectName, fieldName, want string) {
	t.Helper()
	object, ok := raw[objectName].(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", objectName, raw[objectName])
	}
	if object[fieldName] != want {
		t.Fatalf("%s.%s = %#v, want %q", objectName, fieldName, object[fieldName], want)
	}
}

func formatMode(mode os.FileMode) string {
	if mode == 0 {
		return ""
	}
	return fmt.Sprintf("%#o", mode.Perm())
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
