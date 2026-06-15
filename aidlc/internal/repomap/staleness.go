package repomap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type IndexManifest struct {
	model.IndexMeta
	Files []IndexFileHash `json:"files"`
}

type IndexFileHash struct {
	Path        string `json:"path"`
	ContentHash string `json:"content_hash"`
}

type StalenessStatus struct {
	Fresh        bool
	MissingIndex bool
	Changed      []string
	Missing      []string
	Added        []string
}

func WriteIndex(mapDir string, shards Shards) error {
	if err := os.MkdirAll(mapDir, 0o755); err != nil {
		return fmt.Errorf("create map dir: %w", err)
	}

	manifest := IndexManifest{
		IndexMeta: model.DefaultIndexMeta(),
		Files:     fileHashes(shards.Files),
	}
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal repo-map index: %w", err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(filepath.Join(mapDir, model.IndexFilename), content, 0o644); err != nil {
		return fmt.Errorf("write repo-map index: %w", err)
	}
	return nil
}

func CheckStaleness(root string) (StalenessStatus, error) {
	manifest, err := ReadIndex(filepath.Join(root, model.MapDir))
	if err != nil {
		if os.IsNotExist(err) {
			return StalenessStatus{MissingIndex: true}, nil
		}
		return StalenessStatus{}, err
	}

	current, err := ScanDir(root)
	if err != nil {
		return StalenessStatus{}, err
	}

	indexed := map[string]string{}
	for _, file := range manifest.Files {
		indexed[file.Path] = file.ContentHash
	}
	seen := map[string]bool{}

	status := StalenessStatus{}
	for _, file := range current.Files {
		seen[file.Path] = true
		hash, ok := indexed[file.Path]
		if !ok {
			status.Added = append(status.Added, file.Path)
			continue
		}
		if hash != file.ContentHash {
			status.Changed = append(status.Changed, file.Path)
		}
	}
	for path := range indexed {
		if !seen[path] {
			status.Missing = append(status.Missing, path)
		}
	}
	sort.Strings(status.Changed)
	sort.Strings(status.Missing)
	sort.Strings(status.Added)
	status.Fresh = len(status.Changed) == 0 && len(status.Missing) == 0 && len(status.Added) == 0
	return status, nil
}

func ReadIndex(mapDir string) (IndexManifest, error) {
	content, err := os.ReadFile(filepath.Join(mapDir, model.IndexFilename))
	if err != nil {
		return IndexManifest{}, err
	}
	var manifest IndexManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return IndexManifest{}, fmt.Errorf("read repo-map index: %w", err)
	}
	return manifest, nil
}

func fileHashes(files []model.FileRecord) []IndexFileHash {
	hashes := make([]IndexFileHash, 0, len(files))
	for _, file := range files {
		hashes = append(hashes, IndexFileHash{Path: file.Path, ContentHash: file.ContentHash})
	}
	sort.SliceStable(hashes, func(i, j int) bool {
		return hashes[i].Path < hashes[j].Path
	})
	return hashes
}
