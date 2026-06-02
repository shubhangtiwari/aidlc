package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aidlc/ai-dlc-template/aidlc/internal/contract"
)

type Local struct {
	Root   string
	Source string
	Ref    string
	Commit string
}

func (l Local) Snapshot(ctx context.Context) (Snapshot, error) {
	root := l.Root
	if root == "" {
		root = "."
	}

	manifestPath := filepath.Join(root, filepath.FromSlash(contract.TemplateManifestPath))
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read template manifest: %w", err)
	}
	manifest, err := ParseTemplateManifest(manifestBytes)
	if err != nil {
		return Snapshot{}, err
	}

	includePaths, err := ManifestIncludes(manifest)
	if err != nil {
		return Snapshot{}, err
	}

	files := make([]File, 0, len(includePaths))
	for _, name := range includePaths {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		fullPath := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Stat(fullPath)
		if err != nil {
			return Snapshot{}, fmt.Errorf("stat payload file %s: %w", name, err)
		}
		if info.IsDir() {
			return Snapshot{}, fmt.Errorf("payload include %s is a directory", name)
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return Snapshot{}, fmt.Errorf("read payload file %s: %w", name, err)
		}
		files = append(files, File{Path: name, Content: content, Mode: info.Mode().Perm()})
	}

	snapshot := Snapshot{
		Manifest: manifest,
		Upstream: contract.UpstreamRef{
			Source: l.Source,
			Ref:    l.Ref,
			Commit: l.Commit,
		},
		Files: files,
	}
	if snapshot.Upstream.Source == "" {
		snapshot.Upstream.Source = "local"
	}
	return snapshot, ValidateSnapshot(snapshot)
}
