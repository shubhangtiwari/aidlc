package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMakefileIncUsesExplicitAIDLCBinForPublicTargets(t *testing.T) {
	project := createMakeHelperProject(t)
	fake := writeFakeAIDLC(t, t.TempDir(), "aidlc")
	logPath := filepath.Join(t.TempDir(), "aidlc.log")

	targets := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "ai-map", want: "map --dir ."},
		{name: "ai-map-check", want: "map --dir . --check"},
		{name: "ai-query", env: map[string]string{"AI_QUERY": "resolver terms"}, want: "query --dir . --limit 10 resolver terms"},
		{name: "ai-doctor", want: "doctor --dir ."},
	}
	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
				t.Fatalf("remove log: %v", err)
			}
			env := map[string]string{
				"AIDLC_BIN": fake,
				"AIDLC_LOG": logPath,
				"PATH":      t.TempDir(),
			}
			for key, value := range target.env {
				env[key] = value
			}

			output, code := runMakeHelper(t, project, env, target.name)
			if code != 0 {
				t.Fatalf("make %s failed with code %d:\n%s", target.name, code, output)
			}
			got := strings.TrimSpace(readText(t, logPath))
			if got != target.want {
				t.Fatalf("logged args = %q, want %q", got, target.want)
			}
		})
	}
}

func TestMakefileIncResolvesAIDLCFromPATH(t *testing.T) {
	project := createMakeHelperProject(t)
	pathDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "aidlc.log")
	writeFakeAIDLC(t, pathDir, "aidlc")

	output, code := runMakeHelper(t, project, map[string]string{
		"AIDLC_LOG": logPath,
		"PATH":      pathDir,
	}, "ai-doctor")
	if code != 0 {
		t.Fatalf("make ai-doctor failed with code %d:\n%s", code, output)
	}
	if got := strings.TrimSpace(readText(t, logPath)); got != "doctor --dir ." {
		t.Fatalf("logged args = %q, want doctor --dir .", got)
	}
}

func TestMakefileIncResolvesAIDLCFromCommonHomeLocation(t *testing.T) {
	project := createMakeHelperProject(t)
	home := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "aidlc.log")
	writeFakeAIDLC(t, filepath.Join(home, ".local", "bin"), "aidlc")

	output, code := runMakeHelper(t, project, map[string]string{
		"AIDLC_LOG": logPath,
		"HOME":      home,
		"PATH":      t.TempDir(),
	}, "ai-doctor")
	if code != 0 {
		t.Fatalf("make ai-doctor failed with code %d:\n%s", code, output)
	}
	if got := strings.TrimSpace(readText(t, logPath)); got != "doctor --dir ." {
		t.Fatalf("logged args = %q, want doctor --dir .", got)
	}
}

func TestMakefileIncResolvesAIDLCFromWindowsLocalAppDataUnderSanitizedPATH(t *testing.T) {
	project := createMakeHelperProject(t)
	localAppData := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "aidlc.log")
	writeFakeAIDLC(t, filepath.Join(localAppData, "Programs", "aidlc", "bin"), "aidlc.exe")

	output, code := runMakeHelper(t, project, map[string]string{
		"AIDLC_LOG":    logPath,
		"HOME":         t.TempDir(),
		"LOCALAPPDATA": localAppData,
		"PATH":         t.TempDir(),
	}, "ai-doctor")
	if code != 0 {
		t.Fatalf("make ai-doctor failed with code %d:\n%s", code, output)
	}
	if got := strings.TrimSpace(readText(t, logPath)); got != "doctor --dir ." {
		t.Fatalf("logged args = %q, want doctor --dir .", got)
	}
}

func TestMakefileIncMissingAIDLCFailureIsActionable(t *testing.T) {
	project := createMakeHelperProject(t)

	output, code := runMakeHelper(t, project, map[string]string{
		"AIDLC_SYSTEM_BIN_DIRS": "",
		"HOME":                  t.TempDir(),
		"PATH":                  t.TempDir(),
	}, "ai-doctor")
	if code == 0 {
		t.Fatalf("make ai-doctor succeeded, want failure:\n%s", output)
	}
	for _, want := range []string{
		"aidlc executable not found",
		"AIDLC_BIN=/path/to/aidlc",
		"make ai-doctor",
		"AIDLC_INSTALL_DIR",
		"$LOCALAPPDATA/Programs/aidlc/bin/aidlc.exe",
		"%LOCALAPPDATA%\\Programs\\aidlc\\bin",
		"PowerShell installer",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("failure output missing %q:\n%s", want, output)
		}
	}
}

func TestMakefileIncDoctorFailureIsActionableForTooOldAIDLC(t *testing.T) {
	project := createMakeHelperProject(t)
	fake := writeFakeOldAIDLC(t, t.TempDir(), "aidlc")
	logPath := filepath.Join(t.TempDir(), "aidlc.log")

	output, code := runMakeHelper(t, project, map[string]string{
		"AIDLC_BIN": fake,
		"AIDLC_LOG": logPath,
		"PATH":      t.TempDir(),
	}, "ai-doctor")
	if code == 0 {
		t.Fatalf("make ai-doctor succeeded, want failure:\n%s", output)
	}
	for _, want := range []string{
		"does not support the doctor command",
		"Resolved aidlc: " + fake,
		"Upgrade or reinstall aidlc",
		"AIDLC_BIN=/path/to/newer/aidlc",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("failure output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, `unknown command "doctor"`) {
		t.Fatalf("failure output surfaced raw old-binary error:\n%s", output)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("old aidlc should not run doctor after failed capability check, log stat err = %v", err)
	}
}

func createMakeHelperProject(t *testing.T) string {
	t.Helper()

	project := t.TempDir()
	if err := os.Mkdir(filepath.Join(project, ".ai"), 0o755); err != nil {
		t.Fatalf("mkdir .ai: %v", err)
	}
	include, err := os.ReadFile(scopeRootFile(t, ".ai", "Makefile.inc"))
	if err != nil {
		t.Fatalf("read Makefile.inc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, ".ai", "Makefile.inc"), include, 0o644); err != nil {
		t.Fatalf("write Makefile.inc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "Makefile"), []byte("-include .ai/Makefile.inc\n"), 0o644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}
	return project
}

func writeFakeAIDLC(t *testing.T, dir, name string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir fake aidlc dir: %v", err)
	}
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\nif [ \"$1\" = \"doctor\" ] && [ \"$2\" = \"--help\" ]; then exit 0; fi\nprintf '%s\\n' \"$*\" >> \"$AIDLC_LOG\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake aidlc: %v", err)
	}
	return path
}

func writeFakeOldAIDLC(t *testing.T, dir, name string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir fake old aidlc dir: %v", err)
	}
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\nif [ \"$1\" = \"doctor\" ]; then printf '%s\\n' 'aidlc: unknown command \"doctor\"' >&2; exit 1; fi\nprintf '%s\\n' \"$*\" >> \"$AIDLC_LOG\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake old aidlc: %v", err)
	}
	return path
}

func runMakeHelper(t *testing.T, dir string, extraEnv map[string]string, targets ...string) (string, int) {
	t.Helper()

	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is required for integration tests: %v", err)
	}
	cmd := exec.Command(makePath, targets...)
	cmd.Dir = dir
	cmd.Env = makeHelperEnv(extraEnv)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(output), exitErr.ExitCode()
	}
	t.Fatalf("run make: %v\n%s", err, output)
	return "", 1
}

func makeHelperEnv(extra map[string]string) []string {
	envMap := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			envMap[key] = value
		}
	}
	for key, value := range extra {
		envMap[key] = value
	}

	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, key+"="+value)
	}
	return env
}

func scopeRootFile(t *testing.T, parts ...string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	allParts := append([]string{root}, parts...)
	return filepath.Join(allParts...)
}

func readText(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
