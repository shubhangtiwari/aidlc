package repomap

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxSnippetRunes = 240

type snippetSource struct {
	StartLine int
	EndLine   int
	Text      string
}

func CompactSnippet(text string) string {
	return compactSnippet(snippetSource{Text: text})
}

func compactSnippet(source snippetSource) string {
	text := strings.Join(strings.Fields(source.Text), " ")
	if text == "" {
		return ""
	}
	if source.StartLine > 0 {
		line := fmt.Sprintf("L%d", source.StartLine)
		if source.EndLine > source.StartLine {
			line = fmt.Sprintf("L%d-L%d", source.StartLine, source.EndLine)
		}
		text = line + " " + text
	}
	if utf8.RuneCountInString(text) <= maxSnippetRunes {
		return text
	}
	runes := []rune(text)
	text = strings.TrimSpace(string(runes[:maxSnippetRunes-1]))
	return text + "..."
}
