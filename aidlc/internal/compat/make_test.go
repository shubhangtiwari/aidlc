package compat

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/cli"
)

func TestBashInitAndNativeInitGenerateMatchingCodexOutputs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bash compatibility entrypoint is not part of the normal Windows CLI flow")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not available: %v", err)
	}

	repo := repoRoot(t)
	temp := t.TempDir()
	bashTarget := filepath.Join(temp, "bash", "consumer")
	nativeTarget := filepath.Join(temp, "native", "consumer")
	if err := os.MkdirAll(bashTarget, 0o755); err != nil {
		t.Fatalf("create bash target: %v", err)
	}
	if err := os.MkdirAll(nativeTarget, 0o755); err != nil {
		t.Fatalf("create native target: %v", err)
	}
	copyDir(t, filepath.Join(repo, ".ai"), filepath.Join(bashTarget, ".ai"))

	run(t, repo, "bash", filepath.Join(repo, ".ai/scripts/ai_init.sh"), "--repo", bashTarget, "codex")

	var stdout, stderr bytes.Buffer
	withWorkingDir(t, nativeTarget, func() {
		code := cli.Run(context.Background(), []string{
			"init",
			"--source", "local",
			"--path", repo,
			"--ref", "compat-test",
			"codex",
		}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("native init exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
		}
	})

	assertSameFile(t, bashTarget, nativeTarget, "AGENTS.md")
	assertSameTree(t, bashTarget, nativeTarget, ".codex")

	assertMissing(t, nativeTarget, "docs/ARCHITECTURE.md")
	assertMissing(t, nativeTarget, "docs/architecture/software.md")
	assertMissing(t, nativeTarget, "docs/spec/1780346463-add-aidlc-cli.md")
	assertMissing(t, nativeTarget, "docs/adr/1780346463-aidlc-cli-distribution-and-sync.md")
	assertMissing(t, nativeTarget, "docs/blueprints/aidlc.md")
	assertMissing(t, nativeTarget, "aidlc/internal/compat/make_test.go")
}

func TestBashInitRecordsCodexInRootLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bash compatibility entrypoint is not part of the normal Windows CLI flow")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not available: %v", err)
	}

	repo := repoRoot(t)
	target := bashTargetWithAI(t, repo)

	run(t, repo, "bash", filepath.Join(repo, ".ai/scripts/ai_init.sh"), "--repo", target, "codex")

	assertWorkspaceIDEs(t, target, []string{"codex"})
	assertMissing(t, target, ".aidlc/manifest.json")
}

func TestBashInitAllRecordsConcreteIDEsInRootLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bash compatibility entrypoint is not part of the normal Windows CLI flow")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not available: %v", err)
	}

	repo := repoRoot(t)
	target := bashTargetWithAI(t, repo)

	run(t, repo, "bash", filepath.Join(repo, ".ai/scripts/ai_init.sh"), "--repo", target, "all")

	assertWorkspaceIDEs(t, target, []string{"claude", "codex", "cursor", "copilot", "windsurf"})
	assertMissing(t, target, ".aidlc/manifest.json")
}

func TestBashInitPreservesRootLockAndSeedsFromLegacy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bash compatibility entrypoint is not part of the normal Windows CLI flow")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not available: %v", err)
	}

	repo := repoRoot(t)

	rootTarget := bashTargetWithAI(t, repo)
	writeJSON(t, rootTarget, "aidlc.lock.json", map[string]any{
		"schema_version": 1,
		"upstream": map[string]any{
			"source": "local",
			"ref":    "root-ref",
			"commit": "root-commit",
		},
		"workspace": map[string]any{
			"ides": []any{"cursor"},
		},
		"generated": map[string]any{
			"ide":     "cursor",
			"version": "root-version",
		},
		"files": []any{
			map[string]any{
				"path":     ".ai/README.md",
				"checksum": "sha256:root",
				"mode":     "0644",
			},
		},
		"metadata": map[string]any{
			"source_kind": "local",
			"source_path": "/root/source",
		},
	})

	run(t, repo, "bash", filepath.Join(repo, ".ai/scripts/ai_init.sh"), "--repo", rootTarget, "codex")

	rootLock := readLock(t, rootTarget)
	assertWorkspaceIDEsFromLock(t, rootLock, []string{"codex"})
	assertNestedString(t, rootLock, "upstream", "ref", "root-ref")
	assertNestedString(t, rootLock, "generated", "version", "root-version")
	assertNestedString(t, rootLock, "metadata", "source_path", "/root/source")
	files, ok := rootLock["files"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("root lock files = %#v, want one preserved file", rootLock["files"])
	}

	legacyTarget := bashTargetWithAI(t, repo)
	writeJSON(t, legacyTarget, ".aidlc/manifest.json", map[string]any{
		"schema_version": 1,
		"upstream": map[string]any{
			"source": "local",
			"ref":    "legacy-ref",
			"commit": "legacy-commit",
		},
		"generated": map[string]any{
			"ide": "cursor",
		},
		"files": []any{
			map[string]any{
				"path":     ".ai/README.md",
				"checksum": "sha256:legacy",
			},
		},
	})

	run(t, repo, "bash", filepath.Join(repo, ".ai/scripts/ai_init.sh"), "--repo", legacyTarget, "codex")

	legacySeededLock := readLock(t, legacyTarget)
	assertWorkspaceIDEsFromLock(t, legacySeededLock, []string{"codex"})
	assertNestedString(t, legacySeededLock, "upstream", "ref", "legacy-ref")
	assertNestedString(t, legacySeededLock, "generated", "ide", "cursor")
}

func TestBashUpdateAndNativeUpdateSurfacesRemainAvailable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bash compatibility entrypoint is not part of the normal Windows CLI flow")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not available: %v", err)
	}

	repo := repoRoot(t)
	output := run(t, repo, "bash", filepath.Join(repo, ".ai/scripts/ai_update.sh"), "--help")
	if !strings.Contains(output, "usage: ai_update.sh") {
		t.Fatalf("bash update help missing usage:\n%s", output)
	}
	if !strings.Contains(output, "Updates the local .ai/ directory from the upstream AIDLC repository.") {
		t.Fatalf("bash update help changed user-facing contract:\n%s", output)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), []string{"update", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("native update help exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: aidlc update [flags]") {
		t.Fatalf("native update help missing usage:\n%s", stdout.String())
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "../../.."))
	if _, err := os.Stat(filepath.Join(root, ".ai/scripts/ai_init.sh")); err != nil {
		t.Fatalf("resolve repository root from %s: %v", wd, err)
	}
	return root
}

func bashTargetWithAI(t *testing.T, repo string) string {
	t.Helper()

	target := filepath.Join(t.TempDir(), "consumer")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("create bash target: %v", err)
	}
	copyDir(t, filepath.Join(repo, ".ai"), filepath.Join(target, ".ai"))
	return target
}

func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func writeJSON(t *testing.T, root, name string, value any) {
	t.Helper()

	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	content = append(content, '\n')
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", name, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func readLock(t *testing.T, root string) map[string]any {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, "aidlc.lock.json"))
	if err != nil {
		t.Fatalf("read aidlc.lock.json: %v", err)
	}
	var lock map[string]any
	if err := json.Unmarshal(content, &lock); err != nil {
		t.Fatalf("parse aidlc.lock.json: %v", err)
	}
	return lock
}

func assertWorkspaceIDEs(t *testing.T, root string, want []string) {
	t.Helper()

	assertWorkspaceIDEsFromLock(t, readLock(t, root), want)
}

func assertWorkspaceIDEsFromLock(t *testing.T, lock map[string]any, want []string) {
	t.Helper()

	workspace, ok := lock["workspace"].(map[string]any)
	if !ok {
		t.Fatalf("workspace = %#v, want object", lock["workspace"])
	}
	rawIDEs, ok := workspace["ides"].([]any)
	if !ok {
		t.Fatalf("workspace.ides = %#v, want array", workspace["ides"])
	}
	if len(rawIDEs) != len(want) {
		t.Fatalf("workspace.ides = %#v, want %#v", rawIDEs, want)
	}
	for i, raw := range rawIDEs {
		got, ok := raw.(string)
		if !ok {
			t.Fatalf("workspace.ides[%d] = %#v, want string", i, raw)
		}
		if got != want[i] {
			t.Fatalf("workspace.ides = %#v, want %#v", rawIDEs, want)
		}
	}
}

func assertNestedString(t *testing.T, lock map[string]any, parent, key, want string) {
	t.Helper()

	obj, ok := lock[parent].(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", parent, lock[parent])
	}
	got, ok := obj[key].(string)
	if !ok || got != want {
		t.Fatalf("%s.%s = %#v, want %q", parent, key, obj[key], want)
	}
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to %s: %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore working directory to %s: %v", previous, err)
		}
	}()

	fn()
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()

	err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, content, info.Mode().Perm())
	})
	if err != nil {
		t.Fatalf("copy %s to %s: %v", src, dst, err)
	}
}

func assertSameTree(t *testing.T, leftRoot, rightRoot, name string) {
	t.Helper()

	leftFiles := treeFiles(t, filepath.Join(leftRoot, filepath.FromSlash(name)))
	rightFiles := treeFiles(t, filepath.Join(rightRoot, filepath.FromSlash(name)))
	if strings.Join(leftFiles, "\n") != strings.Join(rightFiles, "\n") {
		t.Fatalf("%s file list differs\nbash:\n%s\nnative:\n%s", name, strings.Join(leftFiles, "\n"), strings.Join(rightFiles, "\n"))
	}
	for _, file := range leftFiles {
		assertSameFile(t, filepath.Join(leftRoot, filepath.FromSlash(name)), filepath.Join(rightRoot, filepath.FromSlash(name)), file)
	}
}

func treeFiles(t *testing.T, root string) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

func assertSameFile(t *testing.T, leftRoot, rightRoot, name string) {
	t.Helper()

	left, err := os.ReadFile(filepath.Join(leftRoot, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read bash %s: %v", name, err)
	}
	right, err := os.ReadFile(filepath.Join(rightRoot, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read native %s: %v", name, err)
	}
	if !bytes.Equal(left, right) {
		t.Fatalf("%s differs between bash and native output", name)
	}
}

func assertMissing(t *testing.T, root, name string) {
	t.Helper()

	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
	if err == nil {
		t.Fatalf("%s should not have been copied", name)
	}
	if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", name, err)
	}
}
