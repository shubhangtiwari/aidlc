package source

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aidlc/ai-dlc-template/aidlc/internal/contract"
)

type Archive struct {
	Data   []byte
	Root   string
	Source string
	Ref    string
	Commit string
}

func (a Archive) Snapshot(ctx context.Context) (Snapshot, error) {
	reader, err := zip.NewReader(bytes.NewReader(a.Data), int64(len(a.Data)))
	if err != nil {
		return Snapshot{}, fmt.Errorf("open archive: %w", err)
	}

	root := strings.Trim(strings.ReplaceAll(a.Root, "\\", "/"), "/")
	entries := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		if file.FileInfo().IsDir() {
			continue
		}
		name := strings.TrimPrefix(file.Name, "/")
		if root != "" {
			name = strings.TrimPrefix(name, root+"/")
		} else if first, rest, ok := strings.Cut(name, "/"); ok && strings.Contains(first, "-") {
			name = rest
		}
		entries[path.Clean(name)] = file
	}

	manifestFile, ok := entries[contract.TemplateManifestPath]
	if !ok {
		return Snapshot{}, fmt.Errorf("archive missing %s", contract.TemplateManifestPath)
	}
	manifestBytes, err := readZipFile(manifestFile)
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
		file, ok := entries[name]
		if !ok {
			return Snapshot{}, fmt.Errorf("archive missing payload file %s", name)
		}
		content, err := readZipFile(file)
		if err != nil {
			return Snapshot{}, fmt.Errorf("read payload file %s: %w", name, err)
		}
		files = append(files, File{Path: name, Content: content, Mode: file.Mode().Perm()})
	}

	snapshot := Snapshot{
		Manifest: manifest,
		Upstream: contract.UpstreamRef{
			Source: a.Source,
			Ref:    a.Ref,
			Commit: a.Commit,
		},
		Files: files,
	}
	if snapshot.Upstream.Source == "" {
		snapshot.Upstream.Source = "archive"
	}
	return snapshot, ValidateSnapshot(snapshot)
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
