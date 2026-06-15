package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

type BinaryDiscoveryRequest struct {
	ExplicitPath    string
	PathEnv         string
	HomeDir         string
	LocalAppDataDir string
	GoOS            string
	Executable      func(string) bool
}

func DiscoverBinary(req BinaryDiscoveryRequest) contract.AIDLCBinaryDiscovery {
	checkExecutable := req.Executable
	if checkExecutable == nil {
		checkExecutable = func(path string) bool {
			return isExecutableFile(path, req.GoOS)
		}
	}

	var result contract.AIDLCBinaryDiscovery
	addCandidate := func(path string, source contract.AIDLCBinarySource) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		candidate := contract.AIDLCBinaryCandidate{
			Path:       path,
			Source:     source,
			Executable: checkExecutable(path),
		}
		result.Candidates = append(result.Candidates, candidate)
		if !result.Found && candidate.Executable {
			result.Path = candidate.Path
			result.Source = candidate.Source
			result.Found = true
		}
	}

	if strings.TrimSpace(req.ExplicitPath) != "" {
		addCandidate(req.ExplicitPath, contract.AIDLCBinarySourceExplicit)
		return result
	}

	goos := req.GoOS
	if goos == "" {
		goos = runtime.GOOS
	}
	for _, dir := range splitPathEnv(req.PathEnv, goos) {
		for _, binary := range binaryNames(goos) {
			addCandidate(filepath.Join(dir, binary), contract.AIDLCBinarySourcePATH)
		}
	}
	for _, path := range commonBinaryLocations(req.HomeDir, req.LocalAppDataDir, goos) {
		addCandidate(path, contract.AIDLCBinarySourceCommonLocation)
	}
	return result
}

func CommonBinaryLocations(homeDir, goos string) []string {
	return commonBinaryLocations(homeDir, "", goos)
}

func commonBinaryLocations(homeDir, localAppDataDir, goos string) []string {
	if goos == "" {
		goos = runtime.GOOS
	}
	dirs := []string{}
	if goos == "windows" {
		if strings.TrimSpace(localAppDataDir) == "" && strings.TrimSpace(homeDir) != "" {
			localAppDataDir = filepath.Join(homeDir, "AppData", "Local")
		}
		if strings.TrimSpace(localAppDataDir) != "" {
			dirs = append(dirs, filepath.Join(localAppDataDir, "Programs", "aidlc", "bin"))
		}
	}
	if strings.TrimSpace(homeDir) != "" {
		dirs = append(dirs, filepath.Join(homeDir, ".local", "bin"), filepath.Join(homeDir, "bin"))
	}
	if goos != "windows" {
		dirs = append(dirs, "/opt/homebrew/bin", "/usr/local/bin")
	}

	locations := make([]string, 0, len(dirs)*len(binaryNames(goos)))
	for _, dir := range dirs {
		for _, binary := range binaryNames(goos) {
			locations = append(locations, filepath.Join(dir, binary))
		}
	}
	return locations
}

func splitPathEnv(pathEnv, goos string) []string {
	separator := ":"
	if goos == "windows" {
		separator = ";"
	}
	parts := strings.Split(pathEnv, separator)
	dirs := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			dirs = append(dirs, part)
		}
	}
	return dirs
}

func binaryNames(goos string) []string {
	if goos == "windows" {
		return []string{"aidlc.exe", "aidlc"}
	}
	return []string{"aidlc"}
}

func isExecutableFile(path, goos string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}
