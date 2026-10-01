package search

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

func TestExactSearcherUnavailableIsDiagnosticOnly(t *testing.T) {
	t.Parallel()

	response, err := (&ExactSearcher{RgPath: filepath.Join(t.TempDir(), "missing-rg")}).SearchExact(context.Background(), model.ExactSearchRequest{
		Root:       t.TempDir(),
		Literals:   []string{"Needle("},
		MaxResults: 5,
		MaxBytes:   1024,
		TimeoutMS:  100,
	})
	if err != nil {
		t.Fatalf("SearchExact() error = %v", err)
	}
	if len(response.Results) != 0 {
		t.Fatalf("Results = %#v, want none", response.Results)
	}
	if len(response.Diagnostics) == 0 || !strings.Contains(response.Diagnostics[0], "rg unavailable") {
		t.Fatalf("Diagnostics = %#v, want rg unavailable", response.Diagnostics)
	}
}

func TestExactSearcherUsesFixedStringAndPathHints(t *testing.T) {
	t.Parallel()
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		t.Skip("rg not installed")
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "internal/auth/service.go"), "package auth\nfunc Needle(value string) string { return value }\n")
	writeFile(t, filepath.Join(root, "internal/other/service.go"), "package other\nfunc Needle(value string) string { return value }\n")

	response, err := (&ExactSearcher{RgPath: rgPath}).SearchExact(context.Background(), model.ExactSearchRequest{
		Root:       root,
		Literals:   []string{"Needle("},
		Paths:      []string{"internal/auth"},
		MaxResults: 5,
		MaxBytes:   4096,
		TimeoutMS:  1000,
	})
	if err != nil {
		t.Fatalf("SearchExact() error = %v", err)
	}
	if len(response.Results) != 1 {
		t.Fatalf("Results = %#v, want one scoped result", response.Results)
	}
	if response.Results[0].Path != "internal/auth/service.go" {
		t.Fatalf("Path = %q, want internal/auth/service.go", response.Results[0].Path)
	}
	if response.Results[0].Line != 2 || response.Results[0].Column == 0 {
		t.Fatalf("location = L%d C%d, want line and column", response.Results[0].Line, response.Results[0].Column)
	}
}

func TestExactSearcherSkipsPathSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary on windows")
	}
	t.Parallel()

	rgPath, err := exec.LookPath("rg")
	if err != nil {
		t.Skip("rg not installed")
	}

	root := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.go"), "package secret\nfunc EscapedNeedle() {}\n")
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	response, err := (&ExactSearcher{RgPath: rgPath}).SearchExact(context.Background(), model.ExactSearchRequest{
		Root:       root,
		Literals:   []string{"EscapedNeedle"},
		Paths:      []string{"escape"},
		MaxResults: 5,
		MaxBytes:   4096,
		TimeoutMS:  1000,
	})
	if err != nil {
		t.Fatalf("SearchExact() error = %v", err)
	}
	if len(response.Results) != 0 {
		t.Fatalf("Results = %#v, want no escaped symlink reads", response.Results)
	}
	if len(response.Diagnostics) == 0 || !strings.Contains(strings.Join(response.Diagnostics, "\n"), "escapes repository root") {
		t.Fatalf("Diagnostics = %#v, want symlink escape note", response.Diagnostics)
	}
}

func TestExactSearcherReportsTruncatedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is unix-only")
	}
	t.Parallel()

	root := t.TempDir()
	rgPath := writeExecutable(t, filepath.Join(root, "fake-rg"), "#!/bin/sh\nprintf 'file.go\\t1\\t1\\t0123456789abcdefghijklmnopqrstuvwxyz\\n'\n")
	response, err := (&ExactSearcher{RgPath: rgPath}).SearchExact(context.Background(), model.ExactSearchRequest{
		Root:       root,
		Literals:   []string{"Needle"},
		MaxResults: 5,
		MaxBytes:   10,
		TimeoutMS:  1000,
	})
	if err != nil {
		t.Fatalf("SearchExact() error = %v", err)
	}
	if !response.Truncated {
		t.Fatalf("Truncated = false, want true")
	}
}

func TestExactSearcherReportsTimeoutAsDiagnostic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is unix-only")
	}
	t.Parallel()

	root := t.TempDir()
	rgPath := writeExecutable(t, filepath.Join(root, "fake-rg"), "#!/bin/sh\nsleep 1\n")
	response, err := (&ExactSearcher{RgPath: rgPath}).SearchExact(context.Background(), model.ExactSearchRequest{
		Root:       root,
		Literals:   []string{"Needle"},
		MaxResults: 5,
		MaxBytes:   4096,
		TimeoutMS:  1,
	})
	if err != nil {
		t.Fatalf("SearchExact() error = %v", err)
	}
	if !response.Truncated {
		t.Fatalf("Truncated = false, want true after timeout")
	}
	if !strings.Contains(strings.Join(response.Diagnostics, "\n"), "timed out") {
		t.Fatalf("Diagnostics = %#v, want timeout note", response.Diagnostics)
	}
}

func writeFile(t testing.TB, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeExecutable(t testing.TB, path, content string) string {
	t.Helper()
	writeFile(t, path, content)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	return path
}
