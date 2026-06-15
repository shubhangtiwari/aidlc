package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
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
	if strings.Contains(agents, "ai_init.sh") || strings.Contains(agents, "ai_update.sh") {
		t.Fatalf("AGENTS.md references retired shell compatibility:\n%s", agents)
	}
	if !strings.Contains(agents, "- `architect` — `.cursor/agents/architect.md` — Plans changes.") {
		t.Fatalf("AGENTS.md missing cursor agent summary:\n%s", agents)
	}

	claudeAgent := testutil.ReadFile(t, root, ".claude/agents/architect.md")
	if !strings.Contains(claudeAgent, "model: claude-fable-5") {
		t.Fatalf("claude agent missing model default:\n%s", claudeAgent)
	}
	if !strings.Contains(claudeAgent, "effort: xhigh") {
		t.Fatalf("claude agent missing effort default:\n%s", claudeAgent)
	}
	codexAgent := testutil.ReadFile(t, root, ".codex/agents/architect.toml")
	if !strings.Contains(codexAgent, "sandbox_mode = \"read-only\"") {
		t.Fatalf("codex agent missing read-only sandbox:\n%s", codexAgent)
	}
	cursorRule := testutil.ReadFile(t, root, ".cursor/rules/governance-spec-gate.mdc")
	if !strings.Contains(cursorRule, "globs: {**/*,tests/**,docs/spec/**,docs/blueprints/**,docs/adr/**,docs/ARCHITECTURE.md,docs/architecture/**}") {
		t.Fatalf("cursor governance rule missing default globs:\n%s", cursorRule)
	}
	for _, want := range []string{
		"Portable rules: `.ai/README.md` (especially Scope resolution). Parallel waves: skill `orchestrate-spec`.",
		"- **Do** launch `architect` to draft scope-local spec file(s), then stop for approval.",
		"Scope-local specs live at `docs/spec/<epoch>-<slug>.md` relative to each resolved AIDLC scope root.",
		"- Check `docs/spec/.in-flight.yaml` in the owning scope for specs tied to the current branch.",
	} {
		if !strings.Contains(cursorRule, want) {
			t.Fatalf("cursor governance rule missing %q:\n%s", want, cursorRule)
		}
	}
	if strings.Contains(cursorRule, "- **Do** launch `architect` to draft `docs/spec/<epoch>-<slug>.md`, then stop for approval.") {
		t.Fatalf("cursor governance rule kept invocation-root-only draft guidance:\n%s", cursorRule)
	}
}

func TestGeneratePersonaModelDefaultsExactMappings(t *testing.T) {
	root := newTemplateRepo(t)

	if _, err := Generate(Options{TargetDir: root, IDE: contract.IDEAll}); err != nil {
		t.Fatalf("generate: %v", err)
	}

	mappings := []struct {
		persona       string
		claudeModel    string
		claudeEffort   string
		codexModel     string
		codexReasoning string
		cursorModel    string
	}{
		{
			persona:       "architect",
			claudeModel:    "claude-fable-5",
			claudeEffort:   "xhigh",
			codexModel:     "gpt-5.6-sol",
			codexReasoning: "xhigh",
			cursorModel:    "composer-2.5",
		},
		{
			persona:       "implementer",
			claudeModel:    "claude-sonnet-5",
			claudeEffort:   "high",
			codexModel:     "gpt-5.6-luna",
			codexReasoning: "xhigh",
			cursorModel:    "composer-2.5",
		},
		{
			persona:       "reviewer",
			claudeModel:    "claude-opus-4-8",
			claudeEffort:   "xhigh",
			codexModel:     "gpt-5.6-sol",
			codexReasoning: "xhigh",
			cursorModel:    "composer-2.5",
		},
	}

	for _, mapping := range mappings {
		t.Run(mapping.persona, func(t *testing.T) {
			claudeAgent := testutil.ReadFile(t, root, ".claude/agents/"+mapping.persona+".md")
			claudeFields := agentFrontmatterFields(t, claudeAgent)
			if got := claudeFields["model"]; got != mapping.claudeModel {
				t.Fatalf("claude model = %q, want %q", got, mapping.claudeModel)
			}
			if got := claudeFields["effort"]; got != mapping.claudeEffort {
				t.Fatalf("claude effort = %q, want %q", got, mapping.claudeEffort)
			}
			if _, ok := claudeFields["reasoning"]; ok {
				t.Fatalf("claude agent emitted unsupported reasoning field:\n%s", claudeAgent)
			}

			codexAgent := testutil.ReadFile(t, root, ".codex/agents/"+mapping.persona+".toml")
			if got, ok := agentTOMLSetting(codexAgent, "model"); !ok || got != mapping.codexModel {
				t.Fatalf("codex model = %q, present %t, want %q", got, ok, mapping.codexModel)
			}
			if got, ok := agentTOMLSetting(codexAgent, "model_reasoning_effort"); !ok || got != mapping.codexReasoning {
				t.Fatalf("codex reasoning = %q, present %t, want %q", got, ok, mapping.codexReasoning)
			}
			if _, ok := agentTOMLSetting(codexAgent, "effort"); ok {
				t.Fatalf("codex agent emitted unsupported effort field:\n%s", codexAgent)
			}

			cursorAgent := testutil.ReadFile(t, root, ".cursor/agents/"+mapping.persona+".md")
			cursorFields := agentFrontmatterFields(t, cursorAgent)
			if got := cursorFields["model"]; got != mapping.cursorModel {
				t.Fatalf("cursor model = %q, want %q", got, mapping.cursorModel)
			}
			if _, ok := cursorFields["effort"]; ok {
				t.Fatalf("cursor agent emitted unsupported effort field:\n%s", cursorAgent)
			}
			if _, ok := cursorFields["reasoning"]; ok {
				t.Fatalf("cursor agent emitted unsupported reasoning field:\n%s", cursorAgent)
			}
		})
	}
}

func TestGeneratePersonaModelDefaultsOmitEmptyAndAbsentFields(t *testing.T) {
	root := newTemplateRepo(t)
	testutil.WriteFile(t, root, ".ai/models.defaults.toml", `[claude.architect]
model = "claude-fable-5"
effort = ""

[claude.reviewer]
model = ""
effort = "xhigh"

[codex.architect]
model = ""
reasoning = ""

[cursor.architect]
model = ""
`)

	if _, err := Generate(Options{TargetDir: root, IDE: contract.IDEAll}); err != nil {
		t.Fatalf("generate sparse defaults: %v", err)
	}

	claudeArchitect := agentFrontmatterFields(t, testutil.ReadFile(t, root, ".claude/agents/architect.md"))
	if _, ok := claudeArchitect["effort"]; ok {
		t.Fatalf("claude architect emitted empty effort: %v", claudeArchitect)
	}
	// The implementer sections are intentionally absent for every IDE.
	claudeImplementer := agentFrontmatterFields(t, testutil.ReadFile(t, root, ".claude/agents/implementer.md"))
	if _, ok := claudeImplementer["model"]; ok {
		t.Fatalf("claude implementer emitted absent model: %v", claudeImplementer)
	}
	if _, ok := claudeImplementer["effort"]; ok {
		t.Fatalf("claude implementer emitted absent effort: %v", claudeImplementer)
	}
	claudeReviewer := agentFrontmatterFields(t, testutil.ReadFile(t, root, ".claude/agents/reviewer.md"))
	if _, ok := claudeReviewer["model"]; ok {
		t.Fatalf("claude reviewer emitted empty model: %v", claudeReviewer)
	}
	if got := claudeReviewer["effort"]; got != "xhigh" {
		t.Fatalf("claude reviewer effort = %q, want %q", got, "xhigh")
	}

	codexArchitect := testutil.ReadFile(t, root, ".codex/agents/architect.toml")
	if _, ok := agentTOMLSetting(codexArchitect, "model"); ok {
		t.Fatalf("codex architect emitted empty model:\n%s", codexArchitect)
	}
	if _, ok := agentTOMLSetting(codexArchitect, "model_reasoning_effort"); ok {
		t.Fatalf("codex architect emitted empty reasoning:\n%s", codexArchitect)
	}
	codexImplementer := testutil.ReadFile(t, root, ".codex/agents/implementer.toml")
	if _, ok := agentTOMLSetting(codexImplementer, "model"); ok {
		t.Fatalf("codex implementer emitted absent model:\n%s", codexImplementer)
	}
	if _, ok := agentTOMLSetting(codexImplementer, "model_reasoning_effort"); ok {
		t.Fatalf("codex implementer emitted absent reasoning:\n%s", codexImplementer)
	}

	cursorArchitect := agentFrontmatterFields(t, testutil.ReadFile(t, root, ".cursor/agents/architect.md"))
	if _, ok := cursorArchitect["model"]; ok {
		t.Fatalf("cursor architect emitted empty model: %v", cursorArchitect)
	}
	cursorImplementer := agentFrontmatterFields(t, testutil.ReadFile(t, root, ".cursor/agents/implementer.md"))
	if _, ok := cursorImplementer["model"]; ok {
		t.Fatalf("cursor implementer emitted absent model: %v", cursorImplementer)
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
model = "gpt-5.6-sol"
reasoning = "xhigh"

[codex.implementer]
model = "gpt-5.6-luna"
reasoning = "xhigh"

[codex.reviewer]
model = "gpt-5.6-sol"
reasoning = "xhigh"

[claude.architect]
model = "claude-fable-5"
effort = "xhigh"

[claude.implementer]
model = "claude-sonnet-5"
effort = "high"

[claude.reviewer]
model = "claude-opus-4-8"
effort = "xhigh"

[cursor.architect]
model = "composer-2.5"

[cursor.implementer]
model = "composer-2.5"

[cursor.reviewer]
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

func agentFrontmatterFields(t *testing.T, content string) map[string]string {
	t.Helper()
	frontmatter, _, err := splitFrontmatter(content)
	if err != nil {
		t.Fatalf("split agent frontmatter: %v\n%s", err, content)
	}
	return parseSimpleYAML(frontmatter)
}

func agentTOMLSetting(content, key string) (string, bool) {
	prefix := key + " = "
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.Trim(strings.TrimPrefix(line, prefix), `"`), true
		}
	}
	return "", false
}
