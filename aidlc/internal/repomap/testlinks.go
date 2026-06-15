package repomap

import (
	"path/filepath"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

func LinkTests(files []model.FileRecord) []model.TestRecord {
	sourcePaths := map[string]string{}
	for _, file := range files {
		if !isTestPath(file.Path, file.Language) {
			sourcePaths[file.Path] = file.Language
		}
	}

	var tests []model.TestRecord
	for _, file := range files {
		if !isTestPath(file.Path, file.Language) {
			continue
		}
		for _, candidate := range testTargetCandidates(file.Path, file.Language) {
			if _, ok := sourcePaths[candidate]; ok {
				tests = append(tests, model.TestRecord{
					Path:       file.Path,
					Language:   file.Language,
					TargetPath: candidate,
				})
				break
			}
		}
	}
	return tests
}

func isTestPath(path, language string) bool {
	base := filepath.Base(path)
	switch language {
	case "go":
		return strings.HasSuffix(base, "_test.go")
	case "python":
		return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")
	case "javascript":
		return strings.HasSuffix(base, ".test.js") || strings.HasSuffix(base, ".spec.js")
	case "typescript":
		return strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".spec.ts") ||
			strings.HasSuffix(base, ".test.tsx") || strings.HasSuffix(base, ".spec.tsx")
	case "java":
		return strings.HasSuffix(base, "Test.java")
	case "rust":
		return strings.HasSuffix(base, "_test.rs")
	case "ruby":
		return strings.HasSuffix(base, "_test.rb") || strings.HasSuffix(base, "_spec.rb")
	default:
		return false
	}
}

func testTargetCandidates(path, language string) []string {
	dir := filepath.ToSlash(filepath.Dir(path))
	if dir == "." {
		dir = ""
	}
	join := func(name string) string {
		if dir == "" {
			return name
		}
		return dir + "/" + name
	}
	base := filepath.Base(path)

	switch language {
	case "go":
		return []string{join(strings.TrimSuffix(base, "_test.go") + ".go")}
	case "python":
		if strings.HasPrefix(base, "test_") {
			return []string{join(strings.TrimPrefix(base, "test_"))}
		}
		return []string{join(strings.TrimSuffix(base, "_test.py") + ".py")}
	case "javascript":
		return []string{
			join(strings.TrimSuffix(strings.TrimSuffix(base, ".test.js"), ".spec.js") + ".js"),
			join(strings.TrimSuffix(strings.TrimSuffix(base, ".test.js"), ".spec.js") + ".jsx"),
		}
	case "typescript":
		stem := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(base, ".test.tsx"), ".spec.tsx"), ".test.ts"), ".spec.ts")
		return []string{join(stem + ".ts"), join(stem + ".tsx")}
	case "java":
		return []string{join(strings.TrimSuffix(base, "Test.java") + ".java")}
	case "rust":
		return []string{join(strings.TrimSuffix(base, "_test.rs") + ".rs")}
	case "ruby":
		return []string{join(strings.TrimSuffix(strings.TrimSuffix(base, "_test.rb"), "_spec.rb") + ".rb")}
	default:
		return nil
	}
}
