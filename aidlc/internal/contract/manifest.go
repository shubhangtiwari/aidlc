package contract

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
	IDEs []IDE `json:"ides,omitempty"`
}

type UpstreamRef struct {
	Source string `json:"source"`
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}

type GenerationRecord struct {
	IDE       IDE               `json:"ide"`
	Version   string            `json:"version,omitempty"`
	Timestamp string            `json:"timestamp,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
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
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

type TemplateManifestPolicy struct {
	AllowBroadDirectories    bool `yaml:"allow_broad_directories"`
	PublicDocsMustBeExplicit bool `yaml:"public_docs_must_be_explicit"`
	RejectAbsolutePaths      bool `yaml:"reject_absolute_paths"`
	RejectParentTraversal    bool `yaml:"reject_parent_traversal"`
}
