package model

import (
	"encoding/json"
	"testing"
)

func TestRecordJSONFieldOrder(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{
			name: "file",
			in:   FileRecord{Path: "main.go", Language: "go", SizeBytes: 42, Lines: 3, ContentHash: "abc"},
			want: `{"path":"main.go","language":"go","size_bytes":42,"lines":3,"content_hash":"abc"}`,
		},
		{
			name: "import",
			in:   ImportRecord{Path: "main.go", Language: "go", ImportPath: "fmt"},
			want: `{"path":"main.go","language":"go","import_path":"fmt"}`,
		},
		{
			name: "test",
			in:   TestRecord{Path: "main_test.go", Language: "go", TargetPath: "main.go"},
			want: `{"path":"main_test.go","language":"go","target_path":"main.go"}`,
		},
		{
			name: "blueprint",
			in:   BlueprintRecord{Path: "docs/blueprints/core.md", Module: "core", Section: "Owned State", Text: "none"},
			want: `{"path":"docs/blueprints/core.md","module":"core","section":"Owned State","text":"none"}`,
		},
		{
			name: "doc",
			in:   DocRecord{Path: "docs/ARCHITECTURE.md", Kind: "architecture", Title: "Architecture", Text: "layers"},
			want: `{"path":"docs/ARCHITECTURE.md","kind":"architecture","title":"Architecture","text":"layers"}`,
		},
		{
			name: "change",
			in:   ChangeRecord{Path: "docs/spec/1-add.md", Kind: "spec", ID: "1", Title: "Add", Status: "approved", Text: "body"},
			want: `{"path":"docs/spec/1-add.md","kind":"spec","id":"1","title":"Add","status":"approved","text":"body"}`,
		},
		{
			name: "index",
			in:   DefaultIndexMeta(),
			want: `{"schema_version":1,"map_dir":"docs/map","index_file":"index.json","sqlite_file":"repo-map.sqlite","shards":{"files":"files.jsonl","imports":"imports.jsonl","tests":"tests.jsonl","blueprints":"blueprints.jsonl","docs":"docs.jsonl","changes":"changes.jsonl"}}`,
		},
		{
			name: "query result",
			in:   QueryResult{Path: "main.go", Score: 1.25, Snippet: "func main"},
			want: `{"path":"main.go","score":1.25,"snippet":"func main"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRecordSortKeys(t *testing.T) {
	tests := []struct {
		name string
		in   SortableRecord
		want string
	}{
		{name: "file", in: FileRecord{Path: "b.go"}, want: "b.go"},
		{name: "import", in: ImportRecord{Path: "b.go", ImportPath: "fmt"}, want: "b.go\x00fmt"},
		{name: "test", in: TestRecord{Path: "b_test.go", TargetPath: "b.go"}, want: "b_test.go\x00b.go"},
		{name: "blueprint", in: BlueprintRecord{Path: "docs/blueprints/core.md", Module: "core", Section: "Layer Map"}, want: "docs/blueprints/core.md\x00core\x00Layer Map"},
		{name: "doc", in: DocRecord{Path: "docs/adr/1.md", Kind: "adr", Title: "Decision"}, want: "adr\x00docs/adr/1.md\x00Decision"},
		{name: "change", in: ChangeRecord{Path: "docs/spec/1.md", Kind: "spec", ID: "1"}, want: "spec\x001\x00docs/spec/1.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.SortKey(); got != tt.want {
				t.Fatalf("SortKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
