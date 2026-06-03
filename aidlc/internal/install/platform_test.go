package install

import (
	"errors"
	"testing"
)

func TestReleasePlatformForSupportedTargets(t *testing.T) {
	tests := []struct {
		name        string
		goos        string
		goarch      string
		wantOS      string
		wantArch    string
		wantArchive string
		wantBinary  string
	}{
		{
			name:        "darwin amd64",
			goos:        "darwin",
			goarch:      "amd64",
			wantOS:      "darwin",
			wantArch:    "x86_64",
			wantArchive: "aidlc_darwin_x86_64.tar.gz",
			wantBinary:  "aidlc",
		},
		{
			name:        "darwin arm64",
			goos:        "darwin",
			goarch:      "arm64",
			wantOS:      "darwin",
			wantArch:    "arm64",
			wantArchive: "aidlc_darwin_arm64.tar.gz",
			wantBinary:  "aidlc",
		},
		{
			name:        "linux amd64",
			goos:        "linux",
			goarch:      "amd64",
			wantOS:      "linux",
			wantArch:    "x86_64",
			wantArchive: "aidlc_linux_x86_64.tar.gz",
			wantBinary:  "aidlc",
		},
		{
			name:        "linux arm64",
			goos:        "linux",
			goarch:      "arm64",
			wantOS:      "linux",
			wantArch:    "arm64",
			wantArchive: "aidlc_linux_arm64.tar.gz",
			wantBinary:  "aidlc",
		},
		{
			name:        "windows amd64",
			goos:        "windows",
			goarch:      "amd64",
			wantOS:      "windows",
			wantArch:    "x86_64",
			wantArchive: "aidlc_windows_x86_64.zip",
			wantBinary:  "aidlc.exe",
		},
		{
			name:        "windows arm64",
			goos:        "windows",
			goarch:      "arm64",
			wantOS:      "windows",
			wantArch:    "arm64",
			wantArchive: "aidlc_windows_arm64.zip",
			wantBinary:  "aidlc.exe",
		},
		{
			name:        "uname amd64 alias",
			goos:        "linux",
			goarch:      "x86_64",
			wantOS:      "linux",
			wantArch:    "x86_64",
			wantArchive: "aidlc_linux_x86_64.tar.gz",
			wantBinary:  "aidlc",
		},
		{
			name:        "uname arm64 alias",
			goos:        "linux",
			goarch:      "aarch64",
			wantOS:      "linux",
			wantArch:    "arm64",
			wantArchive: "aidlc_linux_arm64.tar.gz",
			wantBinary:  "aidlc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReleasePlatformFor(tt.goos, tt.goarch)
			if err != nil {
				t.Fatalf("ReleasePlatformFor() error = %v", err)
			}
			if got.OS != tt.wantOS {
				t.Fatalf("OS = %q, want %q", got.OS, tt.wantOS)
			}
			if got.Arch != tt.wantArch {
				t.Fatalf("Arch = %q, want %q", got.Arch, tt.wantArch)
			}
			if got.ArchiveName != tt.wantArchive {
				t.Fatalf("ArchiveName = %q, want %q", got.ArchiveName, tt.wantArchive)
			}
			if got.BinaryName != tt.wantBinary {
				t.Fatalf("BinaryName = %q, want %q", got.BinaryName, tt.wantBinary)
			}
		})
	}
}

func TestReleasePlatformForUnsupportedOSReturnsUsageError(t *testing.T) {
	_, err := ReleasePlatformFor("freebsd", "amd64")
	if err == nil {
		t.Fatal("ReleasePlatformFor() error = nil")
	}
	if !IsUsageError(err) {
		t.Fatalf("IsUsageError() = false for %T %v", err, err)
	}
	if got, want := err.Error(), "unsupported OS: freebsd"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestReleasePlatformForUnsupportedArchitectureReturnsUsageError(t *testing.T) {
	_, err := ReleasePlatformFor("linux", "riscv64")
	if err == nil {
		t.Fatal("ReleasePlatformFor() error = nil")
	}
	var usageErr *UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("error is not UsageError: %T %v", err, err)
	}
	if got, want := usageErr.Error(), "unsupported architecture: riscv64"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
