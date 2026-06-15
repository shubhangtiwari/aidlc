package repomap

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type ScanOptions struct {
	Include []string
}

var invalidIncludeRoots = map[string]bool{
	".cache":        true,
	".claude":       true,
	".codex":        true,
	".cursor":       true,
	".git":          true,
	".idea":         true,
	".mypy_cache":   true,
	".pytest_cache": true,
	".ruff_cache":   true,
	".tox":          true,
	".venv":         true,
	".vscode":       true,
	"build":         true,
	"cache":         true,
	"dist":          true,
	"env":           true,
	"node_modules":  true,
	"out":           true,
	"target":        true,
	"vendor":        true,
	"venv":          true,
}

func NormalizeInclude(include []string) ([]string, error) {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(include))
	for _, item := range include {
		clean, err := NormalizeIncludePath(item)
		if err != nil {
			return nil, err
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		normalized = append(normalized, clean)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func NormalizeIncludePath(item string) (string, error) {
	value := strings.TrimSpace(strings.ReplaceAll(item, "\\", "/"))
	if value == "" {
		return "", fmt.Errorf("map include path is empty")
	}
	if strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("map include path %q must be relative", item)
	}
	clean := path.Clean(value)
	if clean == "." || clean == "" {
		return "", fmt.Errorf("map include path %q must name a directory", item)
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("map include path %q must not escape the map root", item)
	}
	if clean == model.MapDir || strings.HasPrefix(clean, model.MapDir+"/") {
		return "", fmt.Errorf("map include path %q must not include generated map artifacts", item)
	}
	root, _, _ := strings.Cut(clean, "/")
	if invalidIncludeRoots[root] {
		return "", fmt.Errorf("map include path %q must not include generated or dependency directories", item)
	}
	return clean, nil
}
