package repomap

import (
	"fmt"
	"strings"
	"testing"
)

func TestExtractSymbolsCoversSupportedLanguages(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
		want    []symbolExpectation
	}{
		{
			name: "go",
			path: "internal/auth/auth.go",
			content: `package auth

type Service struct{}
const DefaultRole = "reader"
var DefaultService = Service{}

func Authorize(name string) string {
	return name
}

func (s *Service) Normalize(name string) string {
	return name
}
`,
			want: []symbolExpectation{
				{kind: "package", name: "auth", line: 1},
				{kind: "type", name: "Service", container: "auth", line: 3},
				{kind: "const", name: "DefaultRole", container: "auth", line: 4},
				{kind: "var", name: "DefaultService", container: "auth", line: 5},
				{kind: "func", name: "Authorize", container: "auth", line: 7},
				{kind: "method", name: "Normalize", receiver: "Service", container: "auth", line: 11},
			},
		},
		{
			name: "python",
			path: "app.py",
			content: `class Service:
    def normalize(self, value):
        return value

async def authorize(value):
    return value
`,
			want: []symbolExpectation{
				{kind: "class", name: "Service", line: 1},
				{kind: "method", name: "normalize", container: "Service", line: 2},
				{kind: "func", name: "authorize", line: 5},
			},
		},
		{
			name: "typescript",
			path: "web/service.ts",
			content: `export interface Policy {
  role: string
}
export class Service {}
export function authorize() {}
const normalize = (value: string) => value
`,
			want: []symbolExpectation{
				{kind: "type", name: "Policy", line: 1},
				{kind: "class", name: "Service", line: 4},
				{kind: "func", name: "authorize", line: 5},
				{kind: "func", name: "normalize", line: 6},
			},
		},
		{
			name: "java",
			path: "App.java",
			content: `public class App {
  public String normalize(String value) {
    return value;
  }
}
`,
			want: []symbolExpectation{
				{kind: "class", name: "App", line: 1},
				{kind: "method", name: "normalize", container: "App", line: 2},
			},
		},
		{
			name: "rust",
			path: "lib.rs",
			content: `pub struct Service;
impl Service {
    pub fn normalize(value: &str) -> &str {
        value
    }
}
pub fn authorize(value: &str) -> &str {
    value
}
`,
			want: []symbolExpectation{
				{kind: "type", name: "Service", line: 1},
				{kind: "impl", name: "Service", line: 2},
				{kind: "method", name: "normalize", container: "Service", line: 3},
				{kind: "func", name: "authorize", line: 7},
			},
		},
		{
			name: "ruby",
			path: "service.rb",
			content: `class Service
  def normalize(value)
    value
  end
end
`,
			want: []symbolExpectation{
				{kind: "class", name: "Service", line: 1},
				{kind: "method", name: "normalize", container: "Service", line: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSymbols(tt.path, DetectLanguage(tt.path), tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractSymbols() len = %d, want %d: %#v", len(got), len(tt.want), got)
			}
			for i, want := range tt.want {
				if got[i].Kind != want.kind || got[i].Name != want.name || got[i].Receiver != want.receiver ||
					got[i].Container != want.container || got[i].StartLine != want.line || got[i].EndLine != want.line {
					t.Fatalf("ExtractSymbols()[%d] = %#v, want %#v", i, got[i], want)
				}
			}
		})
	}
}

func TestExtractSymbolsIsDeterministicAndBounded(t *testing.T) {
	var builder strings.Builder
	builder.WriteString("package bounded\n")
	for i := 0; i < maxSymbolsPerFile+20; i++ {
		fmt.Fprintf(&builder, "func Symbol%03d() {}\n", i)
	}

	first := ExtractSymbols("bounded.go", "go", builder.String())
	second := ExtractSymbols("bounded.go", "go", builder.String())

	if len(first) != maxSymbolsPerFile {
		t.Fatalf("len(ExtractSymbols()) = %d, want %d", len(first), maxSymbolsPerFile)
	}
	if fmt.Sprintf("%#v", first) != fmt.Sprintf("%#v", second) {
		t.Fatalf("ExtractSymbols() is not deterministic:\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestExtractSymbolsSkipsUnsupportedLanguages(t *testing.T) {
	for _, language := range []string{"markdown", "text", ""} {
		t.Run(language, func(t *testing.T) {
			if got := ExtractSymbols("README.md", language, "# Title\n"); len(got) != 0 {
				t.Fatalf("ExtractSymbols() = %#v, want none", got)
			}
		})
	}
}

type symbolExpectation struct {
	kind      string
	name      string
	receiver  string
	container string
	line      int
}
