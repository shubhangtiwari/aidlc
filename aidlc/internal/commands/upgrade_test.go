package commands

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/install"
)

func TestUpgradeCLIHelpDocumentsFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := RunUpgradeCLI(context.Background(), []string{"--help"}, &stdout, &stderr); code != contract.ExitOK {
		t.Fatalf("upgrade help code = %d", code)
	}
	output := stdout.String()
	for _, want := range []string{
		"Usage: aidlc upgrade [flags]\n",
		"--repo owner/repo",
		"--version latest|TAG",
		"--install-dir DIR",
		"--dry-run",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestUpgradeCLIParsesFlagsAndPrintsInstalledResult(t *testing.T) {
	previousVersion := Version
	previousRun := runUpgrade
	t.Cleanup(func() {
		Version = previousVersion
		runUpgrade = previousRun
	})
	Version = "v1.0.0"
	runUpgrade = func(_ context.Context, req install.UpgradeRequest) (install.UpgradeResult, error) {
		if got, want := req.Options.Repository, "owner/repo"; got != want {
			t.Fatalf("Repository = %q, want %q", got, want)
		}
		if got, want := req.Options.Version.Value, "v1.2.3"; got != want {
			t.Fatalf("Version.Value = %q, want %q", got, want)
		}
		if !req.Options.Version.Explicit {
			t.Fatal("Version.Explicit = false")
		}
		if got, want := req.Options.InstallDir, "/tmp/aidlc-bin"; got != want {
			t.Fatalf("InstallDir = %q, want %q", got, want)
		}
		if req.Options.DryRun {
			t.Fatal("DryRun = true")
		}
		if got, want := req.CurrentVersion, "v1.0.0"; got != want {
			t.Fatalf("CurrentVersion = %q, want %q", got, want)
		}
		return install.UpgradeResult{
			ReleaseTag:  "aidlc/v1.2.3",
			Version:     "v1.2.3",
			AssetName:   "aidlc_linux_x86_64.tar.gz",
			Destination: "/tmp/aidlc-bin/aidlc",
			Installed:   true,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := RunUpgradeCLI(context.Background(), []string{
		"--repo", "owner/repo",
		"--version", "v1.2.3",
		"--install-dir", "/tmp/aidlc-bin",
	}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("upgrade code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	want := strings.Join([]string{
		"current version: v1.0.0",
		"target version: v1.2.3",
		"release tag: aidlc/v1.2.3",
		"selected asset: aidlc_linux_x86_64.tar.gz",
		"destination: /tmp/aidlc-bin/aidlc",
		"status: installed",
		"",
	}, "\n")
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestUpgradeCLIDryRunOutput(t *testing.T) {
	previousRun := runUpgrade
	t.Cleanup(func() { runUpgrade = previousRun })
	runUpgrade = func(_ context.Context, req install.UpgradeRequest) (install.UpgradeResult, error) {
		if !req.Options.DryRun {
			t.Fatal("DryRun = false")
		}
		return install.UpgradeResult{
			ReleaseTag:  "v2.0.0",
			Version:     "v2.0.0",
			AssetName:   "aidlc_darwin_arm64.tar.gz",
			Destination: "/opt/bin/aidlc",
			DryRun:      true,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := RunUpgradeCLI(context.Background(), []string{"--dry-run"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("upgrade dry-run code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "status: dry-run\n") {
		t.Fatalf("stdout missing dry-run status:\n%s", stdout.String())
	}
}

func TestUpgradeCLISkipsLatestWhenCurrentVersionMatches(t *testing.T) {
	previousRun := runUpgrade
	t.Cleanup(func() { runUpgrade = previousRun })
	runUpgrade = func(_ context.Context, req install.UpgradeRequest) (install.UpgradeResult, error) {
		if req.Options.Version.Explicit {
			t.Fatal("latest selector should not request explicit reinstall")
		}
		return install.UpgradeResult{
			ReleaseTag:  "v3.0.0",
			Version:     "v3.0.0",
			AssetName:   "aidlc_linux_x86_64.tar.gz",
			Destination: "/usr/local/bin/aidlc",
			Skipped:     true,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := RunUpgradeCLI(context.Background(), []string{"--version", "latest"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("upgrade skip code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "status: skipped\n") {
		t.Fatalf("stdout missing skipped status:\n%s", stdout.String())
	}
}

func TestUpgradeCLIErrorsExitUsageWithDeterministicPrefix(t *testing.T) {
	previousRun := runUpgrade
	t.Cleanup(func() { runUpgrade = previousRun })
	runUpgrade = func(context.Context, install.UpgradeRequest) (install.UpgradeResult, error) {
		return install.UpgradeResult{}, errors.New("download checksums.txt: 404 Not Found")
	}

	var stdout, stderr bytes.Buffer
	code := RunUpgradeCLI(context.Background(), nil, &stdout, &stderr)
	if code != contract.ExitUsage {
		t.Fatalf("upgrade error code = %d, want usage", code)
	}
	if got, want := stderr.String(), "aidlc upgrade: download checksums.txt: 404 Not Found\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestUpgradeCLIUnexpectedArgumentExitsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunUpgradeCLI(context.Background(), []string{"extra"}, &stdout, &stderr)
	if code != contract.ExitUsage {
		t.Fatalf("upgrade code = %d, want usage", code)
	}
	if got, want := stderr.String(), "aidlc upgrade: unexpected argument \"extra\"\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}
