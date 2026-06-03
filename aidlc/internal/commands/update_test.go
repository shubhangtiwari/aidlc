package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
)

func TestUpdateAppliesCleanManifestAwareChangesAndExcludesImplementationFiles(t *testing.T) {
	sourceV1 := createTemplateSource(t)
	target := t.TempDir()
	if _, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV1, Ref: "v1"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}

	sourceV2 := createTemplateSource(t)
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\n## Main agent delegation\n\nUpdated guidance.\n")
	testutil.WriteFile(t, sourceV2, "aidlc/internal/commands/update.go", "new private source\n")
	testutil.WriteFile(t, sourceV2, ".ai/scripts/ai_init.sh", "retired shell init\n")
	testutil.WriteFile(t, sourceV2, ".ai/scripts/ai_update.sh", "retired shell update\n")
	testutil.WriteFile(t, sourceV2, ".github/workflows/aidlc-ci.yml", "new private ci\n")

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV2, Ref: "v2"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("unexpected conflict")
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "<!-- INIT:BEGIN -->\n\n## Main agent delegation\n\nUpdated guidance.\n" {
		t.Fatalf("README was not updated: %q", got)
	}
	assertMissing(t, target, "aidlc/internal/commands/update.go")
	assertMissing(t, target, ".ai/scripts/ai_init.sh")
	assertMissing(t, target, ".ai/scripts/ai_update.sh")
	assertMissing(t, target, ".github/workflows/aidlc-ci.yml")
	assertMissing(t, target, "docs/ARCHITECTURE.md")
	assertExists(t, target, contract.TargetManifestPath)
}

func TestUpdateRefusesDivergentOverwrite(t *testing.T) {
	sourceV1 := createTemplateSource(t)
	target := t.TempDir()
	if _, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV1, Ref: "v1"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	testutil.WriteFile(t, target, ".ai/README.md", "local edits\n")

	sourceV2 := createTemplateSource(t)
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\nupstream edits\n")

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV2, Ref: "v2"},
	})
	if err != nil {
		t.Fatalf("update conflict should be reported in plan, not as error: %v", err)
	}
	if !hasConflict(result.Plan) {
		t.Fatal("expected conflict")
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "local edits\n" {
		t.Fatalf("divergent file overwritten: %q", got)
	}
}

func TestUpdateDryRunDoesNotWrite(t *testing.T) {
	sourceV1 := createTemplateSource(t)
	target := t.TempDir()
	if _, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV1, Ref: "v1"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	sourceV2 := createTemplateSource(t)
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\nupdated\n")

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV2, Ref: "v2"},
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("update dry run: %v", err)
	}
	if len(result.Written) != 0 {
		t.Fatalf("dry run wrote files: %#v", result.Written)
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "<!-- INIT:BEGIN -->\n\n## Main agent delegation\n\nUse governed workflows.\n" {
		t.Fatalf("dry run changed README: %q", got)
	}
}

func TestUpdateUsesManifestConfiguredLocalSource(t *testing.T) {
	source := createTemplateSource(t)
	target := t.TempDir()
	if _, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: source, Ref: "v1"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	testutil.WriteFile(t, source, ".ai/README.md", "<!-- INIT:BEGIN -->\n\nconfigured source update\n")

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{TargetDir: target})
	if err != nil {
		t.Fatalf("update from configured source: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("unexpected conflict")
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "<!-- INIT:BEGIN -->\n\nconfigured source update\n" {
		t.Fatalf("README was not updated from configured source: %q", got)
	}
}

func TestUpdateCLIUsesManifestConfiguredLocalSourcePath(t *testing.T) {
	source := createTemplateSource(t)
	target := t.TempDir()
	if _, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: source, Ref: "v1"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	testutil.WriteFile(t, source, ".ai/README.md", "<!-- INIT:BEGIN -->\n\ncli configured source update\n")
	chdirForTest(t, target)

	var stdout, stderr bytes.Buffer
	code := RunUpdateCLI(context.Background(), nil, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("update cli code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "<!-- INIT:BEGIN -->\n\ncli configured source update\n" {
		t.Fatalf("README was not updated from configured source: %q", got)
	}
}

func TestUpdateReportsRemovedUpstreamWithoutDeletingLocalFile(t *testing.T) {
	sourceV1 := createTemplateSource(t)
	target := t.TempDir()
	if _, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV1, Ref: "v1"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}

	sourceV2 := createTemplateSource(t)
	testutil.WriteFile(t, sourceV2, ".ai/template-manifest.yaml", `schema_version: 1
payload:
  include:
    - .ai/README.md
    - .ai/models.defaults.toml
    - .ai/personas/architect.md
    - .ai/skills/classify-change.md
    - docs/adr/README.md
    - docs/blueprints/README.md
    - LICENSE
  exclude:
    - docs/spec/[0-9]*-*.md
    - docs/adr/[0-9]*-*.md
    - docs/blueprints/aidlc.md
    - docs/blueprints/template-payload.md
    - docs/ARCHITECTURE.md
    - docs/architecture/**
    - aidlc/**
    - .github/**
    - release/**
    - dist/**
    - build/**
    - aidlc/.goreleaser.yaml
    - aidlc/scripts/**
policy:
  allow_broad_directories: false
  public_docs_must_be_explicit: true
  reject_absolute_paths: true
  reject_parent_traversal: true
`)

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV2, Ref: "v2"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	states := map[string]templatesync.DecisionState{}
	for _, decision := range result.Plan.Decisions {
		states[decision.Path] = decision.State
	}
	if states["docs/spec/README.md"] != templatesync.StateRemovedUpstream {
		t.Fatalf("docs/spec/README.md state = %s", states["docs/spec/README.md"])
	}
	assertExists(t, target, "docs/spec/README.md")
}

func TestUpdateRegeneratesWorkspaceIDEsFromRootLock(t *testing.T) {
	sourceV1 := createTemplateSource(t)
	target := t.TempDir()
	if _, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV1, Ref: "v1"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	manifest := readTargetManifest(t, target)
	manifest.Workspace.IDEs = []contract.IDE{contract.IDECursor}
	if err := templatesync.WriteManifest(target, *manifest); err != nil {
		t.Fatalf("write root lock: %v", err)
	}

	sourceV2 := createTemplateSource(t)
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\n## Main agent delegation\n\nUpdated guidance.\n")

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV2, Ref: "v2"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("unexpected conflict")
	}
	assertGenerated(t, result.Generated, ".cursor/rules/core.mdc")
	assertNotGenerated(t, result.Generated, ".codex/agents/architect.toml")
	assertExists(t, target, ".cursor/rules/core.mdc")
	if got := testutil.ReadFile(t, target, "AGENTS.md"); !strings.Contains(got, "Updated guidance.") {
		t.Fatalf("generated AGENTS.md was not refreshed: %q", got)
	}
	assertIDESelection(t, readTargetManifest(t, target).Workspace.IDEs, []contract.IDE{contract.IDECursor})
}

func TestUpdatePreservesEmptyRootWorkspaceIDEs(t *testing.T) {
	sourceV1 := createTemplateSource(t)
	target := t.TempDir()
	if _, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV1, Ref: "v1"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	manifest := readTargetManifest(t, target)
	manifest.Workspace.IDEs = nil
	manifest.Generated.IDE = contract.IDEAll
	if err := templatesync.WriteManifest(target, *manifest); err != nil {
		t.Fatalf("write root lock: %v", err)
	}

	sourceV2 := createTemplateSource(t)
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\n## Main agent delegation\n\nUpdated guidance.\n")

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV2, Ref: "v2"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("unexpected conflict")
	}
	if len(result.Generated) != 0 {
		t.Fatalf("generated files from empty root workspace IDEs: %#v", result.Generated)
	}
	assertIDESelection(t, readTargetManifest(t, target).Workspace.IDEs, nil)
	lockContent := testutil.ReadFile(t, target, contract.TargetManifestPath)
	if strings.Contains(lockContent, `"ides"`) {
		t.Fatalf("root lock workspace.ides was backfilled from generated.ide:\n%s", lockContent)
	}
}

func TestUpdateFallsBackToLegacyIDESelectionAndWritesRootLock(t *testing.T) {
	sourceV1 := createTemplateSource(t)
	target := t.TempDir()
	if _, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV1, Ref: "v1"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	manifest := readTargetManifest(t, target)
	manifest.Workspace.IDEs = nil
	manifest.Generated.IDE = contract.IDEAll
	writeTargetManifestJSON(t, target, contract.LegacyTargetManifestPath, *manifest)
	removeTargetFile(t, target, contract.TargetManifestPath)

	sourceV2 := createTemplateSource(t)
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\nlegacy fallback update\n")

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV2, Ref: "v2"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("unexpected conflict")
	}
	assertGenerated(t, result.Generated, ".windsurfrules")
	assertGenerated(t, result.Generated, ".cursor/rules/core.mdc")
	assertExists(t, target, contract.TargetManifestPath)
	assertIDESelection(t, readTargetManifest(t, target).Workspace.IDEs, contract.ConcreteIDEs())
}

func TestUpdateDryRunDoesNotRegenerateOrMigrateLegacyLock(t *testing.T) {
	sourceV1 := createTemplateSource(t)
	target := t.TempDir()
	if _, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV1, Ref: "v1"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	manifest := readTargetManifest(t, target)
	writeTargetManifestJSON(t, target, contract.LegacyTargetManifestPath, *manifest)
	removeTargetFile(t, target, contract.TargetManifestPath)
	legacyBefore := testutil.ReadFile(t, target, contract.LegacyTargetManifestPath)
	generatedBefore := testutil.ReadFile(t, target, "AGENTS.md")

	sourceV2 := createTemplateSource(t)
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\ndry run update\n")

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV2, Ref: "v2"},
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("update dry run: %v", err)
	}
	if len(result.Generated) != 0 {
		t.Fatalf("dry run generated files: %#v", result.Generated)
	}
	assertMissing(t, target, contract.TargetManifestPath)
	if got := testutil.ReadFile(t, target, contract.LegacyTargetManifestPath); got != legacyBefore {
		t.Fatalf("legacy manifest changed on dry run:\n%s", got)
	}
	if got := testutil.ReadFile(t, target, "AGENTS.md"); got != generatedBefore {
		t.Fatalf("generated file changed on dry run: %q", got)
	}
}

func TestUpdateConflictDoesNotRegenerateOrRewriteLock(t *testing.T) {
	sourceV1 := createTemplateSource(t)
	target := t.TempDir()
	if _, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV1, Ref: "v1"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	lockBefore := testutil.ReadFile(t, target, contract.TargetManifestPath)
	generatedBefore := testutil.ReadFile(t, target, "AGENTS.md")
	testutil.WriteFile(t, target, ".ai/README.md", "local edits\n")

	sourceV2 := createTemplateSource(t)
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\nupstream edits\n")

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV2, Ref: "v2"},
	})
	if err != nil {
		t.Fatalf("update conflict should be reported in plan, not as error: %v", err)
	}
	if !hasConflict(result.Plan) {
		t.Fatal("expected conflict")
	}
	if len(result.Generated) != 0 {
		t.Fatalf("conflict generated files: %#v", result.Generated)
	}
	if got := testutil.ReadFile(t, target, contract.TargetManifestPath); got != lockBefore {
		t.Fatalf("root lock changed on conflict:\n%s", got)
	}
	if got := testutil.ReadFile(t, target, "AGENTS.md"); got != generatedBefore {
		t.Fatalf("generated file changed on conflict: %q", got)
	}
}

func TestUpdateCLIHelpOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := RunUpdateCLI(context.Background(), []string{"--help"}, &stdout, &stderr); code != contract.ExitOK {
		t.Fatalf("update help code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage: aidlc update") {
		t.Fatalf("help output missing: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "ai_update.sh") || strings.Contains(stdout.String(), "bash") {
		t.Fatalf("update help references retired shell compatibility: %q", stdout.String())
	}
}

func assertGenerated(t testing.TB, generated []string, name string) {
	t.Helper()
	for _, got := range generated {
		if got == name {
			return
		}
	}
	t.Fatalf("%s was not generated; generated = %#v", name, generated)
}

func assertNotGenerated(t testing.TB, generated []string, name string) {
	t.Helper()
	for _, got := range generated {
		if got == name {
			t.Fatalf("%s was generated unexpectedly; generated = %#v", name, generated)
		}
	}
}

func removeTargetFile(t testing.TB, target, name string) {
	t.Helper()
	if err := os.Remove(filepath.Join(target, filepath.FromSlash(name))); err != nil {
		t.Fatalf("remove %s: %v", name, err)
	}
}
