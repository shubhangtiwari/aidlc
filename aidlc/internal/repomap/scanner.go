package repomap

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type Shards struct {
	Files        []model.FileRecord
	Imports      []model.ImportRecord
	Tests        []model.TestRecord
	Blueprints   []model.BlueprintRecord
	Docs         []model.DocRecord
	Changes      []model.ChangeRecord
	SourceChunks []model.SourceChunkRecord
	Symbols      []model.SymbolRecord
}

func ScanDir(root string) (*Shards, error) {
	return ScanDirWithOptions(root, ScanOptions{})
}

func ScanDirWithOptions(root string, options ScanOptions) (*Shards, error) {
	include, err := NormalizeInclude(options.Include)
	if err != nil {
		return nil, err
	}
	scanner := scanner{root: root, include: include}
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
		{model.SourceChunksShard, func(f *os.File) error { return model.WriteJSONL(f, shards.SourceChunks) }},
		{model.SymbolsShard, func(f *os.File) error { return model.WriteJSONL(f, shards.Symbols) }},
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
	root    string
	include []string
	shards  Shards
}

func (s *scanner) scan() error {
	return filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return fmt.Errorf("relativize %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel == "." {
				return nil
			}
			if shouldSkipDir(entry.Name()) || shouldSkipPath(rel) {
				return filepath.SkipDir
			}
			if len(s.include) > 0 && !includeAllowsDir(s.include, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&fs.ModeType != 0 {
			return nil
		}

		if shouldSkipFile(rel) || (len(s.include) > 0 && !isRootFile(rel) && !includeContainsFile(s.include, rel)) {
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
		s.shards.SourceChunks = append(s.shards.SourceChunks, ExtractSourceChunks(rel, language, string(content))...)
		s.shards.Symbols = append(s.shards.Symbols, ExtractSymbols(rel, language, string(content))...)

		docRecords, changeRecords, blueprintRecords := ScanDoc(rel, string(content))
		s.shards.Docs = append(s.shards.Docs, docRecords...)
		s.shards.Changes = append(s.shards.Changes, changeRecords...)
		s.shards.Blueprints = append(s.shards.Blueprints, blueprintRecords...)
		return nil
	})
}

func DetectIncludeCandidates(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read map root: %w", err)
	}

	candidates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if shouldSkipDir(name) || shouldSkipPath(name) {
			continue
		}
		clean, err := NormalizeIncludePath(name)
		if err != nil {
			continue
		}
		candidates = append(candidates, clean)
	}
	sort.Strings(candidates)
	return candidates, nil
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git",
		".claude", ".codex", ".cursor",
		".idea", ".vscode",
		".venv", "venv", "env",
		"node_modules", "vendor",
		"build", "dist", "out", "target",
		".cache", "cache", "__pycache__",
		".pytest_cache", ".mypy_cache", ".ruff_cache",
		".next":
		return true
	default:
		return false
	}
}

func shouldSkipPath(path string) bool {
	return path == model.MapDir || strings.HasPrefix(path, model.MapDir+"/")
}

func shouldSkipFile(path string) bool {
	return strings.HasPrefix(path, model.MapDir+"/")
}

func includeAllowsDir(include []string, dir string) bool {
	for _, item := range include {
		if dir == item || strings.HasPrefix(dir, item+"/") || strings.HasPrefix(item, dir+"/") {
			return true
		}
	}
	return false
}

func includeContainsFile(include []string, path string) bool {
	for _, item := range include {
		if strings.HasPrefix(path, item+"/") {
			return true
		}
	}
	return false
}

func isRootFile(path string) bool {
	return !strings.Contains(path, "/")
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
