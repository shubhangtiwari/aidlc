package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

func TestRunValidateHighRiskEvidencePasses(t *testing.T) {
	root := t.TempDir()
	writeValidateSpec(t, root, "approved", []string{"aidlc/internal/commands/validate.go"})
	writeFile(t, filepath.Join(root, "aidlc", "internal", "commands", "validate.go"), "package commands\n")
	digest := digestForTest(t, root, []string{"aidlc/internal/commands/validate.go"})
	writeTaskRecord(t, root, contract.TaskRecordV1{
		SchemaVersion: contract.TaskRecordSchemaVersion,
		ID:            "task-val",
		Spec:          "docs/spec/1790867975-token-efficient-harness.md",
		Risk:          contract.TaskRiskHigh,
		ScopeRoot:     ".",
		BaseRevision:  "HEAD~1",
		OwnedFiles:    []string{"aidlc/internal/commands/validate.go"},
		ContentDigest: digest,
		Checks: []contract.TaskCheckRecord{{
			Command:       "make aidlc-test",
			Status:        contract.CheckStatusPassed,
			Revision:      "HEAD",
			ContentDigest: digest,
		}},
		Review: &contract.TaskReviewRecord{
			Kind:          contract.ReviewKindIndependent,
			Evidence:      "reviewer summary",
			Revision:      "HEAD",
			ContentDigest: digest,
		},
		OpenNextSteps: []string{},
	})

	result, err := RunValidate(context.Background(), ValidateOptions{
		Dir:          root,
		Spec:         "docs/spec/1790867975-token-efficient-harness.md",
		TaskRecord:   "docs/tasks/task-val.task.json",
		Base:         "HEAD~1",
		ChangedFiles: []string{"aidlc/internal/commands/validate.go"},
		HighRisk:     true,
	}, fakeValidateGit())
	if err != nil {
		t.Fatalf("RunValidate() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("RunValidate() passed = false, failures = %#v", result.Failures)
	}
	if result.ContentDigest != digest {
		t.Fatalf("content digest = %q, want %q", result.ContentDigest, digest)
	}
}

func TestRunValidateRejectsDraftSpecAndStaleEvidence(t *testing.T) {
	root := t.TempDir()
	writeValidateSpec(t, root, "draft", []string{"aidlc/internal/commands/validate.go"})
	writeFile(t, filepath.Join(root, "aidlc", "internal", "commands", "validate.go"), "package commands\n")
	writeTaskRecord(t, root, contract.TaskRecordV1{
		SchemaVersion: contract.TaskRecordSchemaVersion,
		ID:            "task-val",
		Spec:          "docs/spec/1790867975-token-efficient-harness.md",
		Risk:          contract.TaskRiskHigh,
		ScopeRoot:     ".",
		OwnedFiles:    []string{"aidlc/internal/commands/validate.go"},
		ContentDigest: "sha256:stale",
		Checks: []contract.TaskCheckRecord{{
			Command:       "make aidlc-test",
			Status:        contract.CheckStatusPassed,
			Revision:      "HEAD~2",
			ContentDigest: "sha256:stale",
		}},
		Review: &contract.TaskReviewRecord{
			Kind:          contract.ReviewKindIndependent,
			Evidence:      "reviewer summary",
			Revision:      "HEAD~2",
			ContentDigest: "sha256:stale",
		},
		OpenNextSteps: []string{},
	})

	result, err := RunValidate(context.Background(), ValidateOptions{
		Dir:          root,
		Spec:         "docs/spec/1790867975-token-efficient-harness.md",
		TaskRecord:   "docs/tasks/task-val.task.json",
		ChangedFiles: []string{"aidlc/internal/commands/validate.go"},
		HighRisk:     true,
	}, fakeValidateGit())
	if err != nil {
		t.Fatalf("RunValidate() error = %v", err)
	}
	if result.Passed {
		t.Fatal("RunValidate() passed = true, want false")
	}
	joined := strings.Join(result.Failures, "\n")
	for _, want := range []string{"status is \"draft\"", "content_digest is stale", "revision is stale"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("failures missing %q: %#v", want, result.Failures)
		}
	}
}

func TestRunValidateHighRiskUsesTaskRecordBaseRevisionForCommittedChanges(t *testing.T) {
	root := t.TempDir()
	writeValidateSpec(t, root, "approved", []string{"aidlc/internal/commands/validate.go"})
	writeFile(t, filepath.Join(root, "aidlc", "internal", "commands", "validate.go"), "package commands\n")
	writeFile(t, filepath.Join(root, "aidlc", "internal", "commands", "other.go"), "package commands\n")
	digest := digestForTest(t, root, []string{"aidlc/internal/commands/validate.go"})
	writeTaskRecord(t, root, contract.TaskRecordV1{
		SchemaVersion: contract.TaskRecordSchemaVersion,
		ID:            "task-val",
		Spec:          "docs/spec/1790867975-token-efficient-harness.md",
		Risk:          contract.TaskRiskHigh,
		ScopeRoot:     ".",
		BaseRevision:  "HEAD~1",
		OwnedFiles:    []string{"aidlc/internal/commands/validate.go"},
		ContentDigest: digest,
		Checks: []contract.TaskCheckRecord{{
			Command:       "make aidlc-test",
			Status:        contract.CheckStatusPassed,
			Revision:      "HEAD",
			ContentDigest: digest,
		}},
		Review: &contract.TaskReviewRecord{
			Kind:          contract.ReviewKindIndependent,
			Evidence:      "reviewer summary",
			Revision:      "HEAD",
			ContentDigest: digest,
		},
		OpenNextSteps: []string{},
	})

	result, err := RunValidate(context.Background(), ValidateOptions{
		Dir:        root,
		Spec:       "docs/spec/1790867975-token-efficient-harness.md",
		TaskRecord: "docs/tasks/task-val.task.json",
		HighRisk:   true,
	}, fakeValidateGitWithDiff("aidlc/internal/commands/other.go"))
	if err != nil {
		t.Fatalf("RunValidate() error = %v", err)
	}
	if result.Passed {
		t.Fatal("RunValidate() passed = true, want false")
	}
	joined := strings.Join(result.Failures, "\n")
	for _, want := range []string{
		"changed file aidlc/internal/commands/other.go is not listed in the approved spec",
		"changed file aidlc/internal/commands/other.go is not covered by task record owned_files",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("failures missing %q: %#v", want, result.Failures)
		}
	}
}

func TestRunValidateAllowsGoverningSpecInDiff(t *testing.T) {
	root := t.TempDir()
	writeValidateSpec(t, root, "approved", []string{"aidlc/internal/commands/validate.go"})
	writeFile(t, filepath.Join(root, "aidlc", "internal", "commands", "validate.go"), "package commands\n")
	digest := digestForTest(t, root, []string{"aidlc/internal/commands/validate.go"})
	writeTaskRecord(t, root, contract.TaskRecordV1{
		SchemaVersion: contract.TaskRecordSchemaVersion,
		ID:            "task-val",
		Spec:          "docs/spec/1790867975-token-efficient-harness.md",
		Risk:          contract.TaskRiskHigh,
		ScopeRoot:     ".",
		BaseRevision:  "HEAD~1",
		OwnedFiles:    []string{"aidlc/internal/commands/validate.go"},
		ContentDigest: digest,
		Checks: []contract.TaskCheckRecord{{
			Command:       "make aidlc-test",
			Status:        contract.CheckStatusPassed,
			Revision:      "HEAD",
			ContentDigest: digest,
		}},
		Review: &contract.TaskReviewRecord{
			Kind:          contract.ReviewKindIndependent,
			Evidence:      "reviewer summary",
			Revision:      "HEAD",
			ContentDigest: digest,
		},
		OpenNextSteps: []string{},
	})

	result, err := RunValidate(context.Background(), ValidateOptions{
		Dir:        root,
		Spec:       "docs/spec/1790867975-token-efficient-harness.md",
		TaskRecord: "docs/tasks/task-val.task.json",
		HighRisk:   true,
	}, fakeValidateGitWithDiff(
		"docs/spec/1790867975-token-efficient-harness.md",
		"aidlc/internal/commands/validate.go",
	))
	if err != nil {
		t.Fatalf("RunValidate() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("RunValidate() passed = false, failures = %#v", result.Failures)
	}
}

func TestRunValidateRejectsScopeEscapeAndUnownedChange(t *testing.T) {
	root := t.TempDir()
	writeValidateSpec(t, root, "approved", []string{"aidlc/internal/commands/validate.go"})
	writeFile(t, filepath.Join(root, "nested", ".ai", "README.md"), "nested\n")
	writeFile(t, filepath.Join(root, "nested", "docs", "spec", "README.md"), "nested\n")
	writeFile(t, filepath.Join(root, "nested", "owned.go"), "package nested\n")

	result, err := RunValidate(context.Background(), ValidateOptions{
		Dir:          root,
		Spec:         "docs/spec/1790867975-token-efficient-harness.md",
		ChangedFiles: []string{"nested/owned.go"},
	}, fakeValidateGit())
	if err != nil {
		t.Fatalf("RunValidate() error = %v", err)
	}
	if result.Passed {
		t.Fatal("RunValidate() passed = true, want false")
	}
	joined := strings.Join(result.Failures, "\n")
	for _, want := range []string{"nested AIDLC scope", "not listed in the approved spec"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("failures missing %q: %#v", want, result.Failures)
		}
	}
}

func TestRunValidateCLIJSONFailureExitCode(t *testing.T) {
	root := t.TempDir()
	writeValidateSpec(t, root, "draft", []string{"aidlc/internal/commands/validate.go"})

	var stdout, stderr bytes.Buffer
	code := RunValidateCLI(context.Background(), []string{
		"--dir", root,
		"--spec", "docs/spec/1790867975-token-efficient-harness.md",
		"--format", "json",
	}, &stdout, &stderr, fakeValidateGit())
	if code != contract.ExitConflict {
		t.Fatalf("code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"passed": false`) {
		t.Fatalf("stdout missing JSON failure:\n%s", stdout.String())
	}
}

func writeValidateSpec(t testing.TB, root, status string, affected []string) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "---\nstatus: %s\n---\n\n## Affected files\n\n", status)
	for _, path := range affected {
		fmt.Fprintf(&b, "- `%s`\n", path)
	}
	writeFile(t, filepath.Join(root, "docs", "spec", "1790867975-token-efficient-harness.md"), b.String())
}

func writeTaskRecord(t testing.TB, root string, record contract.TaskRecordV1) {
	t.Helper()
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal task record: %v", err)
	}
	writeFile(t, filepath.Join(root, "docs", "tasks", "task-val.task.json"), string(data))
}

func digestForTest(t testing.TB, root string, files []string) string {
	t.Helper()
	digest, err := digestOwnedFiles(root, files)
	if err != nil {
		t.Fatalf("digestOwnedFiles() error = %v", err)
	}
	return digest
}

func fakeValidateGit() ValidateDependencies {
	return fakeValidateGitWithDiff()
}

func fakeValidateGitWithDiff(diffPaths ...string) ValidateDependencies {
	revisions := map[string]string{
		"HEAD":   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"HEAD~1": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"HEAD~2": "cccccccccccccccccccccccccccccccccccccccc",
	}
	return ValidateDependencies{RunGit: func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "rev-parse" && args[1] == "--verify" {
			rev := strings.TrimSuffix(args[2], "^{commit}")
			if resolved, ok := revisions[rev]; ok {
				return []byte(resolved + "\n"), nil
			}
			return nil, fmt.Errorf("unknown revision %s", rev)
		}
		if len(args) > 0 && args[0] == "status" {
			return nil, nil
		}
		if len(args) > 0 && args[0] == "diff" {
			if len(diffPaths) == 0 {
				return nil, nil
			}
			return []byte(strings.Join(diffPaths, "\n") + "\n"), nil
		}
		return nil, fmt.Errorf("unexpected git args: %v", args)
	}}
}

func TestCleanRepoPathRejectsBackslashes(t *testing.T) {
	if _, err := cleanRepoPath("path", `docs\spec.md`); err == nil {
		t.Fatal("cleanRepoPath() error = nil for backslash path")
	}
}

func TestEnsurePathInsideRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := ensurePathInside(root, "escape"); err == nil {
		t.Fatal("ensurePathInside() error = nil for symlink")
	}
}
