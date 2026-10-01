package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

func renderIntro(ide contract.IDE, data sourceData) string {
	var b strings.Builder
	if data.Facts.HasManifest {
		fmt.Fprintf(&b, "<!-- generated from .ai/ + %s -- do not edit by hand. Run `make init %s` to regenerate. -->\n\n", data.Facts.ManifestPath, markerIDE(ide))
	} else {
		fmt.Fprintf(&b, "<!-- generated from .ai/ -- do not edit by hand. Run `make init %s` to regenerate. -->\n\n", markerIDE(ide))
	}
	fmt.Fprintf(&b, "# AI Governance — %s\n\n", data.Facts.ProjectName)
	b.WriteString("Source of truth: `.ai/` for portable guidance, `docs/` for architecture and contracts, and optional project manifests for toolchain facts. This file is generated.\n\n")
	b.WriteString(renderProjectFactsBlock(data.Facts))
	return b.String()
}

func renderProjectFactsBlock(facts ProjectFacts) string {
	var b strings.Builder
	b.WriteString("## Project facts\n\n")
	fmt.Fprintf(&b, "- Project: `%s`\n", facts.ProjectName)
	if facts.HasManifest {
		if facts.Language != "" {
			fmt.Fprintf(&b, "- Language: %s\n", facts.Language)
		}
		fmt.Fprintf(&b, "- Manifest: `%s`\n", facts.ManifestPath)
	} else {
		b.WriteString("- Manifest: not detected (optional — re-run `make init <ide>` after adding one)\n")
	}
	if facts.SourceRoot == "." {
		b.WriteString("- Source root: repository root\n")
	} else {
		fmt.Fprintf(&b, "- Source root: `%s/`\n", facts.SourceRoot)
	}
	if facts.PackageName != "" {
		fmt.Fprintf(&b, "- Package/import namespace: `%s`\n", facts.PackageName)
	}
	if facts.ModulePath != "" {
		fmt.Fprintf(&b, "- Module path: `%s`\n", facts.ModulePath)
	}
	if facts.Runtime != "" {
		fmt.Fprintf(&b, "- Runtime/version constraint: `%s`\n", facts.Runtime)
	}
	if facts.BuildTool != "" {
		fmt.Fprintf(&b, "- Build tool: `%s`\n", facts.BuildTool)
	}
	b.WriteString("- Architecture and layer rules: see `docs/ARCHITECTURE.md` and `docs/architecture/`.\n")
	b.WriteString("- Module contracts and read-only paths: see `docs/blueprints/`.\n")
	b.WriteString("- Execute via `Makefile` only.\n\n")
	return b.String()
}

func renderUnified(ide contract.IDE, data sourceData) []byte {
	var b strings.Builder
	b.WriteString(renderIntro(ide, data))
	b.WriteString(data.SharedBody)
	b.WriteString("\n## Native Agent Support\n\n")
	switch ide {
	case contract.IDECopilot:
		b.WriteString("Copilot uses this root instruction file plus the portable `.ai/` source tree. Full persona and skill bodies remain in `.ai/personas/` and `.ai/skills/`.\n")
	case contract.IDEWindsurf:
		b.WriteString("Windsurf uses this root rules file plus the portable `.ai/` source tree. Full persona and skill bodies remain in `.ai/personas/` and `.ai/skills/`.\n")
	default:
		b.WriteString("Full persona and skill bodies remain in `.ai/` and IDE-native folders when supported.\n")
	}
	b.WriteString("\n## Personas\n\n")
	for _, persona := range data.Personas {
		fmt.Fprintf(&b, "- `%s` — `.ai/personas/%s.md` — %s\n", persona.Name, persona.Name, persona.Description)
	}
	b.WriteString("\n## Skills\n\n")
	for _, skill := range data.Skills {
		fmt.Fprintf(&b, "- `%s` — `.ai/skills/%s.md` — %s\n", skill.Name, skill.Name, skill.Description)
	}
	if len(data.Installed) > 0 {
		b.WriteString("\n## Installed Skills\n\n")
		renderInstalledSkillCatalog(&b, data.Installed, "portable")
	}
	return []byte(b.String())
}

func renderProjectSkill(skill document) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", skill.Name)
	fmt.Fprintf(&b, "description: %s\n", jsonQuote(skill.Description))
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "%s\n", skill.Body)
	return []byte(b.String())
}

func renderClaudeAgent(persona document, defaults map[string]map[string]modelDefault) []byte {
	var b strings.Builder
	def := defaultFor(defaults, "claude", persona.Name)
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", persona.Name)
	fmt.Fprintf(&b, "description: %s\n", jsonQuote(persona.Description))
	if def.Model != "" {
		fmt.Fprintf(&b, "model: %s\n", def.Model)
	}
	if def.Effort != "" {
		fmt.Fprintf(&b, "effort: %s\n", def.Effort)
	}
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "%s\n", persona.Body)
	return []byte(b.String())
}

func renderCursorAgent(persona document, defaults map[string]map[string]modelDefault) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", persona.Name)
	fmt.Fprintf(&b, "description: %s\n", jsonQuote(persona.Description))
	if model := defaultFor(defaults, "cursor", persona.Name).Model; model != "" {
		fmt.Fprintf(&b, "model: %s\n", model)
	}
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "%s\n", persona.Body)
	return []byte(b.String())
}

func renderCodexAgent(persona document, defaults map[string]map[string]modelDefault) []byte {
	var b strings.Builder
	def := defaultFor(defaults, "codex", persona.Name)
	fmt.Fprintf(&b, "name = %s\n", tomlBasicString(persona.Name))
	fmt.Fprintf(&b, "description = %s\n", tomlBasicString(persona.Description))
	if def.Model != "" {
		fmt.Fprintf(&b, "model = %s\n", tomlBasicString(def.Model))
	}
	if def.Reasoning != "" {
		fmt.Fprintf(&b, "model_reasoning_effort = %s\n", tomlBasicString(def.Reasoning))
	}
	if persona.Name == "architect" {
		b.WriteString("sandbox_mode = \"workspace-write\"\n")
	} else if persona.Name == "reviewer" {
		b.WriteString("sandbox_mode = \"read-only\"\n")
	}
	fmt.Fprintf(&b, "developer_instructions = %s\n", tomlMultilineString(persona.Body))
	return []byte(b.String())
}

func renderSharedAgentsRoot(data sourceData) []byte {
	var b strings.Builder
	b.WriteString(renderIntro(contract.IDEAll, data))
	b.WriteString(data.SharedBody)
	b.WriteString("\n## Native Agents\n\n")
	b.WriteString("Codex custom agents live under `.codex/agents/`; Cursor agents live under `.cursor/agents/` and Cursor rules under `.cursor/rules/`.\n\n")
	for _, persona := range data.Personas {
		fmt.Fprintf(&b, "- `%s` — %s\n", persona.Name, persona.Description)
	}
	b.WriteString("\n## Native Skills\n\n")
	b.WriteString("Bundled skill bodies are projected into `.codex/skills/` and `.cursor/skills/`. Full portable sources remain in `.ai/skills/`.\n\n")
	for _, skill := range data.Skills {
		fmt.Fprintf(&b, "- `%s` — %s\n", skill.Name, skill.Description)
	}
	if len(data.Installed) > 0 {
		b.WriteString("\n## Installed Skills\n\n")
		renderInstalledSkillCatalog(&b, data.Installed, "codex-cursor")
	}
	b.WriteString("\nRegenerate after changing `.ai/`: `make init codex`, `make init cursor`, or `make init all`.\n")
	return []byte(b.String())
}

func renderClaudeRoot(data sourceData) []byte {
	var b strings.Builder
	b.WriteString(renderIntro(contract.IDEClaude, data))
	b.WriteString(data.SharedBody)
	b.WriteString("\n## Personas\n\nInvokable as Claude subagents under `.claude/agents/`.\n\n")
	for _, persona := range data.Personas {
		fmt.Fprintf(&b, "- `%s` — %s\n", persona.Name, persona.Description)
	}
	b.WriteString("\n## Skills\n\nInvokable as Claude skills under `.claude/skills/`.\n\n")
	for _, skill := range data.Skills {
		fmt.Fprintf(&b, "- `%s` — %s\n", skill.Name, skill.Description)
	}
	if len(data.Installed) > 0 {
		b.WriteString("\n## Installed Skills\n\n")
		renderInstalledSkillCatalog(&b, data.Installed, "claude")
	}
	return []byte(b.String())
}

func renderInstalledSkillCatalog(b *strings.Builder, installed []installedSkill, surface string) {
	const maxListedInstalledSkills = 25
	limit := len(installed)
	if limit > maxListedInstalledSkills {
		limit = maxListedInstalledSkills
	}
	for i := 0; i < limit; i++ {
		skill := installed[i]
		switch surface {
		case "claude":
			fmt.Fprintf(b, "- `%s` — `.claude/skills/%s/SKILL.md` — %s\n", skill.Name, skill.Name, skill.Description)
		case "codex-cursor":
			fmt.Fprintf(b, "- `%s` — `.codex/skills/%s/SKILL.md`, `.cursor/skills/%s/SKILL.md` — %s\n", skill.Name, skill.Name, skill.Name, skill.Description)
		default:
			fmt.Fprintf(b, "- `%s` — `%s/SKILL.md` — %s\n", skill.Name, skill.Source, skill.Description)
		}
	}
	if len(installed) > limit {
		fmt.Fprintf(b, "- %d more installed skill(s): see `.ai/skills/installed/` and supported native skill folders.\n", len(installed)-limit)
	}
}

func renderCursorMDC(description string, alwaysApply bool, globs string, body string) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: %s\n", jsonQuote(description))
	if globs != "" {
		fmt.Fprintf(&b, "globs: %s\n", globs)
	}
	fmt.Fprintf(&b, "alwaysApply: %t\n", alwaysApply)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "%s\n", body)
	return []byte(b.String())
}

func renderCursorGovernanceRule(globs string) []byte {
	body := `# Governed paths — spec gate

Applies under the source root, ` + "`tests/`, `docs/spec/`, `docs/blueprints/`, `docs/adr/`,\n" +
		"`docs/ARCHITECTURE.md`, and `docs/architecture/`.\n\n" +
		"Portable rules: `.ai/README.md` (especially Scope resolution). Parallel waves: skill `orchestrate-spec`.\n\n" +
		`## Main session

- **Do not** edit governed paths when tier is medium, large, or uncertain without delegating.
- **Do** launch ` + "`architect` to draft scope-local spec file(s), then stop for approval.\n" +
		"  Scope-local specs live at `docs/spec/<epoch>-<slug>.md` relative to each resolved AIDLC scope root.\n" +
		"- **Do** apply skill `classify-change` in the main session before inline intent, spec, or\n" +
		"  implementer on governed paths (Hard Rule 7). Do not delegate triage to `architect`. When triage\n" +
		"  is medium/large/uncertain (`next: draft-spec`), delegate `architect` for planning.\n" +
		"- **Do** launch `implementer` for all governed edits: after `status: approved`, or after\n" +
		"  trivial/small intent is confirmed following triage (no spec). Main session does not patch governed source.\n" +
		"- **Do** expect implementer blueprint sanity on every run (update `docs/blueprints/` when needed).\n" +
		"- Check `docs/spec/.in-flight.yaml` in the owning scope for specs tied to the current branch.\n\n" +
		`## Review

| Tier | Reviewer |
| --- | --- |
| Trivial / small | Skip unless the user explicitly asks for a review |
| Medium / large (approved spec) | **Required** after implementer — diff vs spec; do not report complete or open a PR until ` + "`reviewer` runs |\n\n" +
		"Portable rules: Hard Rule 6 in `.ai/README.md`."
	return renderCursorMDC("Spec gate when editing source, tests, or contract docs — delegate before patching", false, globs, body)
}

func cursorGovernanceGlobs(sourceRoot string) string {
	if sourceRoot == "." {
		return "{**/*,tests/**,docs/spec/**,docs/blueprints/**,docs/adr/**,docs/ARCHITECTURE.md,docs/architecture/**}"
	}
	return fmt.Sprintf("{%s/**,tests/**,docs/spec/**,docs/blueprints/**,docs/adr/**,docs/ARCHITECTURE.md,docs/architecture/**}", sourceRoot)
}

func defaultFor(defaults map[string]map[string]modelDefault, ide, persona string) modelDefault {
	if defaults == nil {
		return modelDefault{}
	}
	return defaults[ide][persona]
}

func jsonQuote(value string) string {
	out, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(out)
}

func tomlBasicString(value string) string {
	var b bytes.Buffer
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func tomlMultilineString(value string) string {
	return "\"\"\"\n" + strings.ReplaceAll(value, `"""`, `\"""`) + "\n\"\"\""
}
