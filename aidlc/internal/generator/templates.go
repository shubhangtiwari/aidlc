package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func loadSourceData(root string) (sourceData, error) {
	facts, err := DetectProjectFacts(root)
	if err != nil {
		return sourceData{}, err
	}
	personas, err := loadDocuments(filepath.Join(root, ".ai", "personas"))
	if err != nil {
		return sourceData{}, err
	}
	skills, err := loadDocuments(filepath.Join(root, ".ai", "skills"))
	if err != nil {
		return sourceData{}, err
	}
	sharedBody, err := loadSharedBody(filepath.Join(root, ".ai", "README.md"))
	if err != nil {
		return sourceData{}, err
	}
	defaults, err := loadModelDefaults(filepath.Join(root, ".ai", "models.defaults.toml"))
	if err != nil {
		return sourceData{}, err
	}
	return sourceData{
		Facts:         facts,
		Personas:      personas,
		Skills:        skills,
		ModelDefaults: defaults,
		SharedBody:    sharedBody,
	}, nil
}

func loadDocuments(dir string) ([]document, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("%s: no markdown documents found", dir)
	}
	docs := make([]document, 0, len(names))
	for _, name := range names {
		doc, err := loadDocument(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func loadDocument(path string) (document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return document{}, err
	}
	frontmatter, body, err := splitFrontmatter(string(content))
	if err != nil {
		return document{}, fmt.Errorf("%s: %w", path, err)
	}
	values := parseSimpleYAML(frontmatter)
	name := values["name"]
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	description := values["description"]
	if description == "" {
		return document{}, fmt.Errorf("%s: frontmatter missing required 'description'", path)
	}
	return document{Name: name, Description: description, Body: stripBlankEdges(body), Raw: content}, nil
}

func splitFrontmatter(content string) (string, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", "", fmt.Errorf("missing YAML frontmatter")
	}
	rest := strings.TrimPrefix(content, "---\n")
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", "", fmt.Errorf("unterminated frontmatter")
	}
	frontmatter := rest[:idx]
	body := rest[idx+len("\n---"):]
	body = strings.TrimPrefix(body, "\n")
	return frontmatter, body, nil
}

func parseSimpleYAML(content string) map[string]string {
	out := map[string]string{}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = stripYAMLString(strings.TrimSpace(value))
	}
	return out
}

func stripYAMLString(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '\'' || first == '"') && first == last {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func loadSharedBody(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	found := false
	started := false
	inComment := false
	var out []string
	for _, line := range lines {
		if !found {
			if strings.Contains(line, readmeInitMarker) {
				found = true
			}
			continue
		}
		if !started && line == "" {
			continue
		}
		if !started && strings.HasPrefix(line, "<!--") {
			inComment = !strings.Contains(line, "-->")
			continue
		}
		if inComment {
			if strings.Contains(line, "-->") {
				inComment = false
			}
			continue
		}
		if !started && line == "" {
			continue
		}
		started = true
		out = append(out, line)
	}
	if !found {
		return "", fmt.Errorf("%s: missing %s", path, readmeInitMarker)
	}
	return strings.Join(out, "\n"), nil
}

func loadModelDefaults(path string) (map[string]map[string]modelDefault, error) {
	if !exists(path) {
		return nil, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	defaults := map[string]map[string]modelDefault{}
	section := ""
	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(stripInlineComment(raw))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || section == "" {
			continue
		}
		parts := strings.Split(section, ".")
		if len(parts) != 2 {
			continue
		}
		ide, persona := parts[0], parts[1]
		if defaults[ide] == nil {
			defaults[ide] = map[string]modelDefault{}
		}
		def := defaults[ide][persona]
		switch strings.TrimSpace(key) {
		case "model":
			def.Model = tomlScalar(value)
		case "reasoning":
			def.Reasoning = tomlScalar(value)
		}
		defaults[ide][persona] = def
	}
	return defaults, nil
}

func stripBlankEdges(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[start:end], "\n")
}
