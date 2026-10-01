package skills

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
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
)

const InstalledRoot = ".ai/skills/installed"

type InstallOptions struct {
	TargetDir string
	SourceDir string
}

type RemoveOptions struct {
	TargetDir string
	Name      string
}

type ListOptions struct {
	TargetDir string
}

type InstalledSkill struct {
	Name        string
	Description string
	Source      string
	Dirs        []string
	Files       []InstalledFile
}

type InstalledFile struct {
	Path     string
	Content  []byte
	Mode     fs.FileMode
	Checksum string
}

func Install(opts InstallOptions) (contract.SkillInstallResult, error) {
	targetDir := defaultTargetDir(opts.TargetDir)
	sourceDir := strings.TrimSpace(opts.SourceDir)
	if sourceDir == "" {
		return contract.SkillInstallResult{}, fmt.Errorf("local skill directory is required")
	}
	skill, validation, err := ValidateSourceTree(sourceDir)
	if err != nil {
		return contract.SkillInstallResult{Validation: validation}, err
	}
	if err := ensureNoBundledCollision(targetDir, skill.Name); err != nil {
		return contract.SkillInstallResult{Validation: validation}, err
	}
	destination := filepath.Join(targetDir, filepath.FromSlash(SourcePath(skill.Name)))
	if _, err := os.Lstat(destination); err == nil {
		return contract.SkillInstallResult{Validation: validation}, fmt.Errorf("installed skill %q already exists; remove it before reinstalling", skill.Name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return contract.SkillInstallResult{Validation: validation}, fmt.Errorf("stat installed skill %q: %w", skill.Name, err)
	}
	if err := ensureNoSymlinkAncestors(targetDir, SourcePath(skill.Name)); err != nil {
		return contract.SkillInstallResult{Validation: validation}, err
	}
	for _, dir := range skill.Dirs {
		target := filepath.Join(destination, filepath.FromSlash(dir))
		if err := os.MkdirAll(target, 0o755); err != nil {
			return contract.SkillInstallResult{Validation: validation}, fmt.Errorf("create installed skill directory %s: %w", dir, err)
		}
	}
	for _, file := range skill.Files {
		target := filepath.Join(destination, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return contract.SkillInstallResult{Validation: validation}, fmt.Errorf("create parent for %s: %w", file.Path, err)
		}
		mode := file.Mode.Perm()
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(target, file.Content, mode); err != nil {
			return contract.SkillInstallResult{Validation: validation}, fmt.Errorf("write installed skill file %s: %w", file.Path, err)
		}
	}
	return contract.SkillInstallResult{
		Skill:      manifestFromSkill(skill),
		Validation: validation,
	}, nil
}

func List(opts ListOptions) (contract.SkillListResult, error) {
	targetDir := defaultTargetDir(opts.TargetDir)
	root := filepath.Join(targetDir, filepath.FromSlash(InstalledRoot))
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return contract.SkillListResult{}, nil
		}
		return contract.SkillListResult{}, fmt.Errorf("read installed skills: %w", err)
	}
	result := contract.SkillListResult{Skills: []contract.InstalledSkillManifest{}}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skill, validation, err := ValidateSourceTree(filepath.Join(root, entry.Name()))
		if err != nil {
			return result, fmt.Errorf("read installed skill %s: %w", entry.Name(), err)
		}
		if !validation.Valid {
			return result, fmt.Errorf("read installed skill %s: invalid source tree", entry.Name())
		}
		result.Skills = append(result.Skills, manifestFromSkill(skill))
	}
	sort.Slice(result.Skills, func(i, j int) bool {
		return result.Skills[i].Name < result.Skills[j].Name
	})
	return result, nil
}

func Remove(opts RemoveOptions) (contract.SkillRemoveResult, error) {
	targetDir := defaultTargetDir(opts.TargetDir)
	name := strings.TrimSpace(opts.Name)
	if err := contract.ValidateInstalledSkillName(name); err != nil {
		return contract.SkillRemoveResult{}, err
	}

	result := contract.SkillRemoveResult{Name: name}
	sourcePath := SourcePath(name)
	if err := ensureNoSymlinkAncestors(targetDir, sourcePath); err != nil {
		return result, err
	}
	sourceFullPath := filepath.Join(targetDir, filepath.FromSlash(sourcePath))
	sourceExists := false
	if info, err := os.Lstat(sourceFullPath); err == nil {
		if info.Mode()&fs.ModeSymlink != 0 {
			return result, fmt.Errorf("installed skill source %s is a symlink", sourcePath)
		}
		sourceExists = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("stat installed skill source: %w", err)
	}

	manifest, err := templatesync.ReadManifest(targetDir)
	if err != nil {
		return result, err
	}
	projectionPlan, err := planProjectionRemoval(targetDir, manifest, name)
	if err != nil {
		return result, err
	}

	if sourceExists {
		if err := os.RemoveAll(sourceFullPath); err != nil {
			return result, fmt.Errorf("remove installed skill source: %w", err)
		}
		result.RemovedSource = true
	}
	result.RemovedProjections = append(result.RemovedProjections, projectionPlan.removed...)
	result.PreservedProjections = append(result.PreservedProjections, projectionPlan.preserved...)
	for _, deletion := range projectionPlan.deletions {
		if err := os.Remove(filepath.Join(targetDir, filepath.FromSlash(deletion.path))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return result, fmt.Errorf("remove projection %s: %w", deletion.path, err)
		}
		removeEmptyParents(targetDir, deletion.path, deletion.stopAt)
	}
	if manifest == nil {
		return result, nil
	}
	manifest.Generated.Projections = projectionPlan.remaining
	if err := templatesync.WriteManifest(targetDir, *manifest); err != nil {
		return result, err
	}
	return result, nil
}

type projectionRemovalPlan struct {
	remaining []contract.GeneratedProjection
	removed   []contract.InstalledSkillProjection
	preserved []contract.InstalledSkillProjection
	deletions []projectionDeletion
}

type projectionDeletion struct {
	path   string
	stopAt string
}

func planProjectionRemoval(targetDir string, manifest *contract.TargetManifest, name string) (projectionRemovalPlan, error) {
	var plan projectionRemovalPlan
	if manifest == nil || len(manifest.Generated.Projections) == 0 {
		return plan, nil
	}
	remaining := make([]contract.GeneratedProjection, 0, len(manifest.Generated.Projections))
	for _, projection := range manifest.Generated.Projections {
		if !projectionBelongsToSkill(projection, name) {
			remaining = append(remaining, projection)
			continue
		}
		converted := contract.InstalledSkillProjection{
			IDE:      projection.IDE,
			Source:   projection.Source,
			Path:     projection.Path,
			Checksum: projection.Checksum,
			Mode:     projection.Mode,
		}
		if err := safeProjectionPath(projection, name); err != nil {
			converted.State = contract.ProjectionStateDiverged
			plan.preserved = append(plan.preserved, converted)
			remaining = append(remaining, projection)
			continue
		}
		if err := ensureNoSymlinkAncestors(targetDir, projection.Path); err != nil {
			converted.State = contract.ProjectionStateDiverged
			plan.preserved = append(plan.preserved, converted)
			remaining = append(remaining, projection)
			continue
		}
		currentChecksum, err := templatesync.FileChecksum(filepath.Join(targetDir, filepath.FromSlash(projection.Path)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				converted.State = contract.ProjectionStateRemoved
				plan.removed = append(plan.removed, converted)
				continue
			}
			return plan, fmt.Errorf("checksum projection %s: %w", projection.Path, err)
		}
		if currentChecksum != projection.Checksum {
			converted.State = contract.ProjectionStateDiverged
			plan.preserved = append(plan.preserved, converted)
			remaining = append(remaining, projection)
			continue
		}
		converted.State = contract.ProjectionStateRemoved
		plan.removed = append(plan.removed, converted)
		plan.deletions = append(plan.deletions, projectionDeletion{
			path:   projection.Path,
			stopAt: nativeSkillRoot(projection.IDE, name),
		})
	}
	plan.remaining = remaining
	return plan, nil
}

func ValidateSourceTree(sourceDir string) (InstalledSkill, contract.InstalledSkillSourceValidation, error) {
	var skill InstalledSkill
	validation := contract.InstalledSkillSourceValidation{Valid: false}
	rootInfo, err := os.Lstat(sourceDir)
	if err != nil {
		validation.Errors = append(validation.Errors, err.Error())
		return skill, validation, fmt.Errorf("stat local skill directory: %w", err)
	}
	if !rootInfo.IsDir() {
		err := fmt.Errorf("local skill source must be a directory")
		validation.Errors = append(validation.Errors, err.Error())
		return skill, validation, err
	}
	if rootInfo.Mode()&fs.ModeSymlink != 0 {
		err := fmt.Errorf("local skill source must not be a symlink")
		validation.Errors = append(validation.Errors, err.Error())
		return skill, validation, err
	}
	sourceDir, err = filepath.Abs(sourceDir)
	if err != nil {
		validation.Errors = append(validation.Errors, err.Error())
		return skill, validation, err
	}

	err = filepath.WalkDir(sourceDir, func(fullPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fullPath == sourceDir {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, fullPath)
		if err != nil {
			return err
		}
		normalized := filepath.ToSlash(rel)
		if err := validateTreePath(normalized); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("installed skill source must not contain symlink %s", normalized)
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("installed skill source must not contain special file %s", normalized)
		}
		if info.IsDir() {
			skill.Dirs = append(skill.Dirs, normalized)
			return nil
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return err
		}
		skill.Files = append(skill.Files, InstalledFile{
			Path:     normalized,
			Content:  content,
			Mode:     info.Mode().Perm(),
			Checksum: templatesync.BytesChecksum(content),
		})
		validation.Files++
		validation.Bytes += int64(len(content))
		return nil
	})
	if err != nil {
		validation.Errors = append(validation.Errors, err.Error())
		return skill, validation, err
	}
	sort.Slice(skill.Files, func(i, j int) bool {
		return skill.Files[i].Path < skill.Files[j].Path
	})
	sort.Strings(skill.Dirs)
	metadata, err := parseSkillMetadata(skill.Files)
	if err != nil {
		validation.Errors = append(validation.Errors, err.Error())
		return skill, validation, err
	}
	skill.Name = metadata["name"]
	skill.Description = metadata["description"]
	validation.Name = skill.Name
	validation.Description = skill.Description
	if err := contract.ValidateInstalledSkillName(skill.Name); err != nil {
		validation.Errors = append(validation.Errors, err.Error())
		return skill, validation, err
	}
	if filepath.Base(sourceDir) != skill.Name {
		err := fmt.Errorf("installed skill name %q must match source directory name %q", skill.Name, filepath.Base(sourceDir))
		validation.Errors = append(validation.Errors, err.Error())
		return skill, validation, err
	}
	if strings.TrimSpace(skill.Description) == "" {
		err := fmt.Errorf("installed skill description is required")
		validation.Errors = append(validation.Errors, err.Error())
		return skill, validation, err
	}
	validation.Valid = true
	skill.Source = SourcePath(skill.Name)
	return skill, validation, nil
}

func SourcePath(name string) string {
	return InstalledRoot + "/" + name
}

func LoadInstalledSkills(targetDir string) ([]InstalledSkill, error) {
	list, err := List(ListOptions{TargetDir: targetDir})
	if err != nil {
		return nil, err
	}
	out := make([]InstalledSkill, 0, len(list.Skills))
	for _, item := range list.Skills {
		skill, _, err := ValidateSourceTree(filepath.Join(defaultTargetDir(targetDir), filepath.FromSlash(item.SourcePath)))
		if err != nil {
			return nil, err
		}
		out = append(out, skill)
	}
	return out, nil
}

func parseSkillMetadata(files []InstalledFile) (map[string]string, error) {
	for _, file := range files {
		if file.Path != "SKILL.md" {
			continue
		}
		return parseFrontmatter(file.Content)
	}
	return nil, fmt.Errorf("installed skill source must contain SKILL.md")
}

func parseFrontmatter(content []byte) (map[string]string, error) {
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	values := map[string]string{}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			if values["name"] == "" {
				return nil, fmt.Errorf("SKILL.md frontmatter name is required")
			}
			if values["description"] == "" {
				return nil, fmt.Errorf("SKILL.md frontmatter description is required")
			}
			return values, nil
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		switch key {
		case "name", "description":
			values[key] = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return nil, fmt.Errorf("SKILL.md frontmatter is not closed")
}

func validateTreePath(value string) error {
	if strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || strings.Contains(value, ":") {
		return fmt.Errorf("installed skill path %q must be slash-relative", value)
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("installed skill path %q escapes source root", value)
	}
	for _, segment := range strings.Split(clean, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("installed skill path %q must not contain empty, current, or parent segments", value)
		}
		switch strings.ToLower(segment) {
		case ".git", ".hg", ".svn":
			return fmt.Errorf("installed skill path %q must not contain hidden VCS metadata", value)
		}
	}
	return nil
}

func ensureNoBundledCollision(targetDir, name string) error {
	bundledPath := filepath.Join(targetDir, ".ai", "skills", name+".md")
	if _, err := os.Lstat(bundledPath); err == nil {
		return fmt.Errorf("installed skill %q collides with bundled skill %s", name, ".ai/skills/"+name+".md")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat bundled skill collision: %w", err)
	}
	return nil
}

func manifestFromSkill(skill InstalledSkill) contract.InstalledSkillManifest {
	files := make([]contract.InstalledSkillFile, 0, len(skill.Files))
	treeHash := sha256.New()
	for _, file := range skill.Files {
		files = append(files, contract.InstalledSkillFile{
			Path:     file.Path,
			Checksum: file.Checksum,
			Mode:     formatMode(file.Mode),
		})
		fmt.Fprintf(treeHash, "%s\x00%s\x00%s\x00", file.Path, formatMode(file.Mode), file.Checksum)
	}
	return contract.InstalledSkillManifest{
		Name:        skill.Name,
		Description: skill.Description,
		SourcePath:  skill.Source,
		Checksum:    "sha256:" + hex.EncodeToString(treeHash.Sum(nil)),
		Files:       files,
	}
}

func projectionBelongsToSkill(projection contract.GeneratedProjection, name string) bool {
	return equalSlashPath(projection.Source, SourcePath(name))
}

func safeProjectionPath(projection contract.GeneratedProjection, name string) error {
	if err := projection.Validate(); err != nil {
		return err
	}
	root := nativeSkillRoot(projection.IDE, name)
	if root == "" {
		return fmt.Errorf("ide %q has no native skill projection root", projection.IDE)
	}
	if !equalSlashPath(projection.Path, root) && !hasSlashPrefix(projection.Path, root+"/") {
		return fmt.Errorf("projection path %q is outside %s", projection.Path, root)
	}
	return nil
}

func nativeSkillRoot(ide contract.IDE, name string) string {
	switch ide {
	case contract.IDECodex:
		return ".codex/skills/" + name
	case contract.IDECursor:
		return ".cursor/skills/" + name
	case contract.IDEClaude:
		return ".claude/skills/" + name
	default:
		return ""
	}
}

func ensureNoSymlinkAncestors(root, relativePath string) error {
	clean := path.Clean(strings.ReplaceAll(relativePath, "\\", "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return fmt.Errorf("path %q must stay inside target root", relativePath)
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

func removeEmptyParents(targetDir, relativePath, stopAt string) {
	dir := path.Dir(relativePath)
	for dir != "." && (equalSlashPath(dir, stopAt) || hasSlashPrefix(dir, stopAt+"/")) {
		full := filepath.Join(targetDir, filepath.FromSlash(dir))
		if err := os.Remove(full); err != nil {
			return
		}
		dir = path.Dir(dir)
	}
}

func equalSlashPath(left, right string) bool {
	return strings.EqualFold(path.Clean(strings.ReplaceAll(left, "\\", "/")), path.Clean(strings.ReplaceAll(right, "\\", "/")))
}

func hasSlashPrefix(value, prefix string) bool {
	cleanValue := strings.ToLower(path.Clean(strings.ReplaceAll(value, "\\", "/")))
	cleanPrefix := strings.ToLower(path.Clean(strings.ReplaceAll(prefix, "\\", "/")))
	return strings.HasPrefix(cleanValue, cleanPrefix)
}

func defaultTargetDir(targetDir string) string {
	if strings.TrimSpace(targetDir) == "" {
		return "."
	}
	return targetDir
}

func formatMode(mode fs.FileMode) string {
	if mode == 0 {
		return ""
	}
	return fmt.Sprintf("%#o", mode.Perm())
}
