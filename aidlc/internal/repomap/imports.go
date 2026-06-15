package repomap

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

var (
	goImportBlockPattern = regexp.MustCompile(`(?ms)import\s*\((.*?)\)`)
	goImportPattern      = regexp.MustCompile(`(?m)^\s*(?:import\s+)?(?:[._A-Za-z0-9]+\s+)?"([^"]+)"`)
	pyImportPattern      = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z0-9_.,\s]+)`)
	pyFromPattern        = regexp.MustCompile(`(?m)^\s*from\s+([A-Za-z0-9_\.]+)\s+import\s+`)
	jsImportPattern      = regexp.MustCompile(`(?m)\bimport\s+(?:[^'"]+\s+from\s+)?["']([^"']+)["']`)
	jsExportPattern      = regexp.MustCompile(`(?m)\bexport\s+[^'"]+\s+from\s+["']([^"']+)["']`)
	jsRequirePattern     = regexp.MustCompile(`(?m)\brequire\s*\(\s*["']([^"']+)["']\s*\)`)
	javaImportPattern    = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([A-Za-z0-9_.*]+)\s*;`)
	rustUsePattern       = regexp.MustCompile(`(?m)^\s*use\s+([^;]+);`)
	rustExternPattern    = regexp.MustCompile(`(?m)^\s*extern\s+crate\s+([A-Za-z0-9_]+)\s*;`)
	rubyRequirePattern   = regexp.MustCompile(`(?m)^\s*require(?:_relative)?\s+["']([^"']+)["']`)
)

func DetectLanguage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".md", ".markdown":
		return "markdown"
	case ".mod":
		if filepath.Base(path) == "go.mod" {
			return "go-mod"
		}
	}
	return "text"
}

func ExtractImports(path, language, content string) []model.ImportRecord {
	seen := map[string]bool{}
	add := func(importPath string) {
		importPath = strings.TrimSpace(importPath)
		importPath = strings.Trim(importPath, "`")
		if importPath != "" {
			seen[importPath] = true
		}
	}

	switch language {
	case "go":
		for _, match := range goImportBlockPattern.FindAllStringSubmatch(content, -1) {
			for _, lineMatch := range goImportPattern.FindAllStringSubmatch(match[1], -1) {
				add(lineMatch[1])
			}
		}
		withoutBlocks := goImportBlockPattern.ReplaceAllString(content, "")
		for _, match := range goImportPattern.FindAllStringSubmatch(withoutBlocks, -1) {
			add(match[1])
		}
	case "python":
		for _, match := range pyImportPattern.FindAllStringSubmatch(content, -1) {
			for _, part := range strings.Split(match[1], ",") {
				fields := strings.Fields(part)
				if len(fields) > 0 {
					add(fields[0])
				}
			}
		}
		for _, match := range pyFromPattern.FindAllStringSubmatch(content, -1) {
			add(match[1])
		}
	case "javascript", "typescript":
		for _, pattern := range []*regexp.Regexp{jsImportPattern, jsExportPattern, jsRequirePattern} {
			for _, match := range pattern.FindAllStringSubmatch(content, -1) {
				add(match[1])
			}
		}
	case "java":
		for _, match := range javaImportPattern.FindAllStringSubmatch(content, -1) {
			add(match[1])
		}
	case "rust":
		for _, match := range rustUsePattern.FindAllStringSubmatch(content, -1) {
			add(match[1])
		}
		for _, match := range rustExternPattern.FindAllStringSubmatch(content, -1) {
			add(match[1])
		}
	case "ruby":
		for _, match := range rubyRequirePattern.FindAllStringSubmatch(content, -1) {
			add(match[1])
		}
	}

	imports := make([]model.ImportRecord, 0, len(seen))
	for importPath := range seen {
		imports = append(imports, model.ImportRecord{Path: path, Language: language, ImportPath: importPath})
	}
	sort.Slice(imports, func(i, j int) bool {
		return imports[i].SortKey() < imports[j].SortKey()
	})
	return imports
}
