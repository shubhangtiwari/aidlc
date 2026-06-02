package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aidlc/ai-dlc-template/aidlc/internal/contract"
)

func ReadManifest(targetDir string) (*contract.TargetManifest, error) {
	rootPath := filepath.Join(targetDir, filepath.FromSlash(contract.TargetManifestPath))
	manifest, err := readManifestFile(rootPath, false)
	if err == nil {
		return manifest, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	legacyPath := filepath.Join(targetDir, filepath.FromSlash(contract.LegacyTargetManifestPath))
	manifest, err = readManifestFile(legacyPath, true)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return manifest, err
}

func readManifestFile(path string, allowGeneratedIDEFallback bool) (*contract.TargetManifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("read target manifest: %w", err)
	}
	var manifest contract.TargetManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("parse target manifest: %w", err)
	}
	if manifest.SchemaVersion == 0 {
		manifest.SchemaVersion = contract.TargetManifestVersion
	}
	if err := normalizeManifestWorkspace(&manifest, allowGeneratedIDEFallback); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func WriteManifest(targetDir string, manifest contract.TargetManifest) error {
	manifest.SchemaVersion = contract.TargetManifestVersion
	if err := normalizeManifestWorkspace(&manifest, false); err != nil {
		return err
	}
	sort.Slice(manifest.Files, func(i, j int) bool {
		return manifest.Files[i].Path < manifest.Files[j].Path
	})
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode target manifest: %w", err)
	}
	content = append(content, '\n')

	path := filepath.Join(targetDir, filepath.FromSlash(contract.TargetManifestPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create target manifest directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".manifest-*.json")
	if err != nil {
		return fmt.Errorf("create target manifest temp file: %w", err)
	}
	tempPath := temp.Name()
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		os.Remove(tempPath)
		return fmt.Errorf("write target manifest temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("close target manifest temp file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("replace target manifest: %w", err)
	}
	return nil
}

func normalizeManifestWorkspace(manifest *contract.TargetManifest, allowGeneratedIDEFallback bool) error {
	selection := manifest.Workspace.IDEs
	if allowGeneratedIDEFallback && len(selection) == 0 && manifest.Generated.IDE != "" {
		selection = []contract.IDE{manifest.Generated.IDE}
	}
	normalized, err := contract.NormalizeIDESelection(selection)
	if err != nil {
		return fmt.Errorf("normalize workspace IDEs: %w", err)
	}
	manifest.Workspace.IDEs = normalized
	return nil
}

func ManifestFromPlan(plan Plan, generated contract.GenerationRecord, metadata map[string]string) contract.TargetManifest {
	files := make([]contract.ManifestFile, 0, len(plan.Files))
	for path, file := range plan.Files {
		files = append(files, contract.ManifestFile{
			Path:     path,
			Checksum: BytesChecksum(file.Content),
			Mode:     formatMode(file.Mode),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return contract.TargetManifest{
		SchemaVersion: contract.TargetManifestVersion,
		Upstream:      plan.Upstream,
		Workspace: contract.WorkspaceRecord{
			IDEs: mustNormalizeIDESelection([]contract.IDE{generated.IDE}),
		},
		Generated: generated,
		Files:     files,
		Metadata:  metadata,
	}
}

func mustNormalizeIDESelection(selection []contract.IDE) []contract.IDE {
	normalized, err := contract.NormalizeIDESelection(selection)
	if err != nil {
		return nil
	}
	return normalized
}
