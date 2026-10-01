package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
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
	previous, err := templatesync.ReadManifest(root)
	if err != nil {
		return Result{}, err
	}
	previousProjections := []contract.GeneratedProjection{}
	if previous != nil {
		previousProjections = append(previousProjections, previous.Generated.Projections...)
	}
	if err := preflightInstalledSkillProjections(root, ides, data.Installed, previousProjections); err != nil {
		return Result{}, err
	}
	stale, err := planStaleInstalledProjections(root, ides, data.Installed, previousProjections)
	if err != nil {
		return Result{}, err
	}

	var result Result
	selected := map[contract.IDE]bool{}
	for _, ide := range ides {
		selected[ide] = true
	}
	for _, projection := range previousProjections {
		if !selected[projection.IDE] {
			result.Projections = append(result.Projections, projection)
		}
	}
	result.Projections = append(result.Projections, stale.Preserved...)
	if err := applyStaleProjectionDeletes(root, stale.DeletePaths); err != nil {
		return Result{}, err
	}
	sharedAgentsWritten := false
	for _, ide := range ides {
		writeSharedAgents := false
		if ide == contract.IDECodex || ide == contract.IDECursor {
			writeSharedAgents = !sharedAgentsWritten
			sharedAgentsWritten = true
		}
		generated, err := generateIDE(root, ide, data, previousProjections, writeSharedAgents)
		if err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, generated.Written...)
		result.Projections = append(result.Projections, generated.Projections...)
	}
	return result, nil
}

func generateIDE(root string, ide contract.IDE, data sourceData, previousProjections []contract.GeneratedProjection, writeSharedAgents bool) (Result, error) {
	switch ide {
	case contract.IDEClaude:
		return generateClaude(root, data, previousProjections)
	case contract.IDECodex:
		return generateCodex(root, data, previousProjections, writeSharedAgents)
	case contract.IDECursor:
		return generateCursor(root, data, previousProjections, writeSharedAgents)
	case contract.IDECopilot:
		return writeOne(root, ".github/copilot-instructions.md", renderUnified(contract.IDECopilot, data))
	case contract.IDEWindsurf:
		return writeOne(root, ".windsurfrules", renderUnified(contract.IDEWindsurf, data))
	default:
		return Result{}, fmt.Errorf("unsupported IDE %q", ide)
	}
}

func generateClaude(root string, data sourceData, previousProjections []contract.GeneratedProjection) (Result, error) {
	var result Result
	for _, persona := range data.Personas {
		rel := filepath.ToSlash(filepath.Join(".claude", "agents", persona.Name+".md"))
		if err := writeFile(root, rel, renderClaudeAgent(persona, data.ModelDefaults)); err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, rel)
	}
	for _, skill := range data.Skills {
		rel := filepath.ToSlash(filepath.Join(".claude", "skills", skill.Name, "SKILL.md"))
		if err := writeFile(root, rel, renderProjectSkill(skill)); err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, rel)
	}
	installed, err := writeInstalledSkillProjections(root, contract.IDEClaude, data.Installed, previousProjections)
	if err != nil {
		return Result{}, err
	}
	result.Written = append(result.Written, installed.Written...)
	result.Projections = append(result.Projections, installed.Projections...)
	if err := writeFile(root, "CLAUDE.md", renderClaudeRoot(data)); err != nil {
		return Result{}, err
	}
	result.Written = append(result.Written, "CLAUDE.md")
	return result, nil
}

func generateCodex(root string, data sourceData, previousProjections []contract.GeneratedProjection, writeSharedAgents bool) (Result, error) {
	var result Result
	for _, persona := range data.Personas {
		rel := filepath.ToSlash(filepath.Join(".codex", "agents", persona.Name+".toml"))
		if err := writeFile(root, rel, renderCodexAgent(persona, data.ModelDefaults)); err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, rel)
	}
	for _, skill := range data.Skills {
		rel := filepath.ToSlash(filepath.Join(".codex", "skills", skill.Name, "SKILL.md"))
		if err := writeFile(root, rel, renderProjectSkill(skill)); err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, rel)
	}
	installed, err := writeInstalledSkillProjections(root, contract.IDECodex, data.Installed, previousProjections)
	if err != nil {
		return Result{}, err
	}
	result.Written = append(result.Written, installed.Written...)
	result.Projections = append(result.Projections, installed.Projections...)
	if writeSharedAgents {
		if err := writeFile(root, "AGENTS.md", renderSharedAgentsRoot(data)); err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, "AGENTS.md")
	}
	return result, nil
}

func generateCursor(root string, data sourceData, previousProjections []contract.GeneratedProjection, writeSharedAgents bool) (Result, error) {
	var result Result
	globs := cursorGovernanceGlobs(data.Facts.SourceRoot)
	for _, persona := range data.Personas {
		rel := filepath.ToSlash(filepath.Join(".cursor", "agents", persona.Name+".md"))
		if err := writeFile(root, rel, renderCursorAgent(persona, data.ModelDefaults)); err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, rel)
	}
	core := renderIntro(contract.IDECursor, data) + data.SharedBody
	if err := writeFile(root, ".cursor/rules/core.mdc", renderCursorMDC("Core architecture, spec gate, and main-agent delegation", true, "", core)); err != nil {
		return Result{}, err
	}
	result.Written = append(result.Written, ".cursor/rules/core.mdc")
	if err := writeFile(root, ".cursor/rules/governance-spec-gate.mdc", renderCursorGovernanceRule(globs)); err != nil {
		return Result{}, err
	}
	result.Written = append(result.Written, ".cursor/rules/governance-spec-gate.mdc")
	for _, persona := range data.Personas {
		rel := filepath.ToSlash(filepath.Join(".cursor", "rules", "persona-"+persona.Name+".mdc"))
		description := "Persona - " + persona.Name + ": " + persona.Description
		if err := writeFile(root, rel, renderCursorMDC(description, false, globs, persona.Body)); err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, rel)
	}
	for _, skill := range data.Skills {
		rel := filepath.ToSlash(filepath.Join(".cursor", "skills", skill.Name, "SKILL.md"))
		if err := writeFile(root, rel, skill.Raw); err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, rel)
	}
	installed, err := writeInstalledSkillProjections(root, contract.IDECursor, data.Installed, previousProjections)
	if err != nil {
		return Result{}, err
	}
	result.Written = append(result.Written, installed.Written...)
	result.Projections = append(result.Projections, installed.Projections...)
	if writeSharedAgents {
		if err := writeFile(root, "AGENTS.md", renderSharedAgentsRoot(data)); err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, "AGENTS.md")
	}
	return result, nil
}

func writeOne(root, rel string, content []byte) (Result, error) {
	if err := writeFile(root, rel, content); err != nil {
		return Result{}, err
	}
	return Result{Written: []string{rel}}, nil
}

func writeFile(root, rel string, content []byte) error {
	return writeFileMode(root, rel, content, 0o644)
}

func writeFileMode(root, rel string, content []byte, mode fs.FileMode) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if mode == 0 {
		mode = 0o644
	}
	return os.WriteFile(path, content, mode.Perm())
}

func sortedWritten(result Result) []string {
	out := append([]string(nil), result.Written...)
	sort.Strings(out)
	return out
}

func writeInstalledSkillProjections(root string, ide contract.IDE, installed []installedSkill, previous []contract.GeneratedProjection) (Result, error) {
	rootPrefix := nativeSkillProjectionRoot(ide)
	if rootPrefix == "" || len(installed) == 0 {
		return Result{}, nil
	}
	result := Result{}
	previousByPath := map[string]contract.GeneratedProjection{}
	for _, projection := range previous {
		if projection.IDE == ide {
			previousByPath[path.Clean(projection.Path)] = projection
		}
	}
	type pendingWrite struct {
		rel     string
		content []byte
		mode    fs.FileMode
		record  contract.GeneratedProjection
	}
	var dirs []string
	var writes []pendingWrite
	for _, skill := range installed {
		for _, dir := range skill.Dirs {
			rel := path.Join(rootPrefix, skill.Name, dir)
			if err := ensureNoSymlinkAncestors(root, rel); err != nil {
				return Result{}, err
			}
			dirs = append(dirs, rel)
		}
		for _, file := range skill.Files {
			rel := path.Join(rootPrefix, skill.Name, file.Path)
			mode, err := parseMode(file.Mode)
			if err != nil {
				return Result{}, fmt.Errorf("installed skill %s file %s mode: %w", skill.Name, file.Path, err)
			}
			record := contract.GeneratedProjection{
				IDE:      ide,
				Source:   skill.Source,
				Path:     rel,
				Checksum: bytesChecksum(file.Content),
				Mode:     formatMode(mode),
			}
			if err := record.Validate(); err != nil {
				return Result{}, err
			}
			if err := ensureNoSymlinkAncestors(root, rel); err != nil {
				return Result{}, err
			}
			if err := checkProjectionDestination(root, record, previousByPath[path.Clean(rel)]); err != nil {
				return Result{}, err
			}
			writes = append(writes, pendingWrite{rel: rel, content: file.Content, mode: mode, record: record})
		}
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
			return Result{}, err
		}
	}
	for _, write := range writes {
		if err := writeFileMode(root, write.rel, write.content, write.mode); err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, write.rel)
		result.Projections = append(result.Projections, write.record)
	}
	return result, nil
}

func preflightInstalledSkillProjections(root string, ides []contract.IDE, installed []installedSkill, previous []contract.GeneratedProjection) error {
	if len(installed) == 0 {
		return nil
	}
	for _, ide := range ides {
		rootPrefix := nativeSkillProjectionRoot(ide)
		if rootPrefix == "" {
			continue
		}
		previousByPath := map[string]contract.GeneratedProjection{}
		for _, projection := range previous {
			if projection.IDE == ide {
				previousByPath[path.Clean(projection.Path)] = projection
			}
		}
		for _, skill := range installed {
			for _, dir := range skill.Dirs {
				if err := ensureNoSymlinkAncestors(root, path.Join(rootPrefix, skill.Name, dir)); err != nil {
					return err
				}
			}
			for _, file := range skill.Files {
				mode, err := parseMode(file.Mode)
				if err != nil {
					return fmt.Errorf("installed skill %s file %s mode: %w", skill.Name, file.Path, err)
				}
				rel := path.Join(rootPrefix, skill.Name, file.Path)
				record := contract.GeneratedProjection{
					IDE:      ide,
					Source:   skill.Source,
					Path:     rel,
					Checksum: bytesChecksum(file.Content),
					Mode:     formatMode(mode),
				}
				if err := record.Validate(); err != nil {
					return err
				}
				if err := ensureNoSymlinkAncestors(root, rel); err != nil {
					return err
				}
				if err := checkProjectionDestination(root, record, previousByPath[path.Clean(rel)]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type staleProjectionPlan struct {
	DeletePaths []string
	Preserved   []contract.GeneratedProjection
}

func planStaleInstalledProjections(root string, ides []contract.IDE, installed []installedSkill, previous []contract.GeneratedProjection) (staleProjectionPlan, error) {
	selected := map[contract.IDE]bool{}
	for _, ide := range ides {
		selected[ide] = true
	}
	desired := map[string]bool{}
	for _, ide := range ides {
		rootPrefix := nativeSkillProjectionRoot(ide)
		if rootPrefix == "" {
			continue
		}
		for _, skill := range installed {
			for _, file := range skill.Files {
				desired[projectionKey(ide, path.Join(rootPrefix, skill.Name, file.Path))] = true
			}
		}
	}

	var plan staleProjectionPlan
	for _, projection := range previous {
		if !selected[projection.IDE] || desired[projectionKey(projection.IDE, projection.Path)] {
			continue
		}
		action, err := staleProjectionAction(root, projection)
		if err != nil {
			return staleProjectionPlan{}, err
		}
		switch action {
		case staleProjectionDelete:
			plan.DeletePaths = append(plan.DeletePaths, projection.Path)
		case staleProjectionPreserve:
			plan.Preserved = append(plan.Preserved, projection)
		}
	}
	return plan, nil
}

type staleProjectionDecision int

const (
	staleProjectionPreserve staleProjectionDecision = iota
	staleProjectionDelete
)

func staleProjectionAction(root string, projection contract.GeneratedProjection) (staleProjectionDecision, error) {
	if err := safeOwnedProjection(projection); err != nil {
		return staleProjectionPreserve, nil
	}
	if err := ensureNoSymlinkAncestors(root, projection.Path); err != nil {
		return staleProjectionPreserve, nil
	}
	currentChecksum, err := templatesync.FileChecksum(filepath.Join(root, filepath.FromSlash(projection.Path)))
	if errors.Is(err, os.ErrNotExist) {
		return staleProjectionDelete, nil
	}
	if err != nil {
		return staleProjectionPreserve, nil
	}
	if currentChecksum != projection.Checksum {
		return staleProjectionPreserve, nil
	}
	return staleProjectionDelete, nil
}

func safeOwnedProjection(projection contract.GeneratedProjection) error {
	if err := projection.Validate(); err != nil {
		return err
	}
	name := installedSkillNameFromSource(projection.Source)
	if name == "" {
		return fmt.Errorf("projection source %q is not an installed skill source", projection.Source)
	}
	root := nativeSkillProjectionRoot(projection.IDE)
	if root == "" {
		return fmt.Errorf("ide %q has no native skill projection root", projection.IDE)
	}
	prefix := path.Join(root, name)
	cleanPath := path.Clean(projection.Path)
	if cleanPath != prefix && !strings.HasPrefix(cleanPath, prefix+"/") {
		return fmt.Errorf("projection path %q is outside %s", projection.Path, prefix)
	}
	return nil
}

func installedSkillNameFromSource(source string) string {
	const prefix = ".ai/skills/installed/"
	clean := path.Clean(source)
	if !strings.HasPrefix(clean, prefix) {
		return ""
	}
	name := strings.TrimPrefix(clean, prefix)
	if name == "" || strings.Contains(name, "/") {
		return ""
	}
	return name
}

func projectionKey(ide contract.IDE, rel string) string {
	return ide.String() + "\x00" + path.Clean(rel)
}

func applyStaleProjectionDeletes(root string, paths []string) error {
	for _, rel := range paths {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove stale projection %s: %w", rel, err)
		}
	}
	return nil
}

func checkProjectionDestination(root string, next contract.GeneratedProjection, previous contract.GeneratedProjection) error {
	current, err := templatesync.FileChecksum(filepath.Join(root, filepath.FromSlash(next.Path)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checksum projection %s: %w", next.Path, err)
	}
	if current == next.Checksum {
		return nil
	}
	if previous.Path == "" {
		return fmt.Errorf("installed skill projection %s already exists and differs from source", next.Path)
	}
	if err := previous.Validate(); err != nil {
		return fmt.Errorf("previous installed skill projection %s is invalid: %w", next.Path, err)
	}
	if current != previous.Checksum {
		return fmt.Errorf("installed skill projection %s diverged from previous generated checksum", next.Path)
	}
	return nil
}

func nativeSkillProjectionRoot(ide contract.IDE) string {
	switch ide {
	case contract.IDECodex:
		return ".codex/skills"
	case contract.IDECursor:
		return ".cursor/skills"
	case contract.IDEClaude:
		return ".claude/skills"
	default:
		return ""
	}
}

func ensureNoSymlinkAncestors(root, rel string) error {
	clean := path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return fmt.Errorf("path %q must stay inside target root", rel)
	}
	current := root
	for _, segment := range strings.Split(clean, "/") {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("stat %s: %w", clean, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("path %s has symlink ancestor %s", clean, segment)
		}
	}
	return nil
}

func bytesChecksum(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func parseMode(value string) (fs.FileMode, error) {
	if strings.TrimSpace(value) == "" {
		return 0o644, nil
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 8, 32)
	if err != nil {
		return 0, err
	}
	return fs.FileMode(parsed).Perm(), nil
}

func formatMode(mode fs.FileMode) string {
	if mode == 0 {
		mode = 0o644
	}
	return fmt.Sprintf("%04o", mode.Perm())
}
