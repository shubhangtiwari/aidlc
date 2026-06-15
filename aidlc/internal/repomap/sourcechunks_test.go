package repomap

import (
	"fmt"
	"strings"
	"testing"
)

func TestExtractSourceChunksUsesBoundedSourceBlocks(t *testing.T) {
	content := strings.Join([]string{
		"package auth",
		"",
		"type TokenClaims struct {",
		"\tSubject string",
		"}",
		"",
		"func AuthorizeToken(raw string) TokenClaims {",
		"\treturn TokenClaims{Subject: strings.TrimSpace(raw)}",
		"}",
		"",
	}, "\n")

	got := ExtractSourceChunks("internal/auth/auth.go", "go", content)

	if len(got) != 3 {
		t.Fatalf("len(ExtractSourceChunks()) = %d, want 3: %#v", len(got), got)
	}
	if got[0].StartLine != 1 || got[0].EndLine != 1 || got[0].Text != "package auth" {
		t.Fatalf("first chunk = %#v", got[0])
	}
	if got[1].StartLine != 3 || got[1].EndLine != 5 || !strings.Contains(got[1].Text, "TokenClaims") {
		t.Fatalf("second chunk = %#v", got[1])
	}
	if got[2].StartLine != 7 || got[2].EndLine != 9 || !strings.Contains(got[2].Text, "AuthorizeToken") {
		t.Fatalf("third chunk = %#v", got[2])
	}
}

func TestExtractSourceChunksIsDeterministicAndBounded(t *testing.T) {
	var builder strings.Builder
	for i := 0; i < maxSourceChunksPerFile+4; i++ {
		fmt.Fprintf(&builder, "func Symbol%d() string {\n", i)
		fmt.Fprintf(&builder, "\treturn %q\n", strings.Repeat("x", maxSourceChunkRunes/2))
		fmt.Fprintf(&builder, "}\n\n")
	}
	content := strings.ReplaceAll(builder.String(), "\n", "\r\n")

	first := ExtractSourceChunks("pkg/util/util.go", "go", content)
	second := ExtractSourceChunks("pkg/util/util.go", "go", content)

	if len(first) != maxSourceChunksPerFile {
		t.Fatalf("len(ExtractSourceChunks()) = %d, want %d", len(first), maxSourceChunksPerFile)
	}
	if fmt.Sprintf("%#v", first) != fmt.Sprintf("%#v", second) {
		t.Fatalf("ExtractSourceChunks() is not deterministic:\nfirst=%#v\nsecond=%#v", first, second)
	}
	for _, chunk := range first {
		if lines := chunk.EndLine - chunk.StartLine + 1; lines > maxSourceChunkLines {
			t.Fatalf("chunk has %d lines, want <= %d: %#v", lines, maxSourceChunkLines, chunk)
		}
		if runes := len([]rune(chunk.Text)); runes > maxSourceChunkRunes {
			t.Fatalf("chunk has %d runes, want <= %d: %#v", runes, maxSourceChunkRunes, chunk)
		}
	}
}

func TestExtractSourceChunksDoesNotSkipLinesWhenSplittingDenseBlocks(t *testing.T) {
	var builder strings.Builder
	for i := 0; i < maxSourceChunkLines+2; i++ {
		fmt.Fprintf(&builder, "const Symbol%d = %d\n", i, i)
	}

	got := ExtractSourceChunks("internal/core/core.go", "go", builder.String())

	if len(got) != 2 {
		t.Fatalf("len(ExtractSourceChunks()) = %d, want 2: %#v", len(got), got)
	}
	if !strings.Contains(got[1].Text, fmt.Sprintf("Symbol%d", maxSourceChunkLines)) {
		t.Fatalf("second chunk skipped capped line: %#v", got[1])
	}
}

func TestExtractSourceChunksSkipsNonCodeLanguages(t *testing.T) {
	for _, language := range []string{"markdown", "text", ""} {
		t.Run(language, func(t *testing.T) {
			if got := ExtractSourceChunks("README.md", language, "# Title\n"); len(got) != 0 {
				t.Fatalf("ExtractSourceChunks() = %#v, want none", got)
			}
		})
	}
}
