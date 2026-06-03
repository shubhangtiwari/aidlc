package commands

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/install"
)

var runUpgrade = install.Upgrade

func RunUpgradeCLI(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if isHelpArg(args) {
		printUpgradeUsage(stdout)
		return contract.ExitOK
	}

	opts := contract.UpgradeOptions{
		Repository: install.DefaultUpgradeRepository,
		Version:    contract.UpgradeVersionSelector{Value: "latest"},
	}
	version := opts.Version.Value
	fs := flag.NewFlagSet("aidlc upgrade", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.Repository, "repo", opts.Repository, "GitHub repository owner/repo")
	fs.StringVar(&version, "version", version, "release version: latest, vX.Y.Z, or aidlc/vX.Y.Z")
	fs.StringVar(&opts.InstallDir, "install-dir", "", "directory where aidlc is installed")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "print planned upgrade without writing")
	fs.Usage = func() { printUpgradeUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "aidlc upgrade: %v\n", err)
		return contract.ExitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "aidlc upgrade: unexpected argument %q\n", fs.Arg(0))
		return contract.ExitUsage
	}
	opts.Version = contract.UpgradeVersionSelector{
		Value:    strings.TrimSpace(version),
		Explicit: strings.TrimSpace(version) != "" && strings.TrimSpace(version) != "latest",
	}

	current := CurrentVersion()
	result, err := runUpgrade(ctx, install.UpgradeRequest{
		Options:        opts,
		CurrentVersion: current,
	})
	if err != nil {
		fmt.Fprintf(stderr, "aidlc upgrade: %v\n", err)
		return contract.ExitUsage
	}
	printUpgradeResult(stdout, current, result)
	return contract.ExitOK
}

func printUpgradeUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc upgrade [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --repo owner/repo      GitHub repository (default shubhangtiwari/aidlc)")
	fmt.Fprintln(w, "  --version latest|TAG   Release selector (default latest)")
	fmt.Fprintln(w, "  --install-dir DIR      Install destination directory (default current executable directory)")
	fmt.Fprintln(w, "  --dry-run              Print planned upgrade without writing")
}

func printUpgradeResult(w io.Writer, current string, result install.UpgradeResult) {
	fmt.Fprintf(w, "current version: %s\n", current)
	fmt.Fprintf(w, "target version: %s\n", result.Version)
	fmt.Fprintf(w, "release tag: %s\n", result.ReleaseTag)
	fmt.Fprintf(w, "selected asset: %s\n", result.AssetName)
	fmt.Fprintf(w, "destination: %s\n", result.Destination)
	fmt.Fprintf(w, "status: %s\n", upgradeStatus(result))
}

func upgradeStatus(result install.UpgradeResult) string {
	switch {
	case result.DryRun:
		return "dry-run"
	case result.Skipped:
		return "skipped"
	case result.Installed:
		return "installed"
	default:
		return "planned"
	}
}
