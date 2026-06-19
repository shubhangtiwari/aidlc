package model

import "strconv"

type FileRecord struct {
	Path        string `json:"path"`
	Language    string `json:"language"`
	SizeBytes   int64  `json:"size_bytes"`
	Lines       int    `json:"lines"`
	ContentHash string `json:"content_hash"`
}

func (r FileRecord) SortKey() string {
	return r.Path
}

type ImportRecord struct {
	Path       string `json:"path"`
	Language   string `json:"language"`
	ImportPath string `json:"import_path"`
}

func (r ImportRecord) SortKey() string {
	return r.Path + "\x00" + r.ImportPath
}

type TestRecord struct {
	Path       string `json:"path"`
	Language   string `json:"language"`
	TargetPath string `json:"target_path"`
}

func (r TestRecord) SortKey() string {
	return r.Path + "\x00" + r.TargetPath
}

type BlueprintRecord struct {
	Path    string `json:"path"`
	Module  string `json:"module"`
	Section string `json:"section"`
	Text    string `json:"text"`
}

func (r BlueprintRecord) SortKey() string {
	return r.Path + "\x00" + r.Module + "\x00" + r.Section
}

type DocRecord struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

func (r DocRecord) SortKey() string {
	return r.Kind + "\x00" + r.Path + "\x00" + r.Title
}

type ChangeRecord struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Text   string `json:"text"`
}

func (r ChangeRecord) SortKey() string {
	return r.Kind + "\x00" + r.ID + "\x00" + r.Path
}

type SourceChunkRecord struct {
	Path      string `json:"path"`
	Language  string `json:"language"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Text      string `json:"text"`
}

func (r SourceChunkRecord) SortKey() string {
	return r.Path + "\x00" + lineSortKey(r.StartLine) + "\x00" + lineSortKey(r.EndLine)
}

func lineSortKey(line int) string {
	if line < 0 {
		line = 0
	}
	const width = 10
	text := strconv.Itoa(line)
	if len(text) >= width {
		return text
	}
	return "0000000000"[:width-len(text)] + text
}
