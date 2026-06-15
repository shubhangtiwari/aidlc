package repomap

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type Shards struct {
	Files      []model.FileRecord
	Imports    []model.ImportRecord
	Tests      []model.TestRecord
	Blueprints []model.BlueprintRecord
	Docs       []model.DocRecord
	Changes    []model.ChangeRecord
}

func ScanDir(root string) (*Shards, error) {
	scanner := scanner{root: root}
	if err := scanner.scan(); err != nil {
		return nil, err
	}
	scanner.shards.Tests = LinkTests(scanner.shards.Files)
	return &scanner.shards, nil
}

func WriteShards(mapDir string, shards Shards) error {
	if err := os.MkdirAll(mapDir, 0o755); err != nil {
		return fmt.Errorf("create map dir: %w", err)
	}

	writes := []struct {
		name  string
		write func(*os.File) error
	}{
		{model.FilesShard, func(f *os.File) error { return model.WriteJSONL(f, shards.Files) }},
		{model.ImportsShard, func(f *os.File) error { return model.WriteJSONL(f, shards.Imports) }},
		{model.TestsShard, func(f *os.File) error { return model.WriteJSONL(f, shards.Tests) }},
		{model.BlueprintsShard, func(f *os.File) error { return model.WriteJSONL(f, shards.Blueprints) }},
		{model.DocsShard, func(f *os.File) error { return model.WriteJSONL(f, shards.Docs) }},
		{model.ChangesShard, func(f *os.File) error { return model.WriteJSONL(f, shards.Changes) }},
	}

	for _, item := range writes {
		path := filepath.Join(mapDir, item.name)
		file, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("create shard %s: %w", item.name, err)
		}
		if err := item.write(file); err != nil {
			_ = file.Close()
			return fmt.Errorf("write shard %s: %w", item.name, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close shard %s: %w", item.name, err)
		}
	}
	return nil
}

type scanner struct {
	root   string
	shards Shards
}

func (s *scanner) scan() error {
	return filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&fs.ModeType != 0 {
			return nil
		}

		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return fmt.Errorf("relativize %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)
		if shouldSkipFile(rel) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		language := DetectLanguage(rel)
		s.shards.Files = append(s.shards.Files, model.FileRecord{
			Path:        rel,
			Language:    language,
			SizeBytes:   int64(len(content)),
			Lines:       countLines(content),
			ContentHash: model.ContentHash(content),
		})
		s.shards.Imports = append(s.shards.Imports, ExtractImports(rel, language, string(content))...)

		docRecords, changeRecords, blueprintRecords := ScanDoc(rel, string(content))
		s.shards.Docs = append(s.shards.Docs, docRecords...)
		s.shards.Changes = append(s.shards.Changes, changeRecords...)
		s.shards.Blueprints = append(s.shards.Blueprints, blueprintRecords...)
		return nil
	})
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".idea", ".vscode":
		return true
	default:
		return false
	}
}

func shouldSkipFile(path string) bool {
	return strings.HasPrefix(path, model.MapDir+"/")
}

func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	lines := bytes.Count(content, []byte{'\n'})
	if !bytes.HasSuffix(content, []byte{'\n'}) {
		lines++
	}
	return lines
}
