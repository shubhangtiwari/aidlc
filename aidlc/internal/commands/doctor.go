package commands

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/install"
)

type DoctorOptions struct {
	Dir string
}

type DoctorDependencies struct {
	ExecutablePath string
	Version        string
	PathEnv        string
	HomeDir        string
	GoOS           string
	Executable     func(string) bool
	Stat           func(string) (os.FileInfo, error)
	ReadFile       func(string) ([]byte, error)
}

type DoctorResult struct {
	Version        string
	ExecutablePath string
	Discovery      contract.AIDLCBinaryDiscovery
	RepoDir        string
	MakeHelper     MakeHelperStatus
	Findings       []string
}

type MakeHelperStatus struct {
	HelperPath       string
	HelperPresent    bool
	MakefilePath     string
	MakefilePresent  bool
	MakefileIncludes bool
}

func RunDoctorCLI(args []string, stdout, stderr io.Writer) int {
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

	result, err := RunDoctor(opts, DoctorDependencies{})
	if err != nil {
		fmt.Fprintf(stderr, "aidlc doctor: %v\n", err)
		return contract.ExitUsage
	}
	printDoctorResult(stdout, result)
	if len(result.Findings) > 0 {
		return contract.ExitConflict
	}
	return contract.ExitOK
}

func newDoctorFlagSet(opts *DoctorOptions, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("aidlc doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.Dir, "dir", opts.Dir, "repository directory to inspect")
	fs.Usage = func() { printDoctorUsage(stderr) }
	return fs
}

func RunDoctor(opts DoctorOptions, deps DoctorDependencies) (DoctorResult, error) {
	deps = doctorDependenciesWithDefaults(deps)
	dir := strings.TrimSpace(opts.Dir)
	if dir == "" {
		return DoctorResult{}, fmt.Errorf("--dir must not be empty")
	}
	repoDir, err := filepath.Abs(dir)
	if err != nil {
		return DoctorResult{}, err
	}
	info, err := deps.Stat(repoDir)
	if err != nil {
		return DoctorResult{}, fmt.Errorf("inspect --dir: %w", err)
	}
	if !info.IsDir() {
		return DoctorResult{}, fmt.Errorf("--dir is not a directory: %s", repoDir)
	}

	result := DoctorResult{
		Version:        deps.Version,
		ExecutablePath: deps.ExecutablePath,
		Discovery: install.DiscoverBinary(install.BinaryDiscoveryRequest{
			PathEnv:    deps.PathEnv,
			HomeDir:    deps.HomeDir,
			GoOS:       deps.GoOS,
			Executable: deps.Executable,
		}),
		RepoDir:    repoDir,
		MakeHelper: inspectMakeHelper(repoDir, deps),
	}
	if !pathDiscoverable(result.Discovery) {
		result.Findings = append(result.Findings, "aidlc is not discoverable through PATH in this process")
	}
	if !result.MakeHelper.HelperPresent {
		result.Findings = append(result.Findings, ".ai/Makefile.inc is missing from the selected directory")
	}
	if result.MakeHelper.HelperPresent && !result.MakeHelper.MakefilePresent {
		result.Findings = append(result.Findings, "root Makefile is missing, so Make helper targets are not exposed")
	}
	if result.MakeHelper.MakefilePresent && !result.MakeHelper.MakefileIncludes {
		result.Findings = append(result.Findings, "root Makefile does not include .ai/Makefile.inc")
	}
	return result, nil
}

func doctorDependenciesWithDefaults(deps DoctorDependencies) DoctorDependencies {
	if deps.ExecutablePath == "" {
		if path, err := os.Executable(); err == nil {
			deps.ExecutablePath = path
		} else {
			deps.ExecutablePath = "unknown"
		}
	}
	if deps.Version == "" {
		deps.Version = CurrentVersion()
	}
	if deps.PathEnv == "" {
		deps.PathEnv = os.Getenv("PATH")
	}
	if deps.HomeDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			deps.HomeDir = home
		}
	}
	if deps.GoOS == "" {
		deps.GoOS = runtime.GOOS
	}
	if deps.Stat == nil {
		deps.Stat = os.Stat
	}
	if deps.ReadFile == nil {
		deps.ReadFile = os.ReadFile
	}
	return deps
}

func inspectMakeHelper(repoDir string, deps DoctorDependencies) MakeHelperStatus {
	status := MakeHelperStatus{
		HelperPath:   filepath.Join(repoDir, ".ai", "Makefile.inc"),
		MakefilePath: filepath.Join(repoDir, "Makefile"),
	}
	if info, err := deps.Stat(status.HelperPath); err == nil && !info.IsDir() {
		status.HelperPresent = true
	}
	if info, err := deps.Stat(status.MakefilePath); err == nil && !info.IsDir() {
		status.MakefilePresent = true
		data, err := deps.ReadFile(status.MakefilePath)
		status.MakefileIncludes = err == nil && makefileIncludesAIDLC(data)
	}
	return status
}

func makefileIncludesAIDLC(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if (fields[0] == "include" || fields[0] == "-include" || fields[0] == "sinclude") && fields[1] == ".ai/Makefile.inc" {
			return true
		}
	}
	return false
}

func pathDiscoverable(discovery contract.AIDLCBinaryDiscovery) bool {
	for _, candidate := range discovery.Candidates {
		if candidate.Source == contract.AIDLCBinarySourcePATH && candidate.Executable {
			return true
		}
	}
	return false
}

func printDoctorResult(w io.Writer, result DoctorResult) {
	fmt.Fprintf(w, "aidlc doctor\n")
	fmt.Fprintf(w, "version: %s\n", result.Version)
	fmt.Fprintf(w, "executable: %s\n", result.ExecutablePath)
	fmt.Fprintf(w, "repository: %s\n", result.RepoDir)
	printPathDiscovery(w, result.Discovery)
	printMakeHelperStatus(w, result.MakeHelper)
	printDoctorFindings(w, result.Findings, result.ExecutablePath)
}

func printPathDiscovery(w io.Writer, discovery contract.AIDLCBinaryDiscovery) {
	if pathDiscoverable(discovery) {
		fmt.Fprintln(w, "PATH discoverable: yes")
	} else {
		fmt.Fprintln(w, "PATH discoverable: no")
	}
	fmt.Fprintln(w, "install candidates:")
	for _, candidate := range discovery.Candidates {
		if candidate.Source != contract.AIDLCBinarySourceCommonLocation {
			continue
		}
		state := "missing"
		if candidate.Executable {
			state = "executable"
		}
		fmt.Fprintf(w, "  - %s [%s]\n", candidate.Path, state)
	}
}

func printMakeHelperStatus(w io.Writer, status MakeHelperStatus) {
	fmt.Fprintln(w, "Make helper:")
	fmt.Fprintf(w, "  .ai/Makefile.inc: %s\n", presentStatus(status.HelperPresent, status.HelperPath))
	if status.MakefilePresent {
		includeStatus := "missing include"
		if status.MakefileIncludes {
			includeStatus = "includes .ai/Makefile.inc"
		}
		fmt.Fprintf(w, "  Makefile: %s (%s)\n", status.MakefilePath, includeStatus)
		return
	}
	fmt.Fprintf(w, "  Makefile: missing (%s)\n", status.MakefilePath)
}

func printDoctorFindings(w io.Writer, findings []string, executablePath string) {
	if len(findings) == 0 {
		fmt.Fprintln(w, "status: healthy")
		fmt.Fprintln(w, "next steps: none")
		return
	}
	fmt.Fprintln(w, "status: action needed")
	fmt.Fprintln(w, "findings:")
	for _, finding := range findings {
		fmt.Fprintf(w, "  - %s\n", finding)
	}
	fmt.Fprintln(w, "next steps:")
	fmt.Fprintln(w, "  - For sanitized IDE shells or CI, set AIDLC_BIN to the executable path above before running Make helpers.")
	if executablePath != "" && executablePath != "unknown" {
		fmt.Fprintf(w, "  - Example: AIDLC_BIN=%s make ai-doctor\n", executablePath)
	}
	fmt.Fprintln(w, "  - Ensure the install directory is on PATH, or rerun the installer with AIDLC_INSTALL_DIR set to a PATH directory.")
	fmt.Fprintln(w, "  - Ensure the repository root Makefile includes: -include .ai/Makefile.inc")
}

func presentStatus(present bool, path string) string {
	if present {
		return path
	}
	return "missing (" + path + ")"
}

func printDoctorUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc doctor [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --dir DIR   Repository directory to inspect (default .)")
}
