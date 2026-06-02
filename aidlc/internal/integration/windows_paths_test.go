package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/payload"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/source"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
)

func TestWindowsStyleManifestPathsNormalizeDuringInitAndGeneration(t *testing.T) {
	sourceRoot := createIntegrationSource(t, "windows")
	testutil.WriteFile(t, sourceRoot, ".ai/template-manifest.yaml", `schema_version: 1
payload:
  include:
    - .ai\README.md
    - .ai\models.defaults.toml
    - .ai\personas\architect.md
    - .ai\personas\implementer.md
    - .ai\personas\reviewer.md
    - .ai\skills\classify-change.md
    - docs\spec\README.md
    - docs\adr\README.md
    - docs\blueprints\README.md
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
	target := t.TempDir()

	runCLIInDir(t, target, contract.ExitOK, "init", "codex", "--source", "local", "--path", sourceRoot, "--ref", "windows")

	assertExists(t, target, ".ai/README.md")
	assertExists(t, target, ".codex/agents/architect.toml")
	assertExists(t, target, ".codex/skills/classify-change/SKILL.md")
	assertExists(t, target, "AGENTS.md")
	assertExists(t, target, contract.TargetManifestPath)
	assertMissing(t, target, contract.LegacyTargetManifestPath)
	assertPrivatePathsMissing(t, target)

	manifest, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if got := testutil.ReadFile(t, target, contract.TargetManifestPath); strings.Contains(got, `\`) {
		t.Fatalf("root lock contains non-normalized path separators:\n%s", got)
	}
	for _, file := range manifest.Files {
		if strings.Contains(file.Path, `\`) {
			t.Fatalf("manifest path was not normalized: %s", file.Path)
		}
		assertPublicManifestPath(t, file.Path)
	}
}

func TestWindowsDrivePathIsRejectedAsTemplatePayloadPath(t *testing.T) {
	if _, err := payload.NormalizeRelativePath(`C:\repo\.ai\README.md`); err == nil {
		t.Fatal("expected Windows drive path to be rejected")
	}

	_, err := source.Local{Root: createSourceWithManifest(t, `schema_version: 1
payload:
  include:
    - C:\repo\.ai\README.md
policy:
  allow_broad_directories: false
  public_docs_must_be_explicit: true
  reject_absolute_paths: true
  reject_parent_traversal: true
`)}.Snapshot(context.Background())
	if err == nil {
		t.Fatal("expected local snapshot to reject Windows drive path")
	}
}

func createSourceWithManifest(t testing.TB, manifest string) string {
	t.Helper()

	root := t.TempDir()
	testutil.WriteFile(t, root, ".ai/template-manifest.yaml", manifest)
	return root
}
