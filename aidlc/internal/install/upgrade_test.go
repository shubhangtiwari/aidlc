package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

func TestUpgradeDryRunResolvesLatestWithoutDownloadingAssets(t *testing.T) {
	installDir := t.TempDir()
	server := upgradeServer(t, upgradeServerOptions{
		Tag:          "aidlc/v0.2.0",
		ArchiveName:  "aidlc_linux_x86_64.tar.gz",
		ArchiveBytes: tarGzipBinary(t, "aidlc", []byte("new binary")),
		DownloadTrap: true,
	})
	result, err := Upgrade(context.Background(), UpgradeRequest{
		Options: contract.UpgradeOptions{
			Repository: "owner/repo",
			Version:    contract.UpgradeVersionSelector{Value: "latest"},
			InstallDir: installDir,
			DryRun:     true,
		},
		GoOS:       "linux",
		GoArch:     "amd64",
		APIBaseURL: server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if !result.DryRun {
		t.Fatal("DryRun = false")
	}
	if result.Installed {
		t.Fatal("Installed = true")
	}
	if got, want := result.ReleaseTag, "aidlc/v0.2.0"; got != want {
		t.Fatalf("ReleaseTag = %q, want %q", got, want)
	}
	if got, want := result.Version, "v0.2.0"; got != want {
		t.Fatalf("Version = %q, want %q", got, want)
	}
	if got, want := result.AssetName, "aidlc_linux_x86_64.tar.gz"; got != want {
		t.Fatalf("AssetName = %q, want %q", got, want)
	}
	if got, want := result.Destination, filepath.Join(installDir, "aidlc"); got != want {
		t.Fatalf("Destination = %q, want %q", got, want)
	}
}

func TestUpgradeAcceptsExplicitAidlcTagAndInstallsTarGzip(t *testing.T) {
	installDir := t.TempDir()
	archive := tarGzipBinary(t, "aidlc", []byte("upgraded"))
	server := upgradeServer(t, upgradeServerOptions{
		Tag:          "aidlc/v1.2.3",
		ArchiveName:  "aidlc_darwin_arm64.tar.gz",
		ArchiveBytes: archive,
	})
	result, err := Upgrade(context.Background(), UpgradeRequest{
		Options: contract.UpgradeOptions{
			Repository: "owner/repo",
			Version:    contract.UpgradeVersionSelector{Value: "aidlc/v1.2.3", Explicit: true},
			InstallDir: installDir,
		},
		GoOS:       "darwin",
		GoArch:     "arm64",
		APIBaseURL: server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if !result.Installed {
		t.Fatal("Installed = false")
	}
	if got, want := result.Version, "v1.2.3"; got != want {
		t.Fatalf("Version = %q, want %q", got, want)
	}
	data, err := os.ReadFile(filepath.Join(installDir, "aidlc"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got, want := string(data), "upgraded"; got != want {
		t.Fatalf("installed binary = %q, want %q", got, want)
	}
}

func TestUpgradeInstallsWindowsZipWithoutShellingOut(t *testing.T) {
	installDir := t.TempDir()
	archive := zipBinary(t, "aidlc.exe", []byte("windows binary"))
	server := upgradeServer(t, upgradeServerOptions{
		Tag:                 "aidlc/v2.0.0",
		ArchiveName:         "aidlc_windows_x86_64.zip",
		ArchiveBytes:        archive,
		ExpectedReleasePath: "/repos/owner/repo/releases/tags/aidlc%2Fv2.0.0",
	})
	result, err := Upgrade(context.Background(), UpgradeRequest{
		Options: contract.UpgradeOptions{
			Repository: "owner/repo",
			Version:    contract.UpgradeVersionSelector{Value: "v2.0.0", Explicit: true},
			InstallDir: installDir,
		},
		GoOS:       "windows",
		GoArch:     "amd64",
		APIBaseURL: server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if !result.Installed {
		t.Fatal("Installed = false")
	}
	data, err := os.ReadFile(filepath.Join(installDir, "aidlc.exe"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got, want := string(data), "windows binary"; got != want {
		t.Fatalf("installed binary = %q, want %q", got, want)
	}
}

func TestUpgradeWindowsDefaultDestinationReplacesExecutableDirectoryBinary(t *testing.T) {
	execDir := t.TempDir()
	destination := filepath.Join(execDir, "aidlc.exe")
	if err := os.WriteFile(destination, []byte("existing windows binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	archive := zipBinary(t, "aidlc.exe", []byte("upgraded windows binary"))
	server := upgradeServer(t, upgradeServerOptions{
		Tag:          "aidlc/v2.1.0",
		ArchiveName:  "aidlc_windows_x86_64.zip",
		ArchiveBytes: archive,
	})
	result, err := Upgrade(context.Background(), UpgradeRequest{
		Options: contract.UpgradeOptions{
			Repository: "owner/repo",
			Version:    contract.UpgradeVersionSelector{Value: "latest"},
		},
		CurrentVersion: "v2.0.0",
		GoOS:           "windows",
		GoArch:         "amd64",
		ExecutablePath: filepath.Join(execDir, "aidlc.exe"),
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
	})
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if !result.Installed {
		t.Fatal("Installed = false")
	}
	if got, want := result.Destination, destination; got != want {
		t.Fatalf("Destination = %q, want %q", got, want)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got, want := string(data), "upgraded windows binary"; got != want {
		t.Fatalf("installed binary = %q, want %q", got, want)
	}
	matches, err := filepath.Glob(filepath.Join(execDir, "aidlc.exe.backup-*"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("backup files left behind = %v, want none", matches)
	}
}

func TestReplaceBinaryWindowsRollsBackExistingBinaryWhenReplacementFails(t *testing.T) {
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "aidlc.exe")
	if err := os.WriteFile(destination, []byte("original"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	err := replaceBinaryWindows(destination, filepath.Join(installDir, "missing-staged.exe"))
	if err == nil {
		t.Fatal("replaceBinaryWindows() error = nil")
	}
	if !strings.Contains(err.Error(), "replace binary") {
		t.Fatalf("error = %q, want replace binary", err)
	}
	data, readErr := os.ReadFile(destination)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if got, want := string(data), "original"; got != want {
		t.Fatalf("destination after rollback = %q, want %q", got, want)
	}
}

func TestUpgradeChecksumMismatchPreventsDestinationWrite(t *testing.T) {
	installDir := t.TempDir()
	destination := filepath.Join(installDir, "aidlc")
	if err := os.WriteFile(destination, []byte("original"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	server := upgradeServer(t, upgradeServerOptions{
		Tag:          "v3.0.0",
		ArchiveName:  "aidlc_linux_arm64.tar.gz",
		ArchiveBytes: tarGzipBinary(t, "aidlc", []byte("changed")),
		ChecksumBody: strings.Repeat("0", 64) + " aidlc_linux_arm64.tar.gz\n",
	})
	_, err := Upgrade(context.Background(), UpgradeRequest{
		Options: contract.UpgradeOptions{
			Repository: "owner/repo",
			Version:    contract.UpgradeVersionSelector{Value: "v3.0.0", Explicit: true},
			InstallDir: installDir,
		},
		GoOS:       "linux",
		GoArch:     "arm64",
		APIBaseURL: server.URL,
		HTTPClient: server.Client(),
	})
	if err == nil {
		t.Fatal("Upgrade() error = nil")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %q, want checksum mismatch", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got, want := string(data), "original"; got != want {
		t.Fatalf("destination = %q, want %q", got, want)
	}
}

func TestUpgradeRejectsBadHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	_, err := Upgrade(context.Background(), UpgradeRequest{
		Options: contract.UpgradeOptions{
			Repository: "owner/repo",
			Version:    contract.UpgradeVersionSelector{Value: "latest"},
			InstallDir: t.TempDir(),
		},
		GoOS:       "linux",
		GoArch:     "amd64",
		APIBaseURL: server.URL,
		HTTPClient: server.Client(),
	})
	if err == nil {
		t.Fatal("Upgrade() error = nil")
	}
	if !strings.Contains(err.Error(), "404 Not Found") {
		t.Fatalf("error = %q, want HTTP status", err)
	}
}

func TestUpgradeSkipsLatestWhenCurrentVersionMatches(t *testing.T) {
	installDir := t.TempDir()
	server := upgradeServer(t, upgradeServerOptions{
		Tag:          "v4.0.0",
		ArchiveName:  "aidlc_linux_x86_64.tar.gz",
		ArchiveBytes: tarGzipBinary(t, "aidlc", []byte("new binary")),
		DownloadTrap: true,
	})
	result, err := Upgrade(context.Background(), UpgradeRequest{
		Options: contract.UpgradeOptions{
			Repository: "owner/repo",
			Version:    contract.UpgradeVersionSelector{Value: "latest"},
			InstallDir: installDir,
		},
		CurrentVersion: "v4.0.0",
		GoOS:           "linux",
		GoArch:         "amd64",
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
	})
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if !result.Skipped {
		t.Fatal("Skipped = false")
	}
	if result.Installed {
		t.Fatal("Installed = true")
	}
	if _, err := os.Stat(filepath.Join(installDir, "aidlc")); !os.IsNotExist(err) {
		t.Fatalf("destination stat error = %v, want not exist", err)
	}
}

func TestUpgradeUsesExecutableDirectoryWhenInstallDirIsEmpty(t *testing.T) {
	execDir := t.TempDir()
	execPath := filepath.Join(execDir, "custom-aidlc")
	server := upgradeServer(t, upgradeServerOptions{
		Tag:          "v5.0.0",
		ArchiveName:  "aidlc_linux_x86_64.tar.gz",
		ArchiveBytes: tarGzipBinary(t, "aidlc", []byte("from executable dir")),
	})
	result, err := Upgrade(context.Background(), UpgradeRequest{
		Options: contract.UpgradeOptions{
			Repository: "owner/repo",
			Version:    contract.UpgradeVersionSelector{Value: "v5.0.0", Explicit: true},
		},
		GoOS:           "linux",
		GoArch:         "amd64",
		ExecutablePath: execPath,
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
	})
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if got, want := result.Destination, filepath.Join(execDir, "aidlc"); got != want {
		t.Fatalf("Destination = %q, want %q", got, want)
	}
	if _, err := os.Stat(result.Destination); err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
}

type upgradeServerOptions struct {
	Tag                 string
	ArchiveName         string
	ArchiveBytes        []byte
	ChecksumBody        string
	DownloadTrap        bool
	ExpectedReleasePath string
}

func upgradeServer(t *testing.T, opts upgradeServerOptions) *httptest.Server {
	t.Helper()
	if opts.Tag == "" {
		opts.Tag = "v0.1.0"
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repos/owner/repo/releases/") {
			if opts.ExpectedReleasePath != "" && r.URL.EscapedPath() != opts.ExpectedReleasePath {
				t.Fatalf("release request path = %q, want %q", r.URL.EscapedPath(), opts.ExpectedReleasePath)
			}
			fmt.Fprintf(w, `{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":"%s"},{"name":"checksums.txt","browser_download_url":"%s"}]}`,
				opts.Tag, opts.ArchiveName, externalURL(r, "/download/archive"), externalURL(r, "/download/checksums"))
			return
		}
		if opts.DownloadTrap {
			t.Fatalf("unexpected asset download: %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/download/archive":
			w.Write(opts.ArchiveBytes)
		case "/download/checksums":
			body := opts.ChecksumBody
			if body == "" {
				sum := sha256.Sum256(opts.ArchiveBytes)
				body = hex.EncodeToString(sum[:]) + " " + opts.ArchiveName + "\n"
			}
			w.Write([]byte(body))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func externalURL(r *http.Request, path string) string {
	return "http://" + r.Host + path
}

func tarGzipBinary(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(data)),
	}); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if _, err := tarWriter.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tar Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzip Close() error = %v", err)
	}
	return buf.Bytes()
}

func zipBinary(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	file, err := writer.Create(name)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buf.Bytes()
}
