package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

func Generate(options Options) (Result, error) {
	if options.TargetDir == "" {
		return Result{}, fmt.Errorf("target directory is required")
	}
	ides, err := selectedIDEs(options)
	if err != nil {
		return Result{}, err
	}
	root, err := filepath.Abs(options.TargetDir)
	if err != nil {
		return Result{}, err
	}
	data, err := loadSourceData(root)
	if err != nil {
		return Result{}, err
	}

	var result Result
	for _, ide := range ides {
		written, err := generateIDE(root, ide, data)
		if err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, written...)
	}
	return result, nil
}

func generateIDE(root string, ide contract.IDE, data sourceData) ([]string, error) {
	switch ide {
	case contract.IDEClaude:
		return generateClaude(root, data)
	case contract.IDECodex:
		return generateCodex(root, data)
	case contract.IDECursor:
		return generateCursor(root, data)
	case contract.IDECopilot:
		return writeOne(root, ".github/copilot-instructions.md", renderUnified(contract.IDECopilot, data))
	case contract.IDEWindsurf:
		return writeOne(root, ".windsurfrules", renderUnified(contract.IDEWindsurf, data))
	default:
		return nil, fmt.Errorf("unsupported IDE %q", ide)
	}
}

func generateClaude(root string, data sourceData) ([]string, error) {
	var written []string
	for _, persona := range data.Personas {
		rel := filepath.ToSlash(filepath.Join(".claude", "agents", persona.Name+".md"))
		if err := writeFile(root, rel, renderClaudeAgent(persona, data.ModelDefaults)); err != nil {
			return nil, err
		}
		written = append(written, rel)
	}
	for _, skill := range data.Skills {
		rel := filepath.ToSlash(filepath.Join(".claude", "skills", skill.Name, "SKILL.md"))
		if err := writeFile(root, rel, renderProjectSkill(skill)); err != nil {
			return nil, err
		}
		written = append(written, rel)
	}
	var b []byte
	b = append(b, renderIntro(contract.IDEClaude, data)...)
	b = append(b, data.SharedBody...)
	b = append(b, []byte("\n## Personas\n\nInvokable as Claude subagents under `.claude/agents/`.\n\n")...)
	for _, persona := range data.Personas {
		b = fmt.Appendf(b, "- `%s` — %s\n", persona.Name, persona.Description)
	}
	b = append(b, []byte("\n## Skills\n\nInvokable as Claude skills under `.claude/skills/`.\n\n")...)
	for _, skill := range data.Skills {
		b = fmt.Appendf(b, "- `%s` — %s\n", skill.Name, skill.Description)
	}
	if err := writeFile(root, "CLAUDE.md", b); err != nil {
		return nil, err
	}
	written = append(written, "CLAUDE.md")
	return written, nil
}

func generateCodex(root string, data sourceData) ([]string, error) {
	var written []string
	for _, persona := range data.Personas {
		rel := filepath.ToSlash(filepath.Join(".codex", "agents", persona.Name+".toml"))
		if err := writeFile(root, rel, renderCodexAgent(persona, data.ModelDefaults)); err != nil {
			return nil, err
		}
		written = append(written, rel)
	}
	for _, skill := range data.Skills {
		rel := filepath.ToSlash(filepath.Join(".codex", "skills", skill.Name, "SKILL.md"))
		if err := writeFile(root, rel, renderProjectSkill(skill)); err != nil {
			return nil, err
		}
		written = append(written, rel)
	}
	var b []byte
	b = append(b, renderIntro(contract.IDECodex, data)...)
	b = append(b, data.SharedBody...)
	b = append(b, []byte("\n## Codex Agents\n\nCodex project instructions in `AGENTS.md` shape the main session. Delegable custom agents live under `.codex/agents/`.\n\n")...)
	for _, persona := range data.Personas {
		b = fmt.Appendf(b, "- `%s` — `.codex/agents/%s.toml`\n", persona.Name, persona.Name)
	}
	b = append(b, []byte("\n## Codex Skills\n\nAgent Skills under `.codex/skills/<name>/SKILL.md` (from `.ai/skills/` via `make init codex`).\n\n")...)
	for _, skill := range data.Skills {
		b = fmt.Appendf(b, "- `%s` — `.codex/skills/%s/SKILL.md` — %s\n", skill.Name, skill.Name, skill.Description)
	}
	b = append(b, []byte("\n## Persona Reference\n\n")...)
	for _, persona := range data.Personas {
		b = fmt.Appendf(b, "### Persona — %s\n\n%s\n\n", persona.Name, persona.Body)
	}
	b = append(b, []byte("## Skill Reference\n\n")...)
	for _, skill := range data.Skills {
		b = fmt.Appendf(b, "### Skill — %s\n\n%s\n\n", skill.Name, skill.Body)
	}
	if err := writeFile(root, "AGENTS.md", b); err != nil {
		return nil, err
	}
	written = append(written, "AGENTS.md")
	return written, nil
}

func generateCursor(root string, data sourceData) ([]string, error) {
	var written []string
	globs := cursorGovernanceGlobs(data.Facts.SourceRoot)
	for _, persona := range data.Personas {
		rel := filepath.ToSlash(filepath.Join(".cursor", "agents", persona.Name+".md"))
		if err := writeFile(root, rel, renderCursorAgent(persona, data.ModelDefaults)); err != nil {
			return nil, err
		}
		written = append(written, rel)
	}
	core := renderIntro(contract.IDECursor, data) + data.SharedBody
	if err := writeFile(root, ".cursor/rules/core.mdc", renderCursorMDC("Core architecture, spec gate, and main-agent delegation", true, "", core)); err != nil {
		return nil, err
	}
	written = append(written, ".cursor/rules/core.mdc")
	if err := writeFile(root, ".cursor/rules/governance-spec-gate.mdc", renderCursorGovernanceRule(globs)); err != nil {
		return nil, err
	}
	written = append(written, ".cursor/rules/governance-spec-gate.mdc")
	for _, persona := range data.Personas {
		rel := filepath.ToSlash(filepath.Join(".cursor", "rules", "persona-"+persona.Name+".mdc"))
		description := "Persona - " + persona.Name + ": " + persona.Description
		if err := writeFile(root, rel, renderCursorMDC(description, false, globs, persona.Body)); err != nil {
			return nil, err
		}
		written = append(written, rel)
	}
	for _, skill := range data.Skills {
		rel := filepath.ToSlash(filepath.Join(".cursor", "skills", skill.Name, "SKILL.md"))
		if err := writeFile(root, rel, skill.Raw); err != nil {
			return nil, err
		}
		written = append(written, rel)
	}

	var b []byte
	b = append(b, renderIntro(contract.IDECursor, data)...)
	b = append(b, data.SharedBody...)
	b = append(b, []byte("\n## Cursor agents\n\nDelegable agents under `.cursor/agents/` (also exposed as Task `subagent_type` when rules are loaded):\n\n")...)
	for _, persona := range data.Personas {
		b = fmt.Appendf(b, "- `%s` — `.cursor/agents/%s.md` — %s\n", persona.Name, persona.Name, persona.Description)
	}
	b = append(b, []byte("\n## Cursor rules\n\n")...)
	b = append(b, []byte("- `core.mdc` — always applied (routing, spec gate, delegation)\n")...)
	b = fmt.Appendf(b, "- `governance-spec-gate.mdc` — globs: `%s`\n", globs)
	for _, persona := range data.Personas {
		b = fmt.Appendf(b, "- `persona-%s.mdc` — same globs as governance rule\n", persona.Name)
	}
	b = append(b, []byte("\n## Cursor skills\n\nAgent Skills under `.cursor/skills/<name>/SKILL.md` (from `.ai/skills/` via `make init cursor`).\n")...)
	b = append(b, []byte("Invoke with `/` + skill name or let Agent decide from the skill description.\n\n")...)
	for _, skill := range data.Skills {
		b = fmt.Appendf(b, "- `%s` — `.cursor/skills/%s/SKILL.md` — %s\n", skill.Name, skill.Name, skill.Description)
	}
	b = append(b, []byte("\nRegenerate after changing `.ai/`: `make init cursor`.\n")...)
	if err := writeFile(root, "AGENTS.md", b); err != nil {
		return nil, err
	}
	written = append(written, "AGENTS.md")
	return written, nil
}

func writeOne(root, rel string, content []byte) ([]string, error) {
	if err := writeFile(root, rel, content); err != nil {
		return nil, err
	}
	return []string{rel}, nil
}

func writeFile(root, rel string, content []byte) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func sortedWritten(result Result) []string {
	out := append([]string(nil), result.Written...)
	sort.Strings(out)
	return out
}
