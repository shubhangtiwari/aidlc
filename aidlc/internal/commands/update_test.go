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

func TestUpdateForceOverwritesDivergentPayloadRegeneratesWorkspaceIDEAndWritesLock(t *testing.T) {
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
	testutil.WriteFile(t, target, ".ai/README.md", "local edits\n")

	sourceV2 := createTemplateSource(t)
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\nforced upstream edits\n")

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV2, Ref: "v2"},
		Force:     true,
	})
	if err != nil {
		t.Fatalf("update force: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("forced update reported conflicts")
	}
	states := map[string]templatesync.DecisionState{}
	for _, decision := range result.Plan.Decisions {
		states[decision.Path] = decision.State
	}
	if states[".ai/README.md"] != templatesync.StateOverwrite {
		t.Fatalf(".ai/README.md state = %s, want overwrite", states[".ai/README.md"])
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "<!-- INIT:BEGIN -->\n\nforced upstream edits\n" {
		t.Fatalf("README was not overwritten: %q", got)
	}
	assertGenerated(t, result.Generated, ".cursor/rules/core.mdc")
	assertNotGenerated(t, result.Generated, ".codex/agents/architect.toml")
	assertIDESelection(t, readTargetManifest(t, target).Workspace.IDEs, []contract.IDE{contract.IDECursor})
	if !manifestHasFile(readTargetManifest(t, target), ".ai/README.md") {
		t.Fatal("updated lock omitted overwritten payload path")
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
    - source: LICENSE
      target: licenses/aidlc.md
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

func TestUpdateFromHistoricalRootLicenseLockPreservesRootLicenseAndWritesMappedPayload(t *testing.T) {
	source := createTemplateSource(t)
	target := t.TempDir()
	testutil.WriteFile(t, target, "LICENSE", "consumer license\n")
	writeTargetManifestJSON(t, target, contract.TargetManifestPath, contract.TargetManifest{
		Upstream: contract.UpstreamRef{Source: "local", Ref: "v1", Commit: "v1"},
		Generated: contract.GenerationRecord{
			IDE: contract.IDECodex,
		},
		Workspace: contract.WorkspaceRecord{
			IDEs: []contract.IDE{contract.IDECodex},
		},
		Files: []contract.ManifestFile{
			{Path: "LICENSE", Checksum: templatesync.BytesChecksum([]byte("old aidlc license\n")), Mode: "0644"},
		},
		Metadata: map[string]string{"source_path": source},
	})

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: source, Ref: "v2"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("unexpected conflict")
	}
	states := map[string]templatesync.DecisionState{}
	for _, decision := range result.Plan.Decisions {
		states[decision.Path] = decision.State
	}
	if states["LICENSE"] != templatesync.StateRemovedUpstream {
		t.Fatalf("LICENSE state = %s, want removed-upstream", states["LICENSE"])
	}
	if states["licenses/aidlc.md"] != templatesync.StateCreate {
		t.Fatalf("licenses/aidlc.md state = %s, want create", states["licenses/aidlc.md"])
	}
	if got := testutil.ReadFile(t, target, "LICENSE"); got != "consumer license\n" {
		t.Fatalf("root LICENSE changed: %q", got)
	}
	if got := testutil.ReadFile(t, target, "licenses/aidlc.md"); got != "license\n" {
		t.Fatalf("mapped license payload = %q", got)
	}

	manifest := readTargetManifest(t, target)
	if manifestHasFile(manifest, "LICENSE") {
		t.Fatal("updated lock retained historical root LICENSE")
	}
	if !manifestHasFile(manifest, "licenses/aidlc.md") {
		t.Fatal("updated lock omitted mapped AIDLC license payload")
	}
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

func TestUpdateDryRunForceReportsOverwriteWithoutRegeneratingOrMigratingLegacyLock(t *testing.T) {
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
	testutil.WriteFile(t, target, ".ai/README.md", "local edits\n")

	sourceV2 := createTemplateSource(t)
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\nforced dry run update\n")

	result, err := RunUpdate(context.Background(), contract.UpdateOptions{
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: sourceV2, Ref: "v2"},
		DryRun:    true,
		Force:     true,
	})
	if err != nil {
		t.Fatalf("update dry-run force: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("dry-run forced update reported conflicts")
	}
	states := map[string]templatesync.DecisionState{}
	for _, decision := range result.Plan.Decisions {
		states[decision.Path] = decision.State
	}
	if states[".ai/README.md"] != templatesync.StateOverwrite {
		t.Fatalf(".ai/README.md state = %s, want overwrite", states[".ai/README.md"])
	}
	if len(result.Written) != 0 || len(result.Generated) != 0 {
		t.Fatalf("dry run force wrote files: %#v", result)
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "local edits\n" {
		t.Fatalf("dry run force changed README: %q", got)
	}
	assertMissing(t, target, contract.TargetManifestPath)
	if got := testutil.ReadFile(t, target, contract.LegacyTargetManifestPath); got != legacyBefore {
		t.Fatalf("legacy manifest changed on dry-run force:\n%s", got)
	}
	if got := testutil.ReadFile(t, target, "AGENTS.md"); got != generatedBefore {
		t.Fatalf("generated file changed on dry-run force: %q", got)
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
	if !strings.Contains(stdout.String(), "--force") || !strings.Contains(stdout.String(), "Overwrite divergent payload files") {
		t.Fatalf("help output missing force flag: %q", stdout.String())
	}
}

func TestUpdateCLIForcePrintsOverwriteAndExitsOK(t *testing.T) {
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
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\ncli forced upstream edits\n")
	chdirForTest(t, target)

	var stdout, stderr bytes.Buffer
	code := RunUpdateCLI(context.Background(), []string{"--force", "--source", "local", "--path", sourceV2, "--ref", "v2"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("update force cli code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "overwrite .ai/README.md force overwrites local file diverged from previous manifest\n") {
		t.Fatalf("output missing overwrite row:\n%s", output)
	}
	if strings.Contains(output, "conflict .ai/README.md") {
		t.Fatalf("output contains conflict row:\n%s", output)
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "<!-- INIT:BEGIN -->\n\ncli forced upstream edits\n" {
		t.Fatalf("README was not overwritten: %q", got)
	}
	assertExists(t, target, contract.TargetManifestPath)
}

func TestUpdateCLIDryRunForcePrintsOverwriteWithoutWriting(t *testing.T) {
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
	testutil.WriteFile(t, sourceV2, ".ai/README.md", "<!-- INIT:BEGIN -->\n\ncli dry-run forced upstream edits\n")
	chdirForTest(t, target)

	var stdout, stderr bytes.Buffer
	code := RunUpdateCLI(context.Background(), []string{"--dry-run", "--force", "--source", "local", "--path", sourceV2, "--ref", "v2"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("update dry-run force cli code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "overwrite .ai/README.md force overwrites local file diverged from previous manifest\n") {
		t.Fatalf("output missing overwrite row:\n%s", output)
	}
	if strings.Contains(output, "conflict .ai/README.md") || strings.Contains(output, "✓ written") || strings.Contains(output, "✦ generated") {
		t.Fatalf("dry-run force output contains unexpected sections:\n%s", output)
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "local edits\n" {
		t.Fatalf("dry run force changed README: %q", got)
	}
	if got := testutil.ReadFile(t, target, contract.TargetManifestPath); got != lockBefore {
		t.Fatalf("root lock changed on dry-run force:\n%s", got)
	}
	if got := testutil.ReadFile(t, target, "AGENTS.md"); got != generatedBefore {
		t.Fatalf("generated file changed on dry-run force: %q", got)
	}
}

func TestUpdateResultOutputUsesSharedOneLineFormatting(t *testing.T) {
	result := CommandResult{
		Mode: templatesync.ModeUpdate,
		Plan: templatesync.Plan{Decisions: []templatesync.Decision{
			{State: templatesync.StateUpdateClean, Path: ".ai/README.md", Reason: "local file matches previous manifest and upstream changed"},
		}},
		Written:   []string{".ai/README.md", contract.TargetManifestPath},
		Generated: []string{"AGENTS.md"},
	}

	var stdout bytes.Buffer
	printCommandResult(&stdout, result)
	output := stdout.String()
	for _, want := range []string{
		"◆ plan\n",
		"update-clean .ai/README.md local file matches previous manifest and upstream changed\n",
		"✓ written\n",
		"write .ai/README.md payload\n",
		"write aidlc.lock.json lock\n",
		"✦ generated\n",
		"generate AGENTS.md ide\n",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	for _, unwanted := range []string{"update plan:", "update dry run:", "  update-clean", "    local file"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("output contains retired formatting %q:\n%s", unwanted, output)
		}
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
