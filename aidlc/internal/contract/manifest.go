package contract

import (
	"fmt"
	"path"
	"strings"
)

const (
	TargetManifestPath       = "aidlc.lock.json"
	LegacyTargetManifestPath = ".aidlc/manifest.json"
	TargetManifestVersion    = 1
	TemplateManifestPath     = ".ai/template-manifest.yaml"
	TemplateManifestV1       = 1
)

type TargetManifest struct {
	SchemaVersion int               `json:"schema_version"`
	Upstream      UpstreamRef       `json:"upstream"`
	Workspace     WorkspaceRecord   `json:"workspace"`
	Generated     GenerationRecord  `json:"generated"`
	Files         []ManifestFile    `json:"files"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type WorkspaceRecord struct {
	IDEs []IDE              `json:"ides,omitempty"`
	Map  MapWorkspaceRecord `json:"map,omitempty"`
}

type MapWorkspaceRecord struct {
	Include []string `json:"include,omitempty"`
}

type UpstreamRef struct {
	Source string `json:"source"`
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}

type GenerationRecord struct {
	IDE         IDE                   `json:"ide"`
	Version     string                `json:"version,omitempty"`
	Timestamp   string                `json:"timestamp,omitempty"`
	Projections []GeneratedProjection `json:"projections,omitempty"`
	Metadata    map[string]string     `json:"metadata,omitempty"`
}

type GeneratedProjection struct {
	IDE      IDE    `json:"ide"`
	Source   string `json:"source"`
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
	Mode     string `json:"mode,omitempty"`
}

func (p GeneratedProjection) Validate() error {
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
	return nil
}

func validateContractSlashRelative(kind, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", kind)
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, ":") {
		return fmt.Errorf("%s %q must be slash-relative", kind, value)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("%s %q must not contain empty, current, or parent path segments", kind, value)
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("%s %q must not traverse parents", kind, value)
	}
	return nil
}

type ManifestFile struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
	Mode     string `json:"mode,omitempty"`
}

type TemplateManifest struct {
	SchemaVersion int                    `yaml:"schema_version"`
	Payload       TemplatePayload        `yaml:"payload"`
	Policy        TemplateManifestPolicy `yaml:"policy"`
}

type TemplatePayload struct {
	Include         []string                 `yaml:"include"`
	IncludeMappings []TemplatePayloadMapping `yaml:"-"`
	Exclude         []string                 `yaml:"exclude"`
}

type TemplatePayloadMapping struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

type TemplateManifestPolicy struct {
	AllowBroadDirectories    bool `yaml:"allow_broad_directories"`
	PublicDocsMustBeExplicit bool `yaml:"public_docs_must_be_explicit"`
	RejectAbsolutePaths      bool `yaml:"reject_absolute_paths"`
	RejectParentTraversal    bool `yaml:"reject_parent_traversal"`
}
