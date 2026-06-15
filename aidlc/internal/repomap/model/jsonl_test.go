package model

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteJSONLSortsAndTerminatesLines(t *testing.T) {
	var out bytes.Buffer
	records := []FileRecord{
		{Path: "z.go", Language: "go", SizeBytes: 2, Lines: 1, ContentHash: "z"},
		{Path: "a.go", Language: "go", SizeBytes: 1, Lines: 1, ContentHash: "a"},
	}

	if err := WriteJSONL(&out, records); err != nil {
		t.Fatalf("WriteJSONL() error = %v", err)
	}

	want := strings.Join([]string{
		`{"path":"a.go","language":"go","size_bytes":1,"lines":1,"content_hash":"a"}`,
		`{"path":"z.go","language":"go","size_bytes":2,"lines":1,"content_hash":"z"}`,
		"",
	}, "\n")
	if out.String() != want {
		t.Fatalf("WriteJSONL() = %q, want %q", out.String(), want)
	}
	if strings.Contains(out.String(), "\r") {
		t.Fatalf("WriteJSONL() used CRLF line endings")
	}

	got, err := ReadJSONL[FileRecord](bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("ReadJSONL() error = %v", err)
	}
	if len(got) != 2 || got[0].Path != "a.go" || got[1].Path != "z.go" {
		t.Fatalf("ReadJSONL() = %#v", got)
	}
}

func TestWriteJSONLDoesNotMutateInput(t *testing.T) {
	var out bytes.Buffer
	records := []ImportRecord{
		{Path: "b.go", ImportPath: "b"},
		{Path: "a.go", ImportPath: "a"},
	}

	if err := WriteJSONL(&out, records); err != nil {
		t.Fatalf("WriteJSONL() error = %v", err)
	}
	if records[0].Path != "b.go" || records[1].Path != "a.go" {
		t.Fatalf("WriteJSONL() mutated input: %#v", records)
	}
}

func TestJSONLEmptyCorpus(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSONL[FileRecord](&out, nil); err != nil {
		t.Fatalf("WriteJSONL(nil) error = %v", err)
	}
	if out.String() != "" {
		t.Fatalf("WriteJSONL(nil) = %q, want empty", out.String())
	}

	got, err := ReadJSONL[FileRecord](strings.NewReader(""))
	if err != nil {
		t.Fatalf("ReadJSONL(empty) error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReadJSONL(empty) len = %d, want 0", len(got))
	}
}

func TestReadJSONLReportsLineNumber(t *testing.T) {
	_, err := ReadJSONL[FileRecord](strings.NewReader("{}\nnot-json\n"))
	if err == nil {
		t.Fatalf("ReadJSONL() error = nil")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("ReadJSONL() error = %q, want line number", err)
	}
}
