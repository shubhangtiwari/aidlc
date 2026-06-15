package install

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

func TestDiscoverBinaryUsesExplicitPathOnly(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "custom-aidlc")

	got := DiscoverBinary(BinaryDiscoveryRequest{
		ExplicitPath: executable,
		PathEnv:      filepath.Join(t.TempDir(), "bin"),
		HomeDir:      t.TempDir(),
		GoOS:         "linux",
		Executable: func(path string) bool {
			return path == executable
		},
	})

	if !got.Found {
		t.Fatal("Found = false, want true")
	}
	if got.Path != executable {
		t.Fatalf("Path = %q, want %q", got.Path, executable)
	}
	if got.Source != contract.AIDLCBinarySourceExplicit {
		t.Fatalf("Source = %q, want %q", got.Source, contract.AIDLCBinarySourceExplicit)
	}
	if len(got.Candidates) != 1 {
		t.Fatalf("Candidates len = %d, want 1", len(got.Candidates))
	}
}

func TestDiscoverBinaryChecksPATHBeforeCommonLocations(t *testing.T) {
	pathDir := filepath.Join(t.TempDir(), "path-bin")
	home := t.TempDir()
	pathBinary := filepath.Join(pathDir, "aidlc")
	commonBinary := filepath.Join(home, ".local", "bin", "aidlc")

	got := DiscoverBinary(BinaryDiscoveryRequest{
		PathEnv: pathDir,
		HomeDir: home,
		GoOS:    "linux",
		Executable: func(path string) bool {
			return path == pathBinary || path == commonBinary
		},
	})

	if got.Path != pathBinary {
		t.Fatalf("Path = %q, want %q", got.Path, pathBinary)
	}
	if got.Source != contract.AIDLCBinarySourcePATH {
		t.Fatalf("Source = %q, want %q", got.Source, contract.AIDLCBinarySourcePATH)
	}
	if got.Candidates[0].Path != pathBinary {
		t.Fatalf("first candidate = %q, want %q", got.Candidates[0].Path, pathBinary)
	}
}

func TestDiscoverBinaryUsesCommonLocationsWhenPATHMisses(t *testing.T) {
	home := t.TempDir()
	want := filepath.Join(home, "bin", "aidlc")

	got := DiscoverBinary(BinaryDiscoveryRequest{
		PathEnv: filepath.Join(t.TempDir(), "path-bin"),
		HomeDir: home,
		GoOS:    "linux",
		Executable: func(path string) bool {
			return path == want
		},
	})

	if !got.Found {
		t.Fatal("Found = false, want true")
	}
	if got.Path != want {
		t.Fatalf("Path = %q, want %q", got.Path, want)
	}
	if got.Source != contract.AIDLCBinarySourceCommonLocation {
		t.Fatalf("Source = %q, want %q", got.Source, contract.AIDLCBinarySourceCommonLocation)
	}
}

func TestDiscoverBinaryUsesWindowsLocalAppDataDefaultWhenPATHMisses(t *testing.T) {
	localAppData := filepath.Join(t.TempDir(), "AppData", "Local")
	want := filepath.Join(localAppData, "Programs", "aidlc", "bin", "aidlc.exe")

	got := DiscoverBinary(BinaryDiscoveryRequest{
		PathEnv:         filepath.Join(t.TempDir(), "path-bin"),
		LocalAppDataDir: localAppData,
		GoOS:            "windows",
		Executable: func(path string) bool {
			return path == want
		},
	})

	if !got.Found {
		t.Fatal("Found = false, want true")
	}
	if got.Path != want {
		t.Fatalf("Path = %q, want %q", got.Path, want)
	}
	if got.Source != contract.AIDLCBinarySourceCommonLocation {
		t.Fatalf("Source = %q, want %q", got.Source, contract.AIDLCBinarySourceCommonLocation)
	}
}

func TestCommonBinaryLocationsDerivesWindowsLocalAppDataFromHome(t *testing.T) {
	home := `C:\Users\me`

	got := CommonBinaryLocations(home, "windows")

	wantPrefix := []string{
		filepath.Join(home, "AppData", "Local", "Programs", "aidlc", "bin", "aidlc.exe"),
		filepath.Join(home, "AppData", "Local", "Programs", "aidlc", "bin", "aidlc"),
	}
	if len(got) < len(wantPrefix) {
		t.Fatalf("CommonBinaryLocations() len = %d, want at least %d", len(got), len(wantPrefix))
	}
	if !reflect.DeepEqual(got[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("CommonBinaryLocations() prefix = %#v, want %#v", got[:len(wantPrefix)], wantPrefix)
	}
}

func TestDiscoverBinaryDefaultExecutableCheckUsesRequestPATH(t *testing.T) {
	pathDir := t.TempDir()
	binary := filepath.Join(pathDir, "aidlc")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write binary fixture: %v", err)
	}

	got := DiscoverBinary(BinaryDiscoveryRequest{
		PathEnv: pathDir,
		GoOS:    "linux",
	})

	if !got.Found {
		t.Fatal("Found = false, want true")
	}
	if got.Path != binary {
		t.Fatalf("Path = %q, want %q", got.Path, binary)
	}
	if got.Source != contract.AIDLCBinarySourcePATH {
		t.Fatalf("Source = %q, want %q", got.Source, contract.AIDLCBinarySourcePATH)
	}
}

func TestDiscoverBinaryReportsDeterministicMisses(t *testing.T) {
	home := t.TempDir()
	pathDir := filepath.Join(t.TempDir(), "path-bin")

	got := DiscoverBinary(BinaryDiscoveryRequest{
		PathEnv: pathDir,
		HomeDir: home,
		GoOS:    "linux",
		Executable: func(string) bool {
			return false
		},
	})

	if got.Found {
		t.Fatalf("Found = true, want false with path %q", got.Path)
	}
	if got.Path != "" {
		t.Fatalf("Path = %q, want empty", got.Path)
	}
	wantPaths := []string{
		filepath.Join(pathDir, "aidlc"),
		filepath.Join(home, ".local", "bin", "aidlc"),
		filepath.Join(home, "bin", "aidlc"),
		"/opt/homebrew/bin/aidlc",
		"/usr/local/bin/aidlc",
	}
	gotPaths := make([]string, 0, len(got.Candidates))
	for _, candidate := range got.Candidates {
		gotPaths = append(gotPaths, candidate.Path)
		if candidate.Executable {
			t.Fatalf("candidate %q executable = true, want false", candidate.Path)
		}
	}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("candidate paths = %#v, want %#v", gotPaths, wantPaths)
	}
}

func TestCommonBinaryLocationsIncludesWindowsExecutableVariants(t *testing.T) {
	home := `C:\Users\me`

	got := CommonBinaryLocations(home, "windows")

	want := []string{
		filepath.Join(home, "AppData", "Local", "Programs", "aidlc", "bin", "aidlc.exe"),
		filepath.Join(home, "AppData", "Local", "Programs", "aidlc", "bin", "aidlc"),
		filepath.Join(home, ".local", "bin", "aidlc.exe"),
		filepath.Join(home, ".local", "bin", "aidlc"),
		filepath.Join(home, "bin", "aidlc.exe"),
		filepath.Join(home, "bin", "aidlc"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CommonBinaryLocations() = %#v, want %#v", got, want)
	}
}
