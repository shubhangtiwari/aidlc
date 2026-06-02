package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
)

func TestInitCopiesOnlyPublicManifestPathsAndGeneratesIDE(t *testing.T) {
	source := createTemplateSource(t)
	target := t.TempDir()

	result, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source: contract.SourceOptions{
			Kind: "local",
			Path: source,
			Ref:  "test",
		},
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("init reported conflicts")
	}

	assertExists(t, target, ".ai/README.md")
	assertExists(t, target, ".ai/personas/architect.md")
	assertExists(t, target, ".ai/skills/classify-change.md")
	assertExists(t, target, "docs/spec/README.md")
	assertExists(t, target, "AGENTS.md")
	assertExists(t, target, ".codex/agents/architect.toml")
	assertExists(t, target, contract.TargetManifestPath)

	assertMissing(t, target, "docs/spec/1780346463-add-aidlc-cli.md")
	assertMissing(t, target, "docs/adr/1780346463-aidlc-cli-distribution-and-sync.md")
	assertMissing(t, target, "docs/blueprints/aidlc.md")
	assertMissing(t, target, "docs/ARCHITECTURE.md")
	assertMissing(t, target, "docs/architecture/software.md")
	assertMissing(t, target, "aidlc/internal/commands/init.go")
	assertMissing(t, target, ".github/workflows/aidlc-release.yml")
	assertMissing(t, target, "aidlc/.goreleaser.yaml")
	assertMissing(t, target, "aidlc/scripts/install.sh")
}

func TestInitDryRunDoesNotWritePayloadOrGeneratedFiles(t *testing.T) {
	source := createTemplateSource(t)
	target := t.TempDir()

	result, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: source},
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("init dry run: %v", err)
	}
	if len(result.Written) != 0 || len(result.Generated) != 0 {
		t.Fatalf("dry run wrote files: %#v", result)
	}
	assertMissing(t, target, ".ai/README.md")
	assertMissing(t, target, "AGENTS.md")
	assertMissing(t, target, contract.TargetManifestPath)
}

func TestInitReportsConflictWithoutOverwriting(t *testing.T) {
	source := createTemplateSource(t)
	target := t.TempDir()
	testutil.WriteFile(t, target, ".ai/README.md", "local edits")

	result, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: source},
	})
	if err != nil {
		t.Fatalf("init conflict should be reported in plan, not as error: %v", err)
	}
	if !hasConflict(result.Plan) {
		t.Fatal("expected conflict")
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "local edits" {
		t.Fatalf("divergent file overwritten: %q", got)
	}
	assertMissing(t, target, "AGENTS.md")
	assertMissing(t, target, contract.TargetManifestPath)
}

func TestInitCLIUsageAndVersionOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := RunInitCLI(context.Background(), nil, &stdout, &stderr); code != contract.ExitUsage {
		t.Fatalf("init code = %d, want usage", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := RunInitCLI(context.Background(), []string{"--help"}, &stdout, &stderr); code != contract.ExitOK {
		t.Fatalf("init help code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage: aidlc init") {
		t.Fatalf("usage output missing: %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunVersionCLI(nil, &stdout, &stderr); code != contract.ExitOK {
		t.Fatalf("version code = %d", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "aidlc dev" {
		t.Fatalf("version output = %q", got)
	}
}

func TestVersionOutputUsesInjectedVersion(t *testing.T) {
	previous := Version
	t.Cleanup(func() {
		Version = previous
	})
	Version = "v9.8.7"

	var stdout, stderr bytes.Buffer
	if code := RunVersionCLI(nil, &stdout, &stderr); code != contract.ExitOK {
		t.Fatalf("version code = %d", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "aidlc v9.8.7" {
		t.Fatalf("version output = %q", got)
	}
}

func TestInitCLIAcceptsFlagsAfterIDE(t *testing.T) {
	source := createTemplateSource(t)
	target := t.TempDir()
	chdirForTest(t, target)

	var stdout, stderr bytes.Buffer
	code := RunInitCLI(context.Background(), []string{"codex", "--source", "local", "--path", source, "--ref", "test"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("init cli code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}

	assertExists(t, target, ".ai/README.md")
	assertExists(t, target, "AGENTS.md")
	assertExists(t, target, ".codex/agents/architect.toml")
	assertExists(t, target, contract.TargetManifestPath)
}

func TestInitMergesIDESelectionFromRootLock(t *testing.T) {
	source := createTemplateSource(t)
	target := t.TempDir()
	writeTargetManifestJSON(t, target, contract.TargetManifestPath, contract.TargetManifest{
		Generated: contract.GenerationRecord{IDE: contract.IDECodex},
		Workspace: contract.WorkspaceRecord{
			IDEs: []contract.IDE{contract.IDECodex},
		},
	})

	result, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECursor,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: source, Ref: "test"},
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("init reported conflicts")
	}

	manifest := readTargetManifest(t, target)
	assertIDESelection(t, manifest.Workspace.IDEs, []contract.IDE{contract.IDECodex, contract.IDECursor})
	if manifest.Generated.IDE != contract.IDECursor {
		t.Fatalf("generated IDE = %q", manifest.Generated.IDE)
	}
}

func TestInitSeedsIDESelectionFromLegacyManifest(t *testing.T) {
	source := createTemplateSource(t)
	target := t.TempDir()
	writeTargetManifestJSON(t, target, contract.LegacyTargetManifestPath, contract.TargetManifest{
		Generated: contract.GenerationRecord{IDE: contract.IDECodex},
		Workspace: contract.WorkspaceRecord{
			IDEs: []contract.IDE{contract.IDEClaude},
		},
	})

	result, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDECodex,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: source, Ref: "test"},
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("init reported conflicts")
	}

	assertExists(t, target, contract.TargetManifestPath)
	manifest := readTargetManifest(t, target)
	assertIDESelection(t, manifest.Workspace.IDEs, []contract.IDE{contract.IDEClaude, contract.IDECodex})
}

func TestInitAllStoresConcreteIDESelection(t *testing.T) {
	source := createTemplateSource(t)
	target := t.TempDir()

	result, err := RunInit(context.Background(), contract.InitOptions{
		IDE:       contract.IDEAll,
		TargetDir: target,
		Source:    contract.SourceOptions{Kind: "local", Path: source, Ref: "test"},
	})
	if err != nil {
		t.Fatalf("init all: %v", err)
	}
	if hasConflict(result.Plan) {
		t.Fatal("init reported conflicts")
	}

	manifest := readTargetManifest(t, target)
	assertIDESelection(t, manifest.Workspace.IDEs, contract.ConcreteIDEs())
}

func createTemplateSource(t testing.TB) string {
	t.Helper()

	root := t.TempDir()
	testutil.WriteFile(t, root, ".ai/template-manifest.yaml", `schema_version: 1
payload:
  include:
    - .ai/README.md
    - .ai/models.defaults.toml
    - .ai/personas/architect.md
    - .ai/skills/classify-change.md
    - docs/spec/README.md
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
	testutil.WriteFile(t, root, ".ai/README.md", "<!-- INIT:BEGIN -->\n\n## Main agent delegation\n\nUse governed workflows.\n")
	testutil.WriteFile(t, root, ".ai/models.defaults.toml", "[codex.architect]\nmodel = \"gpt-5\"\n")
	testutil.WriteFile(t, root, ".ai/personas/architect.md", "---\nname: architect\ndescription: Plans changes.\n---\n\nPlan governed changes.\n")
	testutil.WriteFile(t, root, ".ai/skills/classify-change.md", "---\nname: classify-change\ndescription: Classifies changes.\n---\n\nClassify before editing.\n")
	testutil.WriteFile(t, root, "docs/spec/README.md", "starter specs\n")
	testutil.WriteFile(t, root, "docs/adr/README.md", "starter ADRs\n")
	testutil.WriteFile(t, root, "docs/blueprints/README.md", "starter blueprints\n")
	testutil.WriteFile(t, root, "LICENSE", "license\n")

	testutil.WriteFile(t, root, "docs/spec/1780346463-add-aidlc-cli.md", "private spec\n")
	testutil.WriteFile(t, root, "docs/adr/1780346463-aidlc-cli-distribution-and-sync.md", "private ADR\n")
	testutil.WriteFile(t, root, "docs/blueprints/aidlc.md", "private blueprint\n")
	testutil.WriteFile(t, root, "docs/blueprints/template-payload.md", "private blueprint\n")
	testutil.WriteFile(t, root, "docs/ARCHITECTURE.md", "private architecture\n")
	testutil.WriteFile(t, root, "docs/architecture/software.md", "private layer docs\n")
	testutil.WriteFile(t, root, "aidlc/internal/commands/init.go", "private source\n")
	testutil.WriteFile(t, root, ".github/workflows/aidlc-release.yml", "private ci\n")
	testutil.WriteFile(t, root, "aidlc/.goreleaser.yaml", "private release\n")
	testutil.WriteFile(t, root, "aidlc/scripts/install.sh", "private installer\n")
	return root
}

func chdirForTest(t testing.TB, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore wd %s: %v", previous, err)
		}
	})
}

func assertExists(t testing.TB, root, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
		t.Fatalf("expected %s to exist: %v", name, err)
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

func readTargetManifest(t testing.TB, target string) *contract.TargetManifest {
	t.Helper()
	manifest, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if manifest == nil {
		t.Fatal("manifest missing")
	}
	return manifest
}

func writeTargetManifestJSON(t testing.TB, target, relativePath string, manifest contract.TargetManifest) {
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
