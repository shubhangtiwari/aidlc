package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aidlc/ai-dlc-template/aidlc/internal/contract"
	"github.com/aidlc/ai-dlc-template/aidlc/internal/testutil"
)

func TestGenerateMinimalAllIDEs(t *testing.T) {
	root := newTemplateRepo(t)

	result, err := Generate(Options{TargetDir: root, IDE: contract.IDEAll})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	written := strings.Join(sortedWritten(result), "\n")
	for _, want := range []string{
		"AGENTS.md",
		"CLAUDE.md",
		".github/copilot-instructions.md",
		".windsurfrules",
		".claude/agents/architect.md",
		".codex/agents/architect.toml",
		".cursor/rules/core.mdc",
		".cursor/skills/classify-change/SKILL.md",
	} {
		if !strings.Contains(written, want) {
			t.Fatalf("written files missing %s:\n%s", want, written)
		}
	}

	agents := testutil.ReadFile(t, root, "AGENTS.md")
	if !strings.Contains(agents, "<!-- generated from .ai/ -- do not edit by hand. Run `make init cursor` to regenerate. -->") {
		t.Fatalf("AGENTS.md missing generated marker:\n%s", agents)
	}
	if !strings.Contains(agents, "- Manifest: not detected (optional — re-run `make init <ide>` after adding one)") {
		t.Fatalf("AGENTS.md missing no-manifest facts:\n%s", agents)
	}
	if !strings.Contains(agents, "- `architect` — `.cursor/agents/architect.md` — Plans changes.") {
		t.Fatalf("AGENTS.md missing cursor agent summary:\n%s", agents)
	}

	claudeAgent := testutil.ReadFile(t, root, ".claude/agents/architect.md")
	if !strings.Contains(claudeAgent, "model: claude-opus-4-6") {
		t.Fatalf("claude agent missing model default:\n%s", claudeAgent)
	}
	codexAgent := testutil.ReadFile(t, root, ".codex/agents/architect.toml")
	if !strings.Contains(codexAgent, "sandbox_mode = \"read-only\"") {
		t.Fatalf("codex agent missing read-only sandbox:\n%s", codexAgent)
	}
	cursorRule := testutil.ReadFile(t, root, ".cursor/rules/governance-spec-gate.mdc")
	if !strings.Contains(cursorRule, "globs: {**/*,tests/**,docs/spec/**,docs/blueprints/**,docs/adr/**,docs/ARCHITECTURE.md,docs/architecture/**}") {
		t.Fatalf("cursor governance rule missing default globs:\n%s", cursorRule)
	}
}

func TestGenerateManifestEnrichedCodex(t *testing.T) {
	root := newTemplateRepo(t)
	testutil.WriteFile(t, root, "package.json", `{
  "name": "manifest-app",
  "packageManager": "npm@11.0.0",
  "engines": {"node": ">=22"}
}`)

	if _, err := Generate(Options{TargetDir: root, IDE: contract.IDECodex}); err != nil {
		t.Fatalf("generate: %v", err)
	}

	agents := testutil.ReadFile(t, root, "AGENTS.md")
	for _, want := range []string{
		"<!-- generated from .ai/ + package.json -- do not edit by hand. Run `make init codex` to regenerate. -->",
		"# AI Governance — manifest-app",
		"- Language: JavaScript / Node",
		"- Manifest: `package.json`",
		"- Package/import namespace: `manifest-app`",
		"- Runtime/version constraint: `>=22`",
		"- Build tool: `npm@11.0.0`",
	} {
		if !strings.Contains(agents, want) {
			t.Fatalf("AGENTS.md missing %q:\n%s", want, agents)
		}
	}
}

func TestGenerateExplicitSubsetOnlyWritesRequestedIDESurfaces(t *testing.T) {
	root := newTemplateRepo(t)

	result, err := Generate(Options{
		TargetDir: root,
		IDEs:      []contract.IDE{contract.IDECopilot, contract.IDEClaude, contract.IDECopilot},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	written := sortedWritten(result)
	for _, want := range []string{
		".claude/agents/architect.md",
		".claude/skills/classify-change/SKILL.md",
		".github/copilot-instructions.md",
		"CLAUDE.md",
	} {
		if !containsString(written, want) {
			t.Fatalf("written files missing %s:\n%s", want, strings.Join(written, "\n"))
		}
	}
	for _, notWant := range []string{
		"AGENTS.md",
		".codex/agents/architect.toml",
		".cursor/rules/core.mdc",
		".windsurfrules",
	} {
		if containsString(written, notWant) {
			t.Fatalf("written files included unrequested surface %s:\n%s", notWant, strings.Join(written, "\n"))
		}
		if fileExists(root, notWant) {
			t.Fatalf("generated unrequested surface %s", notWant)
		}
	}
}

func TestGenerateExplicitSelectionRejectsAggregateAndUnsupportedIDEs(t *testing.T) {
	for _, tt := range []struct {
		name string
		ides []contract.IDE
		want string
	}{
		{name: "aggregate", ides: []contract.IDE{contract.IDEAll}, want: `aggregate IDE "all"`},
		{name: "unsupported", ides: []contract.IDE{contract.IDE("zed")}, want: `unsupported IDE "zed"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := newTemplateRepo(t)

			_, err := Generate(Options{TargetDir: root, IDEs: tt.ides})
			if err == nil {
				t.Fatal("generate succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want to contain %q", err, tt.want)
			}
		})
	}
}

func newTemplateRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testutil.WriteFile(t, root, ".ai/README.md", `# Portable guidance

<!-- INIT:BEGIN -->

<!-- generated body starts after this comment -->

## Main agent delegation

Portable governance rules.
`)
	testutil.WriteFile(t, root, ".ai/personas/architect.md", `---
name: architect
description: Plans changes.
---

# Persona: Architect

Plans only.
`)
	testutil.WriteFile(t, root, ".ai/personas/implementer.md", `---
name: implementer
description: Edits code.
---

# Persona: Implementer

Edits source.
`)
	testutil.WriteFile(t, root, ".ai/personas/reviewer.md", `---
name: reviewer
description: Reviews diffs.
---

# Persona: Reviewer

Reviews changes.
`)
	testutil.WriteFile(t, root, ".ai/skills/classify-change.md", `---
name: classify-change
description: Classifies governed changes.
---

# classify-change

Triage instructions.
`)
	testutil.WriteFile(t, root, ".ai/models.defaults.toml", `[codex.architect]
model = "gpt-5.5"
reasoning = "high"

[codex.implementer]
model = "gpt-5.5"
reasoning = "medium"

[codex.reviewer]
model = "gpt-5.5"
reasoning = "high"

[claude.architect]
model = "claude-opus-4-6"

[cursor.architect]
model = "composer-2.5"
`)
	if err := os.MkdirAll(filepath.Join(root, ".ai", "skills"), 0o755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	return root
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func fileExists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}
