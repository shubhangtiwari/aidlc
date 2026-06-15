package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

func TestDoctorHealthy(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".ai", "Makefile.inc"), "ai-doctor:\n")
	writeFile(t, filepath.Join(root, "Makefile"), "-include .ai/Makefile.inc\n")
	home := t.TempDir()
	pathDir := filepath.Join(t.TempDir(), "bin")
	pathBinary := filepath.Join(pathDir, "aidlc")
	commonBinary := filepath.Join(home, ".local", "bin", "aidlc")

	var stdout, stderr bytes.Buffer
	code := runDoctorForTest([]string{"--dir", root}, &stdout, &stderr, DoctorDependencies{
		ExecutablePath: pathBinary,
		Version:        "v1.2.3",
		PathEnv:        pathDir,
		HomeDir:        home,
		GoOS:           "linux",
		Executable: func(path string) bool {
			return path == pathBinary || path == commonBinary
		},
	})

	if code != contract.ExitOK {
		t.Fatalf("doctor code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"version: v1.2.3",
		"executable: " + pathBinary,
		"PATH discoverable: yes",
		commonBinary + " [executable]",
		"Makefile: " + filepath.Join(root, "Makefile") + " (includes .ai/Makefile.inc)",
		"status: healthy",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestDoctorReportsActionableFindings(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".ai", "Makefile.inc"), "ai-doctor:\n")
	writeFile(t, filepath.Join(root, "Makefile"), "test:\n\tgo test ./...\n")
	home := t.TempDir()
	executable := filepath.Join(home, ".local", "bin", "aidlc")

	var stdout, stderr bytes.Buffer
	code := runDoctorForTest([]string{"--dir", root}, &stdout, &stderr, DoctorDependencies{
		ExecutablePath: executable,
		Version:        "dev",
		PathEnv:        filepath.Join(t.TempDir(), "path-bin"),
		HomeDir:        home,
		GoOS:           "linux",
		Executable: func(path string) bool {
			return path == executable
		},
	})

	if code != contract.ExitConflict {
		t.Fatalf("doctor code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"PATH discoverable: no",
		executable + " [executable]",
		"root Makefile does not include .ai/Makefile.inc",
		"For sanitized IDE shells or CI, set AIDLC_BIN",
		"AIDLC_BIN=" + executable + " make ai-doctor",
		"Ensure the repository root Makefile includes: -include .ai/Makefile.inc",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestDoctorMissingHelperAndMakefileAreFindings(t *testing.T) {
	root := t.TempDir()
	pathDir := filepath.Join(t.TempDir(), "bin")
	pathBinary := filepath.Join(pathDir, "aidlc")

	result, err := RunDoctor(DoctorOptions{Dir: root}, DoctorDependencies{
		ExecutablePath: pathBinary,
		Version:        "dev",
		PathEnv:        pathDir,
		HomeDir:        t.TempDir(),
		GoOS:           "linux",
		Executable: func(path string) bool {
			return path == pathBinary
		},
	})
	if err != nil {
		t.Fatalf("RunDoctor() error = %v", err)
	}
	joined := strings.Join(result.Findings, "\n")
	for _, want := range []string{
		".ai/Makefile.inc is missing",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("findings missing %q: %#v", want, result.Findings)
		}
	}
	if strings.Contains(joined, "root Makefile is missing") {
		t.Fatalf("missing Makefile should not be reported when helper is absent: %#v", result.Findings)
	}
}

func TestDoctorUsageErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"--unknown"}},
		{name: "extra arg", args: []string{"extra"}},
		{name: "empty dir", args: []string{"--dir", ""}},
		{name: "missing dir", args: []string{"--dir", filepath.Join(t.TempDir(), "missing")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runDoctorForTest(tc.args, &stdout, &stderr, DoctorDependencies{
				PathEnv: filepath.Join(t.TempDir(), "bin"),
				HomeDir: t.TempDir(),
				GoOS:    "linux",
			})
			if code != contract.ExitUsage {
				t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestDoctorHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunDoctorCLI([]string{"--help"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("help code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage: aidlc doctor [flags]") {
		t.Fatalf("help missing usage:\n%s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestMakefileIncludeParsing(t *testing.T) {
	for _, line := range []string{
		"include .ai/Makefile.inc",
		"-include .ai/Makefile.inc # generated helper",
		"sinclude .ai/Makefile.inc",
	} {
		if !makefileIncludesAIDLC([]byte(line + "\n")) {
			t.Fatalf("makefileIncludesAIDLC(%q) = false, want true", line)
		}
	}
	if makefileIncludesAIDLC([]byte("include other.mk\n# -include .ai/Makefile.inc\n")) {
		t.Fatal("makefileIncludesAIDLC() = true for commented include")
	}
}

func runDoctorForTest(args []string, stdout, stderr *bytes.Buffer, deps DoctorDependencies) int {
	if isHelpArg(args) {
		printDoctorUsage(stdout)
		return contract.ExitOK
	}

	opts := DoctorOptions{Dir: "."}
	fs := newDoctorFlagSet(&opts, stderr)
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return contract.ExitUsage
	}
	result, err := RunDoctor(opts, deps)
	if err != nil {
		return contract.ExitUsage
	}
	printDoctorResult(stdout, result)
	if len(result.Findings) > 0 {
		return contract.ExitConflict
	}
	return contract.ExitOK
}

func writeFile(t testing.TB, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}
