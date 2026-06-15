package repomap

import (
	"regexp"
	"sort"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

const maxSymbolsPerFile = 128

var (
	goPackagePattern = regexp.MustCompile(`^package\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	goTypePattern    = regexp.MustCompile(`^type\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	goFuncPattern    = regexp.MustCompile(`^func\s+(?:\(([^)]*)\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	goValuePattern   = regexp.MustCompile(`^(const|var)\s+([A-Za-z_][A-Za-z0-9_]*)\b`)

	pythonClassPattern = regexp.MustCompile(`^class\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	pythonFuncPattern  = regexp.MustCompile(`^(?:async\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

	jsClassPattern    = regexp.MustCompile(`^(?:export\s+default\s+|export\s+)?class\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)
	jsFunctionPattern = regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	jsArrowPattern    = regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][A-Za-z0-9_$]*)\s*=>`)
	tsTypePattern     = regexp.MustCompile(`^(?:export\s+)?(?:interface|type)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)

	javaTypePattern   = regexp.MustCompile(`^(?:(?:public|protected|private|abstract|final|static)\s+)*(class|interface|enum|record)\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	javaMethodPattern = regexp.MustCompile(`^(?:(?:public|protected|private|static|final|abstract|synchronized|native)\s+)+[A-Za-z_][A-Za-z0-9_<>\[\].?,\s]*\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

	rustTypePattern = regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?(?:struct|enum|trait)\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	rustImplPattern = regexp.MustCompile(`^impl(?:\s*<[^>]+>)?\s+([A-Za-z_][A-Za-z0-9_:<>]*)\b`)
	rustFuncPattern = regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

	rubyContainerPattern = regexp.MustCompile(`^(class|module)\s+([A-Za-z_][A-Za-z0-9_:]*)\b`)
	rubyMethodPattern    = regexp.MustCompile(`^def\s+(?:self\.)?([A-Za-z_][A-Za-z0-9_!?=]*)\b`)
)

func ExtractSymbols(path, language, content string) []model.SymbolRecord {
	switch language {
	case "go":
		return extractGoSymbols(path, language, content)
	case "python":
		return extractPythonSymbols(path, language, content)
	case "javascript":
		return extractJavaScriptSymbols(path, language, content, false)
	case "typescript":
		return extractJavaScriptSymbols(path, language, content, true)
	case "java":
		return extractJavaSymbols(path, language, content)
	case "rust":
		return extractRustSymbols(path, language, content)
	case "ruby":
		return extractRubySymbols(path, language, content)
	default:
		return nil
	}
}

func extractGoSymbols(path, language, content string) []model.SymbolRecord {
	lines := splitSourceLines(content)
	var records []model.SymbolRecord
	container := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case goPackagePattern.MatchString(trimmed):
			match := goPackagePattern.FindStringSubmatch(trimmed)
			container = match[1]
			records = appendSymbol(records, path, language, "package", match[1], "", "", i+1)
		case goTypePattern.MatchString(trimmed):
			match := goTypePattern.FindStringSubmatch(trimmed)
			records = appendSymbol(records, path, language, "type", match[1], "", container, i+1)
		case goFuncPattern.MatchString(trimmed):
			match := goFuncPattern.FindStringSubmatch(trimmed)
			receiver := cleanGoReceiver(match[1])
			kind := "func"
			if receiver != "" {
				kind = "method"
			}
			records = appendSymbol(records, path, language, kind, match[2], receiver, container, i+1)
		case goValuePattern.MatchString(trimmed):
			match := goValuePattern.FindStringSubmatch(trimmed)
			records = appendSymbol(records, path, language, match[1], match[2], "", container, i+1)
		}
		if len(records) >= maxSymbolsPerFile {
			break
		}
	}
	return sortSymbols(records)
}

func extractPythonSymbols(path, language, content string) []model.SymbolRecord {
	lines := splitSourceLines(content)
	var records []model.SymbolRecord
	var containers []indentedContainer
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := leadingSpaces(line)
		containers = trimContainers(containers, indent)
		container := currentContainer(containers)
		if match := pythonClassPattern.FindStringSubmatch(trimmed); match != nil {
			records = appendSymbol(records, path, language, "class", match[1], "", container, i+1)
			containers = append(containers, indentedContainer{indent: indent, name: match[1]})
		} else if match := pythonFuncPattern.FindStringSubmatch(trimmed); match != nil {
			kind := "func"
			if container != "" {
				kind = "method"
			}
			records = appendSymbol(records, path, language, kind, match[1], "", container, i+1)
		}
		if len(records) >= maxSymbolsPerFile {
			break
		}
	}
	return sortSymbols(records)
}

func extractJavaScriptSymbols(path, language, content string, includeTypes bool) []model.SymbolRecord {
	lines := splitSourceLines(content)
	var records []model.SymbolRecord
	container := ""
	containerDepth := -1
	braceDepth := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if container != "" && braceDepth <= containerDepth {
			container = ""
			containerDepth = -1
		}
		if match := jsClassPattern.FindStringSubmatch(trimmed); match != nil {
			records = appendSymbol(records, path, language, "class", match[1], "", "", i+1)
			if netBraces(trimmed) > 0 {
				container = match[1]
				containerDepth = braceDepth
			}
		} else if match := jsFunctionPattern.FindStringSubmatch(trimmed); match != nil {
			records = appendSymbol(records, path, language, "func", match[1], "", container, i+1)
		} else if match := jsArrowPattern.FindStringSubmatch(trimmed); match != nil {
			records = appendSymbol(records, path, language, "func", match[1], "", container, i+1)
		} else if includeTypes {
			if match := tsTypePattern.FindStringSubmatch(trimmed); match != nil {
				records = appendSymbol(records, path, language, "type", match[1], "", container, i+1)
			}
		}
		if len(records) >= maxSymbolsPerFile {
			break
		}
		braceDepth += netBraces(trimmed)
		if braceDepth < 0 {
			braceDepth = 0
		}
	}
	return sortSymbols(records)
}

func extractJavaSymbols(path, language, content string) []model.SymbolRecord {
	lines := splitSourceLines(content)
	var records []model.SymbolRecord
	container := ""
	containerDepth := -1
	braceDepth := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if container != "" && braceDepth <= containerDepth {
			container = ""
			containerDepth = -1
		}
		if match := javaTypePattern.FindStringSubmatch(trimmed); match != nil {
			records = appendSymbol(records, path, language, match[1], match[2], "", "", i+1)
			if netBraces(trimmed) > 0 {
				container = match[2]
				containerDepth = braceDepth
			}
		} else if match := javaMethodPattern.FindStringSubmatch(trimmed); match != nil {
			if !isControlKeyword(match[1]) {
				records = appendSymbol(records, path, language, "method", match[1], "", container, i+1)
			}
		}
		if len(records) >= maxSymbolsPerFile {
			break
		}
		braceDepth += netBraces(trimmed)
		if braceDepth < 0 {
			braceDepth = 0
		}
	}
	return sortSymbols(records)
}

func extractRustSymbols(path, language, content string) []model.SymbolRecord {
	lines := splitSourceLines(content)
	var records []model.SymbolRecord
	container := ""
	containerDepth := -1
	braceDepth := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if container != "" && braceDepth <= containerDepth {
			container = ""
			containerDepth = -1
		}
		if match := rustImplPattern.FindStringSubmatch(trimmed); match != nil {
			records = appendSymbol(records, path, language, "impl", match[1], "", "", i+1)
			if netBraces(trimmed) > 0 {
				container = match[1]
				containerDepth = braceDepth
			}
		} else if match := rustTypePattern.FindStringSubmatch(trimmed); match != nil {
			records = appendSymbol(records, path, language, "type", match[1], "", "", i+1)
		} else if match := rustFuncPattern.FindStringSubmatch(trimmed); match != nil {
			kind := "func"
			if container != "" {
				kind = "method"
			}
			records = appendSymbol(records, path, language, kind, match[1], "", container, i+1)
		}
		if len(records) >= maxSymbolsPerFile {
			break
		}
		braceDepth += netBraces(trimmed)
		if braceDepth < 0 {
			braceDepth = 0
		}
	}
	return sortSymbols(records)
}

func extractRubySymbols(path, language, content string) []model.SymbolRecord {
	lines := splitSourceLines(content)
	var records []model.SymbolRecord
	container := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if match := rubyContainerPattern.FindStringSubmatch(trimmed); match != nil {
			container = match[2]
			records = appendSymbol(records, path, language, match[1], match[2], "", "", i+1)
		} else if match := rubyMethodPattern.FindStringSubmatch(trimmed); match != nil {
			records = appendSymbol(records, path, language, "method", match[1], "", container, i+1)
		}
		if len(records) >= maxSymbolsPerFile {
			break
		}
	}
	return sortSymbols(records)
}

func appendSymbol(records []model.SymbolRecord, path, language, kind, name, receiver, container string, line int) []model.SymbolRecord {
	if len(records) >= maxSymbolsPerFile || name == "" {
		return records
	}
	return append(records, model.SymbolRecord{
		Path:      path,
		Language:  language,
		Kind:      kind,
		Name:      name,
		Receiver:  receiver,
		Container: container,
		StartLine: line,
		EndLine:   line,
	})
}

func sortSymbols(records []model.SymbolRecord) []model.SymbolRecord {
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].SortKey() < records[j].SortKey()
	})
	return records
}

func cleanGoReceiver(receiver string) string {
	receiver = strings.TrimSpace(receiver)
	if receiver == "" {
		return ""
	}
	fields := strings.Fields(receiver)
	receiverType := fields[len(fields)-1]
	receiverType = strings.TrimPrefix(receiverType, "*")
	if idx := strings.Index(receiverType, "["); idx >= 0 {
		receiverType = receiverType[:idx]
	}
	return receiverType
}

type indentedContainer struct {
	indent int
	name   string
}

func trimContainers(containers []indentedContainer, indent int) []indentedContainer {
	for len(containers) > 0 && indent <= containers[len(containers)-1].indent {
		containers = containers[:len(containers)-1]
	}
	return containers
}

func currentContainer(containers []indentedContainer) string {
	if len(containers) == 0 {
		return ""
	}
	return containers[len(containers)-1].name
}

func leadingSpaces(line string) int {
	count := 0
	for _, r := range line {
		switch r {
		case ' ':
			count++
		case '\t':
			count += 4
		default:
			return count
		}
	}
	return count
}

func isControlKeyword(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "catch":
		return true
	default:
		return false
	}
}

func netBraces(line string) int {
	return strings.Count(line, "{") - strings.Count(line, "}")
}
