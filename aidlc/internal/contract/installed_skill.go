package contract

import (
	"fmt"
	"strings"
)

const (
	ProjectionStateOwned    = "owned"
	ProjectionStateDiverged = "diverged"
	ProjectionStateRemoved  = "removed"
)

type InstalledSkillManifest struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	SourcePath  string                     `json:"source_path"`
	Checksum    string                     `json:"checksum"`
	Files       []InstalledSkillFile       `json:"files,omitempty"`
	Projections []InstalledSkillProjection `json:"projections,omitempty"`
	Metadata    map[string]string          `json:"metadata,omitempty"`
}

type InstalledSkillFile struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
	Mode     string `json:"mode,omitempty"`
}

type InstalledSkillProjection struct {
	IDE      IDE    `json:"ide"`
	Source   string `json:"source"`
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
	Mode     string `json:"mode,omitempty"`
	State    string `json:"state,omitempty"`
}

type InstalledSkillSourceValidation struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Valid       bool     `json:"valid"`
	Errors      []string `json:"errors,omitempty"`
	Files       int      `json:"files,omitempty"`
	Bytes       int64    `json:"bytes,omitempty"`
}

type SkillInstallResult struct {
	Skill       InstalledSkillManifest         `json:"skill"`
	Validation  InstalledSkillSourceValidation `json:"validation"`
	Projections []InstalledSkillProjection     `json:"projections,omitempty"`
}

type SkillListResult struct {
	Skills []InstalledSkillManifest `json:"skills"`
}

type SkillRemoveResult struct {
	Name                 string                     `json:"name"`
	RemovedSource        bool                       `json:"removed_source"`
	RemovedProjections   []InstalledSkillProjection `json:"removed_projections,omitempty"`
	PreservedProjections []InstalledSkillProjection `json:"preserved_projections,omitempty"`
}

func (m InstalledSkillManifest) Validate() error {
	if err := ValidateInstalledSkillName(m.Name); err != nil {
		return err
	}
	if strings.TrimSpace(m.SourcePath) == "" {
		return fmt.Errorf("installed skill source_path is required")
	}
	if strings.TrimSpace(m.Checksum) == "" {
		return fmt.Errorf("installed skill checksum is required")
	}
	if !strings.HasPrefix(m.Checksum, "sha256:") {
		return fmt.Errorf("installed skill checksum must use sha256:")
	}
	for i, file := range m.Files {
		if strings.TrimSpace(file.Path) == "" {
			return fmt.Errorf("installed skill files[%d].path is required", i)
		}
		if file.Checksum != "" && !strings.HasPrefix(file.Checksum, "sha256:") {
			return fmt.Errorf("installed skill files[%d].checksum must use sha256:", i)
		}
	}
	for i, projection := range m.Projections {
		if err := projection.Validate(); err != nil {
			return fmt.Errorf("installed skill projections[%d]: %w", i, err)
		}
	}
	return nil
}

func (p InstalledSkillProjection) Validate() error {
	if p.IDE == "" {
		return fmt.Errorf("ide is required")
	}
	if _, err := ParseIDE(p.IDE.String()); err != nil || p.IDE == IDEAll {
		return fmt.Errorf("ide %q is invalid", p.IDE)
	}
	if err := validateContractSlashRelative("source", p.Source); err != nil {
		return err
	}
	if err := validateContractSlashRelative("path", p.Path); err != nil {
		return err
	}
	if strings.TrimSpace(p.Checksum) == "" {
		return fmt.Errorf("checksum is required")
	}
	if !strings.HasPrefix(p.Checksum, "sha256:") {
		return fmt.Errorf("checksum must use sha256:")
	}
	if p.State != "" && !validProjectionState(p.State) {
		return fmt.Errorf("state %q is invalid", p.State)
	}
	return nil
}

func ValidateInstalledSkillName(name string) error {
	if name == "" {
		return fmt.Errorf("installed skill name is required")
	}
	for i, r := range name {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-'
		if !valid {
			return fmt.Errorf("installed skill name %q must be kebab-case", name)
		}
		if i == 0 && r == '-' {
			return fmt.Errorf("installed skill name %q must not start with hyphen", name)
		}
	}
	if strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return fmt.Errorf("installed skill name %q must be kebab-case", name)
	}
	return nil
}

func validProjectionState(state string) bool {
	switch state {
	case ProjectionStateOwned, ProjectionStateDiverged, ProjectionStateRemoved:
		return true
	default:
		return false
	}
}
