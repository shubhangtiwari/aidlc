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
	allChunks := make([]model.SourceChunkRecord, 0, minInt(maxSourceChunksPerFile, len(lines)))
	for start := 0; start < len(lines); {
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
			allChunks = append(allChunks, model.SourceChunkRecord{
				Path:      path,
				Language:  language,
				StartLine: start + 1,
				EndLine:   end,
				Text:      text,
			})
		}
		start = end
	}
	return representativeSourceChunks(allChunks)
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

func representativeSourceChunks(chunks []model.SourceChunkRecord) []model.SourceChunkRecord {
	if len(chunks) <= maxSourceChunksPerFile {
		return chunks
	}
	selected := make([]model.SourceChunkRecord, 0, maxSourceChunksPerFile)
	seen := map[int]struct{}{}
	for i := 0; i < maxSourceChunksPerFile; i++ {
		index := 0
		if maxSourceChunksPerFile > 1 {
			index = i * (len(chunks) - 1) / (maxSourceChunksPerFile - 1)
		}
		if _, ok := seen[index]; ok {
			for index < len(chunks) {
				if _, exists := seen[index]; !exists {
					break
				}
				index++
			}
			if index >= len(chunks) {
				continue
			}
		}
		seen[index] = struct{}{}
		selected = append(selected, chunks[index])
	}
	return selected
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
