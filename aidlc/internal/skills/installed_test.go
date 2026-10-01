package skills_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/skills"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
)

func TestInstallCopiesValidatedTreeAndListReadsMetadata(t *testing.T) {
	target := t.TempDir()
	source := createSkillSource(t, "local-tool")
	testutil.WriteFile(t, source, "scripts/run.sh", "#!/bin/sh\nexit 0\n")
	testutil.WriteFile(t, source, "assets/prompt.txt", "prompt\n")

	result, err := skills.Install(skills.InstallOptions{TargetDir: target, SourceDir: source})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if result.Skill.Name != "local-tool" || !result.Validation.Valid {
		t.Fatalf("result = %#v", result)
	}
	assertFile(t, target, ".ai/skills/installed/local-tool/SKILL.md", "---\nname: local-tool\ndescription: Local tool skill\n---\n\nUse the local tool.\n")
	assertFile(t, target, ".ai/skills/installed/local-tool/scripts/run.sh", "#!/bin/sh\nexit 0\n")
	assertFile(t, target, ".ai/skills/installed/local-tool/assets/prompt.txt", "prompt\n")

	list, err := skills.List(skills.ListOptions{TargetDir: target})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Skills) != 1 || list.Skills[0].Name != "local-tool" {
		t.Fatalf("list = %#v", list)
	}
}

func TestInstallRejectsBundledAndInstalledCollisions(t *testing.T) {
	target := t.TempDir()
	testutil.WriteFile(t, target, ".ai/skills/local-tool.md", "bundled\n")
	source := createSkillSource(t, "local-tool")
	if _, err := skills.Install(skills.InstallOptions{TargetDir: target, SourceDir: source}); err == nil || !strings.Contains(err.Error(), "collides with bundled skill") {
		t.Fatalf("install bundled collision err = %v", err)
	}

	target = t.TempDir()
	if _, err := skills.Install(skills.InstallOptions{TargetDir: target, SourceDir: source}); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := skills.Install(skills.InstallOptions{TargetDir: target, SourceDir: source}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("reinstall err = %v", err)
	}
}

func TestValidateRejectsTraversalSymlinkAndHiddenVCS(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		source := createSkillSource(t, "local-tool")
		if err := os.Symlink("SKILL.md", filepath.Join(source, "link.md")); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		if _, _, err := skills.ValidateSourceTree(source); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("validate err = %v", err)
		}
	})
	t.Run("hidden vcs", func(t *testing.T) {
		source := createSkillSource(t, "local-tool")
		testutil.WriteFile(t, source, ".git/config", "metadata\n")
		if _, _, err := skills.ValidateSourceTree(source); err == nil || !strings.Contains(err.Error(), "hidden VCS") {
			t.Fatalf("validate err = %v", err)
		}
	})
}

func TestInstallRejectsSymlinkedDestinationAncestors(t *testing.T) {
	target := t.TempDir()
	source := createSkillSource(t, "local-tool")
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(target, ".ai"), 0o755); err != nil {
		t.Fatalf("mkdir .ai: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(target, ".ai", "skills")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := skills.Install(skills.InstallOptions{TargetDir: target, SourceDir: source}); err == nil || !strings.Contains(err.Error(), "symlink ancestor") {
		t.Fatalf("install err = %v", err)
	}
}

func TestRemoveDeletesOnlyOwnedUnmodifiedProjectionAndReportsDiverged(t *testing.T) {
	target := t.TempDir()
	source := createSkillSource(t, "local-tool")
	if _, err := skills.Install(skills.InstallOptions{TargetDir: target, SourceDir: source}); err != nil {
		t.Fatalf("install: %v", err)
	}
	cleanContent := []byte("generated skill\n")
	divergedContent := []byte("edited skill\n")
	testutil.WriteFile(t, target, ".codex/skills/local-tool/SKILL.md", string(cleanContent))
	testutil.WriteFile(t, target, ".cursor/skills/local-tool/SKILL.md", string(divergedContent))
	testutil.WriteFile(t, target, ".codex/agents/local-tool.toml", "must not delete\n")
	manifest := contract.TargetManifest{
		Generated: contract.GenerationRecord{
			IDE: contract.IDEAll,
			Projections: []contract.GeneratedProjection{
				{
					IDE:      contract.IDECodex,
					Source:   ".ai/skills/installed/local-tool",
					Path:     ".codex/skills/local-tool/SKILL.md",
					Checksum: templatesync.BytesChecksum(cleanContent),
					Mode:     "0644",
				},
				{
					IDE:      contract.IDECursor,
					Source:   ".ai/skills/installed/local-tool",
					Path:     ".cursor/skills/local-tool/SKILL.md",
					Checksum: templatesync.BytesChecksum([]byte("old generated\n")),
					Mode:     "0644",
				},
				{
					IDE:      contract.IDECodex,
					Source:   ".ai/skills/installed/local-tool",
					Path:     ".codex/agents/local-tool.toml",
					Checksum: templatesync.BytesChecksum([]byte("must not delete\n")),
					Mode:     "0644",
				},
			},
		},
	}
	if err := templatesync.WriteManifest(target, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err := skills.Remove(skills.RemoveOptions{TargetDir: target, Name: "local-tool"})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !result.RemovedSource {
		t.Fatal("source was not removed")
	}
	if len(result.RemovedProjections) != 1 || result.RemovedProjections[0].Path != ".codex/skills/local-tool/SKILL.md" {
		t.Fatalf("removed projections = %#v", result.RemovedProjections)
	}
	if len(result.PreservedProjections) != 2 {
		t.Fatalf("preserved projections = %#v", result.PreservedProjections)
	}
	assertMissing(t, target, ".ai/skills/installed/local-tool/SKILL.md")
	assertMissing(t, target, ".codex/skills/local-tool/SKILL.md")
	assertFile(t, target, ".cursor/skills/local-tool/SKILL.md", string(divergedContent))
	assertFile(t, target, ".codex/agents/local-tool.toml", "must not delete\n")
	read, err := templatesync.ReadManifest(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if len(read.Generated.Projections) != 2 {
		t.Fatalf("remaining projections = %#v", read.Generated.Projections)
	}
}

func TestRemoveMalformedLockPreservesInstalledSource(t *testing.T) {
	target := t.TempDir()
	source := createSkillSource(t, "local-tool")
	testutil.WriteFile(t, source, "assets/prompt.txt", "prompt\n")
	if _, err := skills.Install(skills.InstallOptions{TargetDir: target, SourceDir: source}); err != nil {
		t.Fatalf("install: %v", err)
	}
	testutil.WriteFile(t, target, contract.TargetManifestPath, "{not json")

	result, err := skills.Remove(skills.RemoveOptions{TargetDir: target, Name: "local-tool"})
	if err == nil {
		t.Fatal("remove succeeded with malformed lock")
	}
	if result.RemovedSource {
		t.Fatalf("source reported removed despite malformed lock: %#v", result)
	}
	assertFile(t, target, ".ai/skills/installed/local-tool/SKILL.md", "---\nname: local-tool\ndescription: Local tool skill\n---\n\nUse the local tool.\n")
	assertFile(t, target, ".ai/skills/installed/local-tool/assets/prompt.txt", "prompt\n")
}

func createSkillSource(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	testutil.WriteFile(t, root, "SKILL.md", "---\nname: "+name+"\ndescription: Local tool skill\n---\n\nUse the local tool.\n")
	return root
}

func assertFile(t *testing.T, root, name, want string) {
	t.Helper()
	got := testutil.ReadFile(t, root, name)
	if got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}

func assertMissing(t *testing.T, root, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); !os.IsNotExist(err) {
		t.Fatalf("%s exists or stat failed unexpectedly: %v", name, err)
	}
}
