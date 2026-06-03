package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/cli"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
)

func TestCLIInitThenUpdateUsesManifestAndKeepsPrivateRepoFilesOut(t *testing.T) {
	sourceV1 := createIntegrationSource(t, "v1")
	target := t.TempDir()
	testutil.WriteFile(t, target, "LICENSE", "consumer license\n")

	runCLIInDir(t, target, contract.ExitOK, "init", "all", "--source", "local", "--path", sourceV1, "--ref", "v1")

	if got := testutil.ReadFile(t, target, "LICENSE"); got != "consumer license\n" {
		t.Fatalf("consumer root LICENSE changed: %q", got)
	}
	for _, name := range publicPayloadPaths() {
		assertExists(t, target, name)
	}
	for _, name := range generatedIDEPaths() {
		assertExists(t, target, name)
	}
	assertExists(t, target, contract.TargetManifestPath)
	assertPrivatePathsMissing(t, target)

	manifest, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if manifest == nil {
		t.Fatal("manifest was not written")
	}
	assertIDESelection(t, manifest.Workspace.IDEs, contract.ConcreteIDEs())
	if manifest.Upstream.Source != "local" || manifest.Upstream.Ref != "v1" {
		t.Fatalf("manifest upstream = %#v", manifest.Upstream)
	}
	if manifest.Metadata["source_path"] != sourceV1 {
		t.Fatalf("manifest source_path = %q, want %q", manifest.Metadata["source_path"], sourceV1)
	}

	sourceV2 := createIntegrationSource(t, "v2")
	testutil.WriteFile(t, sourceV2, ".ai/README.md", readme("updated shared guidance"))
	testutil.WriteFile(t, sourceV2, ".ai/skills/classify-change.md", skillDoc("classify-change", "Classifies governed changes.", "Updated classify body."))
	testutil.WriteFile(t, sourceV2, "docs/spec/1780346463-add-aidlc-cli.md", "private spec changed in source\n")
	testutil.WriteFile(t, sourceV2, ".ai/scripts/ai_init.sh", "retired shell init changed in source\n")
	testutil.WriteFile(t, sourceV2, ".ai/scripts/ai_update.sh", "retired shell update changed in source\n")
	testutil.WriteFile(t, sourceV2, "aidlc/internal/commands/update.go", "private CLI source changed in source\n")
	testutil.WriteFile(t, sourceV2, ".github/workflows/aidlc-ci.yml", "private CI changed in source\n")

	runCLIInDir(t, target, contract.ExitOK, "update", "--source", "local", "--path", sourceV2, "--ref", "v2")

	if got := testutil.ReadFile(t, target, ".ai/README.md"); !strings.Contains(got, "updated shared guidance") {
		t.Fatalf("updated payload was not applied:\n%s", got)
	}
	if got := testutil.ReadFile(t, target, ".ai/skills/classify-change.md"); !strings.Contains(got, "Updated classify body.") {
		t.Fatalf("updated skill was not applied:\n%s", got)
	}
	if got := testutil.ReadFile(t, target, "LICENSE"); got != "consumer license\n" {
		t.Fatalf("consumer root LICENSE changed on update: %q", got)
	}
	for _, name := range generatedIDEPaths() {
		assertExists(t, target, name)
	}
	assertPrivatePathsMissing(t, target)

	updatedManifest, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read updated manifest: %v", err)
	}
	if updatedManifest.Upstream.Ref != "v2" {
		t.Fatalf("updated manifest ref = %q, want v2", updatedManifest.Upstream.Ref)
	}
	for _, file := range updatedManifest.Files {
		assertPublicManifestPath(t, file.Path)
	}
}

func TestCLIUpdateRegeneratesOnlyStoredWorkspaceIDEs(t *testing.T) {
	sourceV1 := createIntegrationSource(t, "v1")
	target := t.TempDir()

	runCLIInDir(t, target, contract.ExitOK, "init", "cursor", "--source", "local", "--path", sourceV1, "--ref", "v1")

	manifest, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	assertIDESelection(t, manifest.Workspace.IDEs, []contract.IDE{contract.IDECursor})
	assertExists(t, target, contract.TargetManifestPath)
	assertMissing(t, target, contract.LegacyTargetManifestPath)
	assertExists(t, target, "AGENTS.md")
	assertExists(t, target, ".cursor/rules/core.mdc")
	assertMissing(t, target, ".codex/agents/architect.toml")
	assertMissing(t, target, ".claude/agents/architect.md")
	assertMissing(t, target, "CLAUDE.md")
	assertMissing(t, target, ".github/copilot-instructions.md")
	assertMissing(t, target, ".windsurfrules")

	sourceV2 := createIntegrationSource(t, "v2")
	testutil.WriteFile(t, sourceV2, ".ai/README.md", readme("updated cursor guidance"))
	testutil.WriteFile(t, sourceV2, ".ai/skills/classify-change.md", skillDoc("classify-change", "Classifies governed changes.", "Updated cursor skill body."))

	runCLIInDir(t, target, contract.ExitOK, "update", "--source", "local", "--path", sourceV2, "--ref", "v2")

	if got := testutil.ReadFile(t, target, "AGENTS.md"); !strings.Contains(got, "updated cursor guidance") {
		t.Fatalf("AGENTS.md was not regenerated from updated guidance:\n%s", got)
	}
	if got := testutil.ReadFile(t, target, ".cursor/skills/classify-change/SKILL.md"); !strings.Contains(got, "Updated cursor skill body.") {
		t.Fatalf("cursor skill was not regenerated:\n%s", got)
	}
	assertMissing(t, target, ".codex/agents/architect.toml")
	assertMissing(t, target, ".claude/agents/architect.md")
	assertMissing(t, target, "CLAUDE.md")
	assertMissing(t, target, ".github/copilot-instructions.md")
	assertMissing(t, target, ".windsurfrules")

	updatedManifest, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read updated manifest: %v", err)
	}
	assertIDESelection(t, updatedManifest.Workspace.IDEs, []contract.IDE{contract.IDECursor})
	if updatedManifest.Upstream.Ref != "v2" {
		t.Fatalf("updated manifest ref = %q, want v2", updatedManifest.Upstream.Ref)
	}
}

func TestCLIUpdateFallsBackToLegacyManifestAndWritesRootLockAfterCleanRun(t *testing.T) {
	sourceV1 := createIntegrationSource(t, "v1")
	target := t.TempDir()
	runCLIInDir(t, target, contract.ExitOK, "init", "codex", "--source", "local", "--path", sourceV1, "--ref", "v1")

	manifest, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	manifest.Workspace.IDEs = []contract.IDE{contract.IDEWindsurf}
	writeLegacyManifestJSON(t, target, *manifest)
	removeTargetFile(t, target, contract.TargetManifestPath)
	legacyBefore := testutil.ReadFile(t, target, contract.LegacyTargetManifestPath)

	sourceV2 := createIntegrationSource(t, "v2")
	testutil.WriteFile(t, sourceV2, ".ai/README.md", readme("legacy fallback guidance"))

	runCLIInDir(t, target, contract.ExitOK, "update", "--dry-run", "--source", "local", "--path", sourceV2, "--ref", "v2")
	assertMissing(t, target, contract.TargetManifestPath)
	if got := testutil.ReadFile(t, target, contract.LegacyTargetManifestPath); got != legacyBefore {
		t.Fatalf("legacy manifest changed on dry run:\n%s", got)
	}
	assertMissing(t, target, ".windsurfrules")

	runCLIInDir(t, target, contract.ExitOK, "update", "--source", "local", "--path", sourceV2, "--ref", "v2")

	assertExists(t, target, contract.TargetManifestPath)
	if got := testutil.ReadFile(t, target, ".windsurfrules"); !strings.Contains(got, "legacy fallback guidance") {
		t.Fatalf("windsurf rules were not regenerated from legacy fallback:\n%s", got)
	}
	assertMissing(t, target, ".cursor/rules/core.mdc")
	assertMissing(t, target, ".github/copilot-instructions.md")
	updatedManifest, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read updated manifest: %v", err)
	}
	assertIDESelection(t, updatedManifest.Workspace.IDEs, []contract.IDE{contract.IDEWindsurf})
	if updatedManifest.Upstream.Ref != "v2" {
		t.Fatalf("updated manifest ref = %q, want v2", updatedManifest.Upstream.Ref)
	}
}

func TestCLIUpdateReportsConflictWithoutOverwritingOrLeakingPrivateFiles(t *testing.T) {
	sourceV1 := createIntegrationSource(t, "v1")
	target := t.TempDir()
	runCLIInDir(t, target, contract.ExitOK, "init", "codex", "--source", "local", "--path", sourceV1, "--ref", "v1")

	testutil.WriteFile(t, target, ".ai/README.md", "local project edits\n")

	sourceV2 := createIntegrationSource(t, "v2")
	testutil.WriteFile(t, sourceV2, ".ai/README.md", readme("upstream edits"))
	testutil.WriteFile(t, sourceV2, "docs/ARCHITECTURE.md", "private architecture changed in source\n")
	testutil.WriteFile(t, sourceV2, "aidlc/.goreleaser.yaml", "private release changed in source\n")

	stdout, stderr, code := runCLIInDir(t, target, contract.ExitConflict, "update", "--source", "local", "--path", sourceV2, "--ref", "v2")
	if code != contract.ExitConflict {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "conflict") {
		t.Fatalf("stdout missing conflict decision:\n%s", stdout)
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "local project edits\n" {
		t.Fatalf("conflicted file overwritten: %q", got)
	}
	assertPrivatePathsMissing(t, target)
}

func TestCLIInitWithConflictsWritesSafePayloadGeneratedFilesAndPartialLock(t *testing.T) {
	source := createIntegrationSource(t, "v1")
	target := t.TempDir()
	testutil.WriteFile(t, target, "licenses/aidlc.md", "local project edits\n")

	stdout, stderr, code := runCLIInDir(t, target, contract.ExitConflict, "init", "codex", "--source", "local", "--path", source, "--ref", "v1")
	if code != contract.ExitConflict {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "◆ plan") || !strings.Contains(stdout, "✓ written") || !strings.Contains(stdout, "✦ generated") {
		t.Fatalf("stdout missing formatted sections:\n%s", stdout)
	}
	if !strings.Contains(stdout, "conflict licenses/aidlc.md") {
		t.Fatalf("stdout missing conflicted payload path:\n%s", stdout)
	}

	if got := testutil.ReadFile(t, target, "licenses/aidlc.md"); got != "local project edits\n" {
		t.Fatalf("conflicted mapped license overwritten: %q", got)
	}
	assertMissing(t, target, "LICENSE")
	assertExists(t, target, ".ai/README.md")
	assertExists(t, target, ".ai/models.defaults.toml")
	assertExists(t, target, ".ai/references/architectures/tiered-service/architecture.md")
	assertExists(t, target, ".ai/references/architectures/data-engineering/blueprint-template.md")
	assertExists(t, target, ".codex/agents/architect.toml")
	assertExists(t, target, "AGENTS.md")
	assertExists(t, target, contract.TargetManifestPath)
	assertPrivatePathsMissing(t, target)

	manifest, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	assertIDESelection(t, manifest.Workspace.IDEs, []contract.IDE{contract.IDECodex})
	for _, file := range manifest.Files {
		if file.Path == "LICENSE" {
			t.Fatalf("manifest recorded root LICENSE path: %#v", manifest.Files)
		}
		assertPublicManifestPath(t, file.Path)
		if strings.Contains(file.Path, source) || filepath.IsAbs(file.Path) {
			t.Fatalf("manifest leaked private source path %q", file.Path)
		}
	}
	assertManifestIncludes(t, manifest, ".ai/README.md")
	assertManifestIncludes(t, manifest, ".ai/models.defaults.toml")
	assertManifestIncludes(t, manifest, ".ai/references/architectures/tiered-service/architecture.md")
	if manifestHasFile(manifest, "licenses/aidlc.md") {
		t.Fatalf("manifest recorded conflicted mapped license payload path: %#v", manifest.Files)
	}
}

func runCLIInDir(t *testing.T, dir string, wantCode int, args ...string) (string, string, int) {
	t.Helper()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd %s: %v", oldWD, err)
		}
	})

	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), args, &stdout, &stderr)
	if code != wantCode {
		t.Fatalf("aidlc %s code = %d, want %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), code, wantCode, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String(), code
}

func createIntegrationSource(t testing.TB, label string) string {
	t.Helper()

	root := t.TempDir()
	testutil.WriteFile(t, root, ".ai/template-manifest.yaml", templateManifest())
	for _, path := range sourcePayloadPaths() {
		writePublicPayloadFile(t, root, path, label)
	}
	writePrivateRepoFiles(t, root)
	return root
}

func templateManifest() string {
	return `schema_version: 1
payload:
  include:
    - .ai/README.md
    - .ai/models.defaults.toml
    - .ai/personas/architect.md
    - .ai/personas/implementer.md
    - .ai/personas/reviewer.md
    - .ai/references/architecture-template.md
    - .ai/references/architectures/data-engineering/architecture.md
    - .ai/references/architectures/data-engineering/blueprint-template.md
    - .ai/references/architectures/data-science/architecture.md
    - .ai/references/architectures/data-science/blueprint-template.md
    - .ai/references/architectures/minimal-tooling/architecture.md
    - .ai/references/architectures/polyglot-monorepo/architecture.md
    - .ai/references/architectures/polyglot-monorepo/blueprint-template.md
    - .ai/references/architectures/tiered-service/architecture.md
    - .ai/references/architectures/tiered-service/blueprint-template.md
    - .ai/references/ci/github-finalize-spec.yml
    - .ai/references/ci/gitlab-finalize-spec.yml
    - .ai/scripts/finalize_spec.sh
    - .ai/skills/classify-change.md
    - .ai/skills/init-architecture.md
    - .ai/skills/orchestrate-spec.md
    - .ai/templates/approval-brief.md
    - .ai/templates/spec.md
    - docs/spec/README.md
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
`
}

func publicPayloadPaths() []string {
	return []string{
		".ai/README.md",
		".ai/models.defaults.toml",
		".ai/personas/architect.md",
		".ai/personas/implementer.md",
		".ai/personas/reviewer.md",
		".ai/references/architecture-template.md",
		".ai/references/architectures/data-engineering/architecture.md",
		".ai/references/architectures/data-engineering/blueprint-template.md",
		".ai/references/architectures/data-science/architecture.md",
		".ai/references/architectures/data-science/blueprint-template.md",
		".ai/references/architectures/minimal-tooling/architecture.md",
		".ai/references/architectures/polyglot-monorepo/architecture.md",
		".ai/references/architectures/polyglot-monorepo/blueprint-template.md",
		".ai/references/architectures/tiered-service/architecture.md",
		".ai/references/architectures/tiered-service/blueprint-template.md",
		".ai/references/ci/github-finalize-spec.yml",
		".ai/references/ci/gitlab-finalize-spec.yml",
		".ai/scripts/finalize_spec.sh",
		".ai/skills/classify-change.md",
		".ai/skills/init-architecture.md",
		".ai/skills/orchestrate-spec.md",
		".ai/templates/approval-brief.md",
		".ai/templates/spec.md",
		"docs/spec/README.md",
		"docs/adr/README.md",
		"docs/blueprints/README.md",
		"licenses/aidlc.md",
	}
}

func sourcePayloadPaths() []string {
	paths := publicPayloadPaths()
	for i, path := range paths {
		if path == "licenses/aidlc.md" {
			paths[i] = "LICENSE"
		}
	}
	return paths
}

func generatedIDEPaths() []string {
	return []string{
		"AGENTS.md",
		"CLAUDE.md",
		".github/copilot-instructions.md",
		".windsurfrules",
		".claude/agents/architect.md",
		".codex/agents/architect.toml",
		".cursor/rules/core.mdc",
		".cursor/skills/classify-change/SKILL.md",
	}
}

func privateRepoPaths() []string {
	return []string{
		"docs/spec/1780346463-add-aidlc-cli.md",
		"docs/adr/1780346463-aidlc-cli-distribution-and-sync.md",
		"docs/blueprints/aidlc.md",
		"docs/blueprints/template-payload.md",
		"docs/ARCHITECTURE.md",
		"docs/architecture/software.md",
		".ai/scripts/ai_init.sh",
		".ai/scripts/ai_update.sh",
		"aidlc/cmd/aidlc/main.go",
		"aidlc/internal/commands/init.go",
		"aidlc/internal/commands/update.go",
		"aidlc/internal/source/github.go",
		"aidlc/internal/sync/planner.go",
		".github/workflows/aidlc-ci.yml",
		".github/workflows/aidlc-release.yml",
		"aidlc/.goreleaser.yaml",
		"aidlc/scripts/install.sh",
		"aidlc/scripts/install.ps1",
		"aidlc/scripts/verify-release.sh",
	}
}

func writePublicPayloadFile(t testing.TB, root, path, label string) {
	t.Helper()

	switch path {
	case ".ai/README.md":
		testutil.WriteFile(t, root, path, readme("Use governed workflows "+label+"."))
	case ".ai/models.defaults.toml":
		testutil.WriteFile(t, root, path, `[codex.architect]
model = "gpt-5"
reasoning = "high"

[codex.implementer]
model = "gpt-5"
reasoning = "medium"

[codex.reviewer]
model = "gpt-5"
reasoning = "high"

[claude.architect]
model = "claude-opus-4-6"

[cursor.architect]
model = "composer-2.5"
`)
	case ".ai/personas/architect.md":
		testutil.WriteFile(t, root, path, personaDoc("architect", "Plans governed changes.", "Plan governed changes."))
	case ".ai/personas/implementer.md":
		testutil.WriteFile(t, root, path, personaDoc("implementer", "Edits governed changes.", "Edit governed changes."))
	case ".ai/personas/reviewer.md":
		testutil.WriteFile(t, root, path, personaDoc("reviewer", "Reviews governed diffs.", "Review governed diffs."))
	case ".ai/skills/classify-change.md":
		testutil.WriteFile(t, root, path, skillDoc("classify-change", "Classifies governed changes.", "Classify before implementation."))
	case ".ai/skills/init-architecture.md":
		testutil.WriteFile(t, root, path, skillDoc("init-architecture", "Initializes architecture docs.", "Initialize architecture docs."))
	case ".ai/skills/orchestrate-spec.md":
		testutil.WriteFile(t, root, path, skillDoc("orchestrate-spec", "Runs approved spec work packages.", "Run spec work packages."))
	case ".ai/references/architecture-template.md":
		testutil.WriteFile(t, root, path, "# Architecture Template\n\nstarter "+label+"\n")
	case ".ai/references/architectures/data-engineering/architecture.md",
		".ai/references/architectures/data-science/architecture.md",
		".ai/references/architectures/minimal-tooling/architecture.md",
		".ai/references/architectures/polyglot-monorepo/architecture.md",
		".ai/references/architectures/tiered-service/architecture.md":
		testutil.WriteFile(t, root, path, "# "+filepath.Base(filepath.Dir(path))+" Architecture\n\nstarter "+label+"\n")
	case ".ai/references/architectures/data-engineering/blueprint-template.md",
		".ai/references/architectures/data-science/blueprint-template.md",
		".ai/references/architectures/polyglot-monorepo/blueprint-template.md",
		".ai/references/architectures/tiered-service/blueprint-template.md":
		testutil.WriteFile(t, root, path, "# "+filepath.Base(filepath.Dir(path))+" Blueprint Template\n\nstarter "+label+"\n")
	case ".ai/references/ci/github-finalize-spec.yml", ".ai/references/ci/gitlab-finalize-spec.yml":
		testutil.WriteFile(t, root, path, "name: finalize spec "+label+"\n")
	case ".ai/scripts/finalize_spec.sh":
		testutil.WriteFile(t, root, path, "#!/bin/sh\n# finalize spec "+label+"\n")
	case ".ai/templates/approval-brief.md", ".ai/templates/spec.md":
		testutil.WriteFile(t, root, path, "template "+label+"\n")
	case "docs/spec/README.md", "docs/adr/README.md", "docs/blueprints/README.md":
		testutil.WriteFile(t, root, path, "starter "+label+"\n")
	case "LICENSE":
		testutil.WriteFile(t, root, path, "license "+label+"\n")
	default:
		t.Fatalf("unhandled public payload path %s", path)
	}
}

func readme(body string) string {
	return "# Portable guidance\n\n<!-- INIT:BEGIN -->\n\n## Main agent delegation\n\n" + body + "\n"
}

func personaDoc(name, description, body string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\n---\n\n# Persona: " + name + "\n\n" + body + "\n"
}

func skillDoc(name, description, body string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\n---\n\n# " + name + "\n\n" + body + "\n"
}

func writePrivateRepoFiles(t testing.TB, root string) {
	t.Helper()

	for _, path := range privateRepoPaths() {
		testutil.WriteFile(t, root, path, "private "+path+"\n")
	}
}

func assertPrivatePathsMissing(t testing.TB, root string) {
	t.Helper()

	for _, path := range privateRepoPaths() {
		assertMissing(t, root, path)
	}
}

func assertPublicManifestPath(t testing.TB, path string) {
	t.Helper()

	for _, public := range publicPayloadPaths() {
		if path == public {
			return
		}
	}
	t.Fatalf("manifest contains non-public path %s", path)
}

func assertManifestIncludes(t testing.TB, manifest *contract.TargetManifest, path string) {
	t.Helper()

	for _, file := range manifest.Files {
		if file.Path == path {
			return
		}
	}
	t.Fatalf("manifest missing %s: %#v", path, manifest.Files)
}

func manifestHasFile(manifest *contract.TargetManifest, path string) bool {
	for _, file := range manifest.Files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func assertIDESelection(t testing.TB, got, want []contract.IDE) {
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

func assertExists(t testing.TB, root, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
		t.Fatalf("expected %s to exist: %v", name, err)
	}
}

func writeLegacyManifestJSON(t testing.TB, root string, manifest contract.TargetManifest) {
	t.Helper()

	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy manifest: %v", err)
	}
	testutil.WriteFile(t, root, contract.LegacyTargetManifestPath, string(append(content, '\n')))
}

func removeTargetFile(t testing.TB, root, name string) {
	t.Helper()

	if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
		t.Fatalf("remove %s: %v", name, err)
	}
}

func assertMissing(t testing.TB, root, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err == nil {
		t.Fatalf("expected %s to be absent", name)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", name, err)
	}
}
