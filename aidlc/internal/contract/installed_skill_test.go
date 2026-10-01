package contract

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInstalledSkillManifestJSONShape(t *testing.T) {
	record := InstalledSkillManifest{
		Name:        "local-helper",
		Description: "Local helper skill",
		SourcePath:  ".ai/skills/installed/local-helper",
		Checksum:    "sha256:source",
		Files: []InstalledSkillFile{{
			Path:     "SKILL.md",
			Checksum: "sha256:file",
			Mode:     "0644",
		}},
		Projections: []InstalledSkillProjection{{
			IDE:      IDECodex,
			Source:   ".ai/skills/installed/local-helper/SKILL.md",
			Path:     ".codex/skills/local-helper/SKILL.md",
			Checksum: "sha256:projection",
			Mode:     "0644",
			State:    ProjectionStateOwned,
		}},
	}

	got, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `{"name":"local-helper","description":"Local helper skill","source_path":".ai/skills/installed/local-helper","checksum":"sha256:source","files":[{"path":"SKILL.md","checksum":"sha256:file","mode":"0644"}],"projections":[{"ide":"codex","source":".ai/skills/installed/local-helper/SKILL.md","path":".codex/skills/local-helper/SKILL.md","checksum":"sha256:projection","mode":"0644","state":"owned"}]}`
	if string(got) != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestInstalledSkillResultShapes(t *testing.T) {
	manifest := InstalledSkillManifest{
		Name:       "local-helper",
		SourcePath: ".ai/skills/installed/local-helper",
		Checksum:   "sha256:source",
	}
	projection := InstalledSkillProjection{
		IDE:      IDECursor,
		Source:   ".ai/skills/installed/local-helper/SKILL.md",
		Path:     ".cursor/skills/local-helper/SKILL.md",
		Checksum: "sha256:projection",
		State:    ProjectionStateDiverged,
	}
	install := SkillInstallResult{
		Skill: manifest,
		Validation: InstalledSkillSourceValidation{
			Name:        "local-helper",
			Description: "Local helper skill",
			Valid:       true,
			Files:       1,
			Bytes:       120,
		},
		Projections: []InstalledSkillProjection{projection},
	}
	list := SkillListResult{Skills: []InstalledSkillManifest{manifest}}
	remove := SkillRemoveResult{
		Name:                 "local-helper",
		RemovedSource:        true,
		PreservedProjections: []InstalledSkillProjection{projection},
	}
	for name, value := range map[string]any{"install": install, "list": list, "remove": remove} {
		if _, err := json.Marshal(value); err != nil {
			t.Fatalf("%s Marshal() error = %v", name, err)
		}
	}
}

func TestInstalledSkillValidation(t *testing.T) {
	tests := []struct {
		name   string
		record InstalledSkillManifest
		want   string
	}{
		{
			name:   "skill name",
			record: InstalledSkillManifest{Name: "Local_Helper", SourcePath: ".ai/skills/installed/local-helper", Checksum: "sha256:source"},
			want:   "kebab-case",
		},
		{
			name:   "checksum",
			record: InstalledSkillManifest{Name: "local-helper", SourcePath: ".ai/skills/installed/local-helper", Checksum: "source"},
			want:   "sha256:",
		},
		{
			name: "projection ide",
			record: InstalledSkillManifest{
				Name:       "local-helper",
				SourcePath: ".ai/skills/installed/local-helper",
				Checksum:   "sha256:source",
				Projections: []InstalledSkillProjection{{
					IDE:      IDEAll,
					Source:   ".ai/skills/installed/local-helper/SKILL.md",
					Path:     ".codex/skills/local-helper/SKILL.md",
					Checksum: "sha256:projection",
				}},
			},
			want: "invalid",
		},
		{
			name: "projection state",
			record: InstalledSkillManifest{
				Name:       "local-helper",
				SourcePath: ".ai/skills/installed/local-helper",
				Checksum:   "sha256:source",
				Projections: []InstalledSkillProjection{{
					IDE:      IDECodex,
					Source:   ".ai/skills/installed/local-helper/SKILL.md",
					Path:     ".codex/skills/local-helper/SKILL.md",
					Checksum: "sha256:projection",
					State:    "changed",
				}},
			},
			want: "state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.record.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %q, want containing %q", err, tt.want)
			}
		})
	}
}
