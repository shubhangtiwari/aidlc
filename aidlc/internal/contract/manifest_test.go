package contract

import (
	"encoding/json"
	"testing"
)

func TestTargetManifestGeneratedProjectionJSONShape(t *testing.T) {
	manifest := TargetManifest{
		SchemaVersion: TargetManifestVersion,
		Upstream: UpstreamRef{
			Source: "local",
			Ref:    "main",
			Commit: "HEAD",
		},
		Workspace: WorkspaceRecord{
			IDEs: []IDE{IDECodex},
		},
		Generated: GenerationRecord{
			IDE:     IDECodex,
			Version: "v1.0.0",
			Projections: []GeneratedProjection{{
				IDE:      IDECodex,
				Source:   ".ai/skills/installed/local-helper/SKILL.md",
				Path:     ".codex/skills/local-helper/SKILL.md",
				Checksum: "sha256:projection",
				Mode:     "0644",
			}},
		},
	}

	got, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `{"schema_version":1,"upstream":{"source":"local","ref":"main","commit":"HEAD"},"workspace":{"ides":["codex"],"map":{}},"generated":{"ide":"codex","version":"v1.0.0","projections":[{"ide":"codex","source":".ai/skills/installed/local-helper/SKILL.md","path":".codex/skills/local-helper/SKILL.md","checksum":"sha256:projection","mode":"0644"}]},"files":null}`
	if string(got) != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
}

func TestGeneratedProjectionValidate(t *testing.T) {
	valid := GeneratedProjection{
		IDE:      IDECodex,
		Source:   ".ai/skills/installed/local-helper/SKILL.md",
		Path:     ".codex/skills/local-helper/SKILL.md",
		Checksum: "sha256:projection",
		Mode:     "0644",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	tests := []GeneratedProjection{
		{IDE: IDEAll, Source: valid.Source, Path: valid.Path, Checksum: valid.Checksum},
		{IDE: IDECodex, Source: "../SKILL.md", Path: valid.Path, Checksum: valid.Checksum},
		{IDE: IDECodex, Source: valid.Source, Path: "/tmp/SKILL.md", Checksum: valid.Checksum},
		{IDE: IDECodex, Source: valid.Source, Path: valid.Path, Checksum: "projection"},
	}
	for _, tt := range tests {
		if err := tt.Validate(); err == nil {
			t.Fatalf("Validate() error = nil for %#v", tt)
		}
	}
}
