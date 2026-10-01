package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/skills"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
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
	if !strings.Contains(agents, "<!-- generated from .ai/ -- do not edit by hand. Run `make init all` to regenerate. -->") {
		t.Fatalf("AGENTS.md missing generated marker:\n%s", agents)
	}
	if !strings.Contains(agents, "- Manifest: not detected (optional — re-run `make init <ide>` after adding one)") {
		t.Fatalf("AGENTS.md missing no-manifest facts:\n%s", agents)
	}
	if strings.Contains(agents, "ai_init.sh") || strings.Contains(agents, "ai_update.sh") {
		t.Fatalf("AGENTS.md references retired shell compatibility:\n%s", agents)
	}
	if !strings.Contains(agents, "- `architect` — Plans changes.") {
		t.Fatalf("AGENTS.md missing compact agent summary:\n%s", agents)
	}
	if strings.Contains(agents, "# Persona: Architect") || strings.Contains(agents, "Triage instructions.") {
		t.Fatalf("AGENTS.md embedded full persona or skill bodies:\n%s", agents)
	}

	claudeAgent := testutil.ReadFile(t, root, ".claude/agents/architect.md")
	if !strings.Contains(claudeAgent, "model: claude-fable-5") {
		t.Fatalf("claude agent missing model default:\n%s", claudeAgent)
	}
	if !strings.Contains(claudeAgent, "effort: xhigh") {
		t.Fatalf("claude agent missing effort default:\n%s", claudeAgent)
	}
	codexAgent := testutil.ReadFile(t, root, ".codex/agents/architect.toml")
	if !strings.Contains(codexAgent, "sandbox_mode = \"workspace-write\"") {
		t.Fatalf("codex architect missing workspace-write sandbox:\n%s", codexAgent)
	}
	reviewerAgent := testutil.ReadFile(t, root, ".codex/agents/reviewer.toml")
	if !strings.Contains(reviewerAgent, "sandbox_mode = \"read-only\"") {
		t.Fatalf("codex reviewer missing read-only sandbox:\n%s", reviewerAgent)
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
		persona        string
		claudeModel    string
		claudeEffort   string
		codexModel     string
		codexReasoning string
		cursorModel    string
	}{
		{
			persona:        "architect",
			claudeModel:    "claude-fable-5",
			claudeEffort:   "xhigh",
			codexModel:     "gpt-5.6-sol",
			codexReasoning: "xhigh",
			cursorModel:    "composer-2.5",
		},
		{
			persona:        "implementer",
			claudeModel:    "claude-sonnet-5",
			claudeEffort:   "high",
			codexModel:     "gpt-5.6-luna",
			codexReasoning: "xhigh",
			cursorModel:    "composer-2.5",
		},
		{
			persona:        "reviewer",
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
		"<!-- generated from .ai/ + package.json -- do not edit by hand. Run `make init all` to regenerate. -->",
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

func TestGenerateSharedAgentsRootIsCompactAndDeterministic(t *testing.T) {
	root := newTemplateRepo(t)
	longBody := strings.Repeat("Follow the governed workflow with specific evidence. ", 160)
	testutil.WriteFile(t, root, ".ai/skills/large-local.md", `---
name: large-local
description: Large local skill.
---

# large-local

`+longBody+"\n")

	if _, err := Generate(Options{TargetDir: root, IDE: contract.IDECodex}); err != nil {
		t.Fatalf("generate codex: %v", err)
	}
	codexAgents := testutil.ReadFile(t, root, "AGENTS.md")
	if _, err := Generate(Options{TargetDir: root, IDE: contract.IDECursor}); err != nil {
		t.Fatalf("generate cursor: %v", err)
	}
	cursorAgents := testutil.ReadFile(t, root, "AGENTS.md")

	if codexAgents != cursorAgents {
		t.Fatalf("shared AGENTS.md differs between codex and cursor")
	}
	if words := wordCount(codexAgents); words > 1200 {
		t.Fatalf("AGENTS.md words = %d, want <= 1200", words)
	}
	if bytes := len([]byte(codexAgents)); bytes > 9000 {
		t.Fatalf("AGENTS.md bytes = %d, want <= 9000", bytes)
	}
	oldStyle := codexAgents + "\n## Persona Reference\n\n" +
		testutil.ReadFile(t, root, ".ai/personas/architect.md") +
		testutil.ReadFile(t, root, ".ai/personas/implementer.md") +
		testutil.ReadFile(t, root, ".ai/personas/reviewer.md") +
		"\n## Skill Reference\n\n" +
		testutil.ReadFile(t, root, ".ai/skills/classify-change.md") +
		testutil.ReadFile(t, root, ".ai/skills/large-local.md")
	if reductionPercent(len([]byte(oldStyle)), len([]byte(codexAgents))) < 40 {
		t.Fatalf("byte reduction below 40%%: old=%d new=%d", len([]byte(oldStyle)), len([]byte(codexAgents)))
	}
	if reductionPercent(wordCount(oldStyle), wordCount(codexAgents)) < 40 {
		t.Fatalf("word reduction below 40%%: old=%d new=%d", wordCount(oldStyle), wordCount(codexAgents))
	}
}

func TestGenerateInstalledSkillsPreservesFullNativeTreeAndReportsProjections(t *testing.T) {
	root := newTemplateRepo(t)
	testutil.WriteFile(t, root, ".ai/skills/installed/local-tool/SKILL.md", `---
name: local-tool
description: Local tool skill.
---

# local-tool

Use local details.
`)
	testutil.WriteFile(t, root, ".ai/skills/installed/local-tool/assets/prompt.txt", "prompt bytes\n")
	if err := os.MkdirAll(filepath.Join(root, ".ai", "skills", "installed", "local-tool", "empty"), 0o755); err != nil {
		t.Fatalf("mkdir empty installed dir: %v", err)
	}

	result, err := Generate(Options{TargetDir: root, IDE: contract.IDEAll})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if got := testutil.ReadFile(t, root, ".codex/skills/local-tool/SKILL.md"); !strings.Contains(got, "Use local details.") {
		t.Fatalf("codex installed skill not preserved:\n%s", got)
	}
	if got := testutil.ReadFile(t, root, ".cursor/skills/local-tool/assets/prompt.txt"); got != "prompt bytes\n" {
		t.Fatalf("cursor asset = %q", got)
	}
	if got := testutil.ReadFile(t, root, ".claude/skills/local-tool/assets/prompt.txt"); got != "prompt bytes\n" {
		t.Fatalf("claude asset = %q", got)
	}
	if info, err := os.Stat(filepath.Join(root, ".codex", "skills", "local-tool", "empty")); err != nil || !info.IsDir() {
		t.Fatalf("codex empty dir not preserved: info=%v err=%v", info, err)
	}
	assertMissing(t, root, ".github/skills/local-tool/SKILL.md")

	paths := map[string]contract.GeneratedProjection{}
	for _, projection := range result.Projections {
		paths[projection.Path] = projection
		if projection.Source != ".ai/skills/installed/local-tool" {
			t.Fatalf("projection source = %q", projection.Source)
		}
		if projection.Mode != "0644" {
			t.Fatalf("projection mode = %q", projection.Mode)
		}
		if err := projection.Validate(); err != nil {
			t.Fatalf("invalid projection %#v: %v", projection, err)
		}
	}
	for _, want := range []string{
		".codex/skills/local-tool/SKILL.md",
		".codex/skills/local-tool/assets/prompt.txt",
		".cursor/skills/local-tool/SKILL.md",
		".cursor/skills/local-tool/assets/prompt.txt",
		".claude/skills/local-tool/SKILL.md",
		".claude/skills/local-tool/assets/prompt.txt",
	} {
		if _, ok := paths[want]; !ok {
			t.Fatalf("missing projection %s in %#v", want, result.Projections)
		}
	}
}

func TestGenerateInstalledSkillProjectionRejectsUntrackedDivergence(t *testing.T) {
	root := newTemplateRepo(t)
	testutil.WriteFile(t, root, ".ai/skills/installed/local-tool/SKILL.md", `---
name: local-tool
description: Local tool skill.
---

# local-tool

Use local details.
`)
	testutil.WriteFile(t, root, ".codex/skills/local-tool/SKILL.md", "local native edits\n")

	_, err := Generate(Options{TargetDir: root, IDE: contract.IDECodex})
	if err == nil {
		t.Fatal("generate succeeded, want conflict")
	}
	if !strings.Contains(err.Error(), "already exists and differs from source") {
		t.Fatalf("error = %q", err)
	}
	assertMissing(t, root, ".codex/agents/architect.toml")
}

func TestGenerateInstalledSkillProjectionReconcilesDeletedAssetsBeforeSkillRemove(t *testing.T) {
	root := newTemplateRepo(t)
	testutil.WriteFile(t, root, ".ai/skills/installed/local-tool/SKILL.md", `---
name: local-tool
description: Local tool skill.
---

# local-tool

Use local details.
`)
	testutil.WriteFile(t, root, ".ai/skills/installed/local-tool/assets/remove.txt", "remove me\n")
	testutil.WriteFile(t, root, ".ai/skills/installed/local-tool/assets/diverge.txt", "owned before\n")

	first, err := Generate(Options{TargetDir: root, IDE: contract.IDECodex})
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	writeProjectionManifest(t, root, first.Projections)

	if err := os.Remove(filepath.Join(root, ".ai", "skills", "installed", "local-tool", "assets", "remove.txt")); err != nil {
		t.Fatalf("remove source asset: %v", err)
	}
	if err := os.Remove(filepath.Join(root, ".ai", "skills", "installed", "local-tool", "assets", "diverge.txt")); err != nil {
		t.Fatalf("remove source diverged asset: %v", err)
	}
	testutil.WriteFile(t, root, ".codex/skills/local-tool/assets/diverge.txt", "local native edits\n")

	second, err := Generate(Options{TargetDir: root, IDE: contract.IDECodex})
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	assertMissing(t, root, ".codex/skills/local-tool/assets/remove.txt")
	if got := testutil.ReadFile(t, root, ".codex/skills/local-tool/assets/diverge.txt"); got != "local native edits\n" {
		t.Fatalf("diverged native asset changed: %q", got)
	}
	if containsProjectionPath(second.Projections, ".codex/skills/local-tool/assets/remove.txt") {
		t.Fatalf("removed asset projection was retained: %#v", second.Projections)
	}
	if !containsProjectionPath(second.Projections, ".codex/skills/local-tool/assets/diverge.txt") {
		t.Fatalf("diverged stale projection was not retained: %#v", second.Projections)
	}
	writeProjectionManifest(t, root, second.Projections)

	remove, err := skills.Remove(skills.RemoveOptions{TargetDir: root, Name: "local-tool"})
	if err != nil {
		t.Fatalf("skill remove: %v", err)
	}
	if !remove.RemovedSource {
		t.Fatalf("installed source was not removed: %#v", remove)
	}
	assertMissing(t, root, ".ai/skills/installed/local-tool/SKILL.md")
	assertMissing(t, root, ".codex/skills/local-tool/SKILL.md")
	if got := testutil.ReadFile(t, root, ".codex/skills/local-tool/assets/diverge.txt"); got != "local native edits\n" {
		t.Fatalf("diverged projection should be preserved after remove: %q", got)
	}
	if len(remove.PreservedProjections) != 1 || remove.PreservedProjections[0].Path != ".codex/skills/local-tool/assets/diverge.txt" {
		t.Fatalf("preserved projections = %#v", remove.PreservedProjections)
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

func assertMissing(t testing.TB, root, rel string) {
	t.Helper()
	if fileExists(root, rel) {
		t.Fatalf("expected %s to be missing", rel)
	}
}

func wordCount(value string) int {
	return len(strings.Fields(value))
}

func reductionPercent(oldValue, newValue int) int {
	if oldValue == 0 {
		return 0
	}
	return ((oldValue - newValue) * 100) / oldValue
}

func writeProjectionManifest(t testing.TB, root string, projections []contract.GeneratedProjection) {
	t.Helper()
	if err := templatesync.WriteManifest(root, contract.TargetManifest{
		SchemaVersion: contract.TargetManifestVersion,
		Generated: contract.GenerationRecord{
			IDE:         contract.IDECodex,
			Projections: projections,
		},
	}); err != nil {
		t.Fatalf("write projection manifest: %v", err)
	}
}

func containsProjectionPath(projections []contract.GeneratedProjection, want string) bool {
	for _, projection := range projections {
		if projection.Path == want {
			return true
		}
	}
	return false
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
