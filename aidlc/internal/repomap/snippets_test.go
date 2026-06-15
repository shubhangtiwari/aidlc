package repomap

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCompactSnippetIsOneLineLineAwareAndBounded(t *testing.T) {
	t.Parallel()

	text := strings.Repeat("token validation\n", 40)
	got := compactSnippet(snippetSource{StartLine: 12, EndLine: 14, Text: text})
	if strings.Contains(got, "\n") {
		t.Fatalf("snippet contains newline: %q", got)
	}
	if !strings.HasPrefix(got, "L12-L14 ") {
		t.Fatalf("snippet = %q, want line prefix", got)
	}
	if utf8.RuneCountInString(got) > maxSnippetRunes+2 {
		t.Fatalf("snippet length = %d, want bounded", utf8.RuneCountInString(got))
	}
}

func TestCompactSnippetNormalizesWhitespace(t *testing.T) {
	t.Parallel()

	got := CompactSnippet(" auth\n\n\t service   token ")
	want := "auth service token"
	if got != want {
		t.Fatalf("CompactSnippet() = %q, want %q", got, want)
	}
}
