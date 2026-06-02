package payload

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var windowsDrivePattern = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

var privatePathPatterns = []string{
	"docs/spec/[0-9]*-*.md",
	"docs/adr/[0-9]*-*.md",
	"docs/blueprints/aidlc.md",
	"docs/blueprints/template-payload.md",
	"docs/ARCHITECTURE.md",
	"docs/architecture/**",
	"aidlc/**",
	".github/**",
	"release/**",
	"dist/**",
	"build/**",
	"aidlc/scripts/**",
}

func PrivatePathPatterns() []string {
	out := make([]string, len(privatePathPatterns))
	copy(out, privatePathPatterns)
	return out
}

func NormalizeRelativePath(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(value) || strings.HasPrefix(value, "/") || windowsDrivePattern.MatchString(value) {
		return "", fmt.Errorf("path %q must be relative", value)
	}

	normalized := path.Clean(strings.ReplaceAll(value, "\\", "/"))
	if normalized == "." || normalized == "" {
		return "", fmt.Errorf("path %q is empty after normalization", value)
	}
	if normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", fmt.Errorf("path %q escapes the payload root", value)
	}
	return normalized, nil
}

func IsPrivatePath(value string) bool {
	normalized, err := NormalizeRelativePath(value)
	if err != nil {
		return true
	}
	for _, pattern := range privatePathPatterns {
		if matchPattern(pattern, normalized) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, value string) bool {
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "**")
		return strings.HasPrefix(value, prefix)
	}
	matched, err := path.Match(pattern, value)
	return err == nil && matched
}
