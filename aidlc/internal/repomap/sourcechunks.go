package repomap

import (
	"strings"
	"unicode/utf8"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

const (
	maxSourceChunksPerFile = 12
	maxSourceChunkLines    = 24
	maxSourceChunkRunes    = 2400
)

var sourceChunkLanguages = map[string]struct{}{
	"go":         {},
	"python":     {},
	"javascript": {},
	"typescript": {},
	"java":       {},
	"rust":       {},
	"ruby":       {},
	"go-mod":     {},
}

func ExtractSourceChunks(path, language, content string) []model.SourceChunkRecord {
	if !isSourceChunkLanguage(language) {
		return nil
	}

	lines := splitSourceLines(content)
	chunks := make([]model.SourceChunkRecord, 0, minInt(maxSourceChunksPerFile, len(lines)))
	for start := 0; start < len(lines) && len(chunks) < maxSourceChunksPerFile; {
		for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
			start++
		}
		if start >= len(lines) {
			break
		}

		end := start
		runes := 0
		for end < len(lines) {
			if end > start && strings.TrimSpace(lines[end]) == "" {
				break
			}
			nextRunes := utf8.RuneCountInString(lines[end])
			if end > start && (end-start >= maxSourceChunkLines || runes+nextRunes > maxSourceChunkRunes) {
				break
			}
			runes += nextRunes
			end++
		}

		text := trimSourceChunkText(strings.Join(lines[start:end], "\n"))
		if text != "" {
			chunks = append(chunks, model.SourceChunkRecord{
				Path:      path,
				Language:  language,
				StartLine: start + 1,
				EndLine:   end,
				Text:      text,
			})
		}
		start = end
	}
	return chunks
}

func isSourceChunkLanguage(language string) bool {
	_, ok := sourceChunkLanguages[language]
	return ok
}

func splitSourceLines(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.TrimSuffix(normalized, "\n")
	if normalized == "" {
		return nil
	}
	return strings.Split(normalized, "\n")
}

func trimSourceChunkText(text string) string {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) <= maxSourceChunkRunes {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:maxSourceChunkRunes]))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
