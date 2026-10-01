package payload_test

import (
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/payload"
)

func TestInstalledSkillSourcePathIsPrivateCaseInsensitively(t *testing.T) {
	for _, name := range []string{
		".ai/skills/installed",
		".ai/skills/installed/local-tool/SKILL.md",
		".AI/SKILLS/INSTALLED/local-tool/SKILL.md",
	} {
		t.Run(name, func(t *testing.T) {
			if !payload.IsInstalledSkillSourcePath(name) {
				t.Fatalf("%s was not recognized as installed skill source path", name)
			}
			if !payload.IsPrivatePath(name) {
				t.Fatalf("%s was not private", name)
			}
		})
	}
	if payload.IsInstalledSkillSourcePath(".ai/skills/classify-change.md") {
		t.Fatal("bundled skill path was treated as installed source path")
	}
}
