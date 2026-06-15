package repomap

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

var headingPattern = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+?)\s*$`)

func ScanDoc(path, content string) ([]model.DocRecord, []model.ChangeRecord, []model.BlueprintRecord) {
	if DetectLanguage(path) != "markdown" {
		return nil, nil, nil
	}

	kind := docKind(path)
	if kind == "" {
		return nil, nil, nil
	}

	title := firstHeading(content)
	docs := []model.DocRecord{{
		Path:  path,
		Kind:  kind,
		Title: title,
		Text:  normalizedText(content),
	}}

	var changes []model.ChangeRecord
	if kind == "spec" || kind == "adr" {
		changes = append(changes, model.ChangeRecord{
			Path:   path,
			Kind:   kind,
			ID:     strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
			Title:  title,
			Status: frontMatterValue(content, "status"),
			Text:   normalizedText(content),
		})
	}

	var blueprints []model.BlueprintRecord
	if kind == "blueprint" {
		module := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		for _, section := range markdownSections(content) {
			blueprints = append(blueprints, model.BlueprintRecord{
				Path:    path,
				Module:  module,
				Section: section.title,
				Text:    section.text,
			})
		}
	}

	return docs, changes, blueprints
}

func docKind(path string) string {
	switch {
	case path == "docs/ARCHITECTURE.md" || strings.HasPrefix(path, "docs/architecture/"):
		return "architecture"
	case strings.HasPrefix(path, "docs/spec/") && path != "docs/spec/README.md":
		return "spec"
	case strings.HasPrefix(path, "docs/blueprints/") && path != "docs/blueprints/README.md":
		return "blueprint"
	case strings.HasPrefix(path, "docs/adr/") && path != "docs/adr/README.md":
		return "adr"
	default:
		return ""
	}
}

func firstHeading(content string) string {
	for _, match := range headingPattern.FindAllStringSubmatch(content, -1) {
		return strings.TrimSpace(match[2])
	}
	return ""
}

func frontMatterValue(content, key string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	prefix := key + ":"
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			return ""
		}
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), prefix)), `"'`)
		}
	}
	return ""
}

type markdownSection struct {
	title string
	text  string
}

func markdownSections(content string) []markdownSection {
	matches := headingPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}

	sections := make([]markdownSection, 0, len(matches))
	for i, match := range matches {
		title := strings.TrimSpace(content[match[4]:match[5]])
		start := match[1]
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		sections = append(sections, markdownSection{
			title: title,
			text:  normalizedText(content[start:end]),
		})
	}
	return sections
}

func normalizedText(content string) string {
	return strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
}
