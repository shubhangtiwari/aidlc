package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aidlc/ai-dlc-template/aidlc/internal/contract"
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
	b.WriteString("\n## Personas\n\n")
	for _, persona := range data.Personas {
		fmt.Fprintf(&b, "### Persona — %s\n\n%s\n\n", persona.Name, persona.Body)
	}
	b.WriteString("## Skills\n\n")
	for _, skill := range data.Skills {
		fmt.Fprintf(&b, "### Skill — %s\n\n%s\n\n", skill.Name, skill.Body)
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
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", persona.Name)
	fmt.Fprintf(&b, "description: %s\n", jsonQuote(persona.Description))
	if model := defaultFor(defaults, "claude", persona.Name).Model; model != "" {
		fmt.Fprintf(&b, "model: %s\n", model)
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
	if persona.Name == "architect" || persona.Name == "reviewer" {
		b.WriteString("sandbox_mode = \"read-only\"\n")
	}
	fmt.Fprintf(&b, "developer_instructions = %s\n", tomlMultilineString(persona.Body))
	return []byte(b.String())
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
		"Portable rules: `.ai/README.md`. Parallel waves: skill `orchestrate-spec`.\n\n" +
		`## Main session

- **Do not** edit governed paths when tier is medium, large, or uncertain without delegating.
- **Do** launch ` + "`architect` to draft `docs/spec/<epoch>-<slug>.md`, then stop for approval.\n" +
		"- **Do** apply skill `classify-change` in the main session before inline intent, spec, or\n" +
		"  implementer on governed paths (Hard Rule 7). Do not delegate triage to `architect`. When triage\n" +
		"  is medium/large/uncertain (`next: draft-spec`), delegate `architect` for planning.\n" +
		"- **Do** launch `implementer` for all governed edits: after `status: approved`, or after\n" +
		"  trivial/small intent is confirmed following triage (no spec). Main session does not patch governed source.\n" +
		"- **Do** expect implementer blueprint sanity on every run (update `docs/blueprints/` when needed).\n" +
		"- Check `docs/spec/.in-flight.yaml` for specs tied to the current branch.\n\n" +
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
