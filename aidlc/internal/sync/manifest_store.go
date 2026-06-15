package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/source"
)

const mapArtifactsDir = "docs/map"

var invalidMapIncludeRoots = map[string]bool{
	".cache":        true,
	".claude":       true,
	".codex":        true,
	".cursor":       true,
	".git":          true,
	".idea":         true,
	".mypy_cache":   true,
	".pytest_cache": true,
	".ruff_cache":   true,
	".tox":          true,
	".venv":         true,
	".vscode":       true,
	"build":         true,
	"cache":         true,
	"dist":          true,
	"env":           true,
	"node_modules":  true,
	"out":           true,
	"target":        true,
	"vendor":        true,
	"venv":          true,
}

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
	preserveMapInclude := len(manifest.Workspace.Map.Include) == 0
	manifest.SchemaVersion = contract.TargetManifestVersion
	if err := normalizeManifestWorkspace(&manifest, false); err != nil {
		return err
	}
	if preserveMapInclude {
		include, ok, err := ReadManifestMapInclude(targetDir)
		if err != nil {
			return err
		}
		if ok {
			manifest.Workspace.Map.Include = include
		}
	}
	sort.Slice(manifest.Files, func(i, j int) bool {
		return manifest.Files[i].Path < manifest.Files[j].Path
	})
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode target manifest: %w", err)
	}
	content = append(content, '\n')

	return writeManifestContent(targetDir, content)
}

func ReadManifestMapInclude(targetDir string) ([]string, bool, error) {
	rootPath := filepath.Join(targetDir, filepath.FromSlash(contract.TargetManifestPath))
	content, err := os.ReadFile(rootPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read target manifest: %w", err)
	}
	var lock targetManifestLock
	if err := json.Unmarshal(content, &lock); err != nil {
		return nil, false, fmt.Errorf("parse target manifest: %w", err)
	}
	if lock.Workspace == nil || lock.Workspace.Map == nil || lock.Workspace.Map.Include == nil {
		return nil, false, nil
	}
	normalized, err := normalizeMapInclude(lock.Workspace.Map.Include)
	if err != nil {
		return nil, false, err
	}
	return normalized, true, nil
}

func WriteManifestMapInclude(targetDir string, include []string) error {
	normalized, err := normalizeMapInclude(include)
	if err != nil {
		return err
	}
	lock, err := readTargetManifestLock(targetDir)
	if err != nil {
		return err
	}
	lock.SchemaVersion = contract.TargetManifestVersion
	if lock.Workspace == nil {
		lock.Workspace = &workspaceLock{}
	}
	if lock.Workspace.Map == nil {
		lock.Workspace.Map = &mapWorkspaceLock{}
	}
	lock.Workspace.Map.Include = normalized

	content, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("encode target manifest: %w", err)
	}
	content = append(content, '\n')
	return writeManifestContent(targetDir, content)
}

func writeManifestContent(targetDir string, content []byte) error {
	manifestPath := filepath.Join(targetDir, filepath.FromSlash(contract.TargetManifestPath))
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return fmt.Errorf("create target manifest directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(manifestPath), ".manifest-*.json")
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
	if err := os.Rename(tempPath, manifestPath); err != nil {
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
	mapInclude, err := normalizeMapInclude(manifest.Workspace.Map.Include)
	if err != nil {
		return err
	}
	manifest.Workspace.Map.Include = mapInclude
	return nil
}

func normalizeMapInclude(include []string) ([]string, error) {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(include))
	for _, item := range include {
		clean, err := normalizeMapIncludePath(item)
		if err != nil {
			return nil, err
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		normalized = append(normalized, clean)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func normalizeMapIncludePath(item string) (string, error) {
	value := strings.TrimSpace(strings.ReplaceAll(item, "\\", "/"))
	if value == "" {
		return "", fmt.Errorf("map include path is empty")
	}
	if strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("map include path %q must be relative", item)
	}
	clean := path.Clean(value)
	if clean == "." || clean == "" {
		return "", fmt.Errorf("map include path %q must name a directory", item)
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("map include path %q must not escape the map root", item)
	}
	if clean == mapArtifactsDir || strings.HasPrefix(clean, mapArtifactsDir+"/") {
		return "", fmt.Errorf("map include path %q must not include generated map artifacts", item)
	}
	root, _, _ := strings.Cut(clean, "/")
	if invalidMapIncludeRoots[root] {
		return "", fmt.Errorf("map include path %q must not include generated or dependency directories", item)
	}
	return clean, nil
}

func readTargetManifestLock(targetDir string) (targetManifestLock, error) {
	rootPath := filepath.Join(targetDir, filepath.FromSlash(contract.TargetManifestPath))
	content, err := os.ReadFile(rootPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return targetManifestLock{}, nil
		}
		return targetManifestLock{}, fmt.Errorf("read target manifest: %w", err)
	}
	var lock targetManifestLock
	if err := json.Unmarshal(content, &lock); err != nil {
		return targetManifestLock{}, fmt.Errorf("parse target manifest: %w", err)
	}
	return lock, nil
}

type targetManifestLock struct {
	SchemaVersion int
	Upstream      json.RawMessage
	Workspace     *workspaceLock
	Generated     json.RawMessage
	Files         json.RawMessage
	Metadata      json.RawMessage
	Extra         map[string]json.RawMessage `json:"-"`
}

type workspaceLock struct {
	IDEs  json.RawMessage
	Map   *mapWorkspaceLock
	Extra map[string]json.RawMessage `json:"-"`
}

type mapWorkspaceLock struct {
	Include []string
	Extra   map[string]json.RawMessage `json:"-"`
}

func (lock *targetManifestLock) UnmarshalJSON(content []byte) error {
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(content, &fields); err != nil {
		return err
	}
	if raw, ok := fields["schema_version"]; ok {
		if err := json.Unmarshal(raw, &lock.SchemaVersion); err != nil {
			return fmt.Errorf("parse schema_version: %w", err)
		}
		delete(fields, "schema_version")
	}
	lock.Upstream = fields["upstream"]
	delete(fields, "upstream")
	if raw, ok := fields["workspace"]; ok {
		var workspace workspaceLock
		if err := json.Unmarshal(raw, &workspace); err != nil {
			return fmt.Errorf("parse workspace: %w", err)
		}
		lock.Workspace = &workspace
		delete(fields, "workspace")
	}
	lock.Generated = fields["generated"]
	delete(fields, "generated")
	lock.Files = fields["files"]
	delete(fields, "files")
	lock.Metadata = fields["metadata"]
	delete(fields, "metadata")
	lock.Extra = fields
	return nil
}

func (lock targetManifestLock) MarshalJSON() ([]byte, error) {
	fields := cloneRawFields(lock.Extra)
	fields["schema_version"] = mustMarshalRaw(lock.SchemaVersion)
	if len(lock.Upstream) > 0 {
		fields["upstream"] = lock.Upstream
	}
	if lock.Workspace != nil {
		raw, err := json.Marshal(lock.Workspace)
		if err != nil {
			return nil, err
		}
		fields["workspace"] = raw
	}
	if len(lock.Generated) > 0 {
		fields["generated"] = lock.Generated
	}
	if len(lock.Files) > 0 {
		fields["files"] = lock.Files
	}
	if len(lock.Metadata) > 0 {
		fields["metadata"] = lock.Metadata
	}
	return json.Marshal(fields)
}

func (workspace *workspaceLock) UnmarshalJSON(content []byte) error {
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(content, &fields); err != nil {
		return err
	}
	workspace.IDEs = fields["ides"]
	delete(fields, "ides")
	if raw, ok := fields["map"]; ok {
		var mapRecord mapWorkspaceLock
		if err := json.Unmarshal(raw, &mapRecord); err != nil {
			return fmt.Errorf("parse workspace.map: %w", err)
		}
		workspace.Map = &mapRecord
		delete(fields, "map")
	}
	workspace.Extra = fields
	return nil
}

func (workspace workspaceLock) MarshalJSON() ([]byte, error) {
	fields := cloneRawFields(workspace.Extra)
	if len(workspace.IDEs) > 0 {
		fields["ides"] = workspace.IDEs
	}
	if workspace.Map != nil {
		raw, err := json.Marshal(workspace.Map)
		if err != nil {
			return nil, err
		}
		fields["map"] = raw
	}
	return json.Marshal(fields)
}

func (mapRecord *mapWorkspaceLock) UnmarshalJSON(content []byte) error {
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(content, &fields); err != nil {
		return err
	}
	if raw, ok := fields["include"]; ok {
		if err := json.Unmarshal(raw, &mapRecord.Include); err != nil {
			return fmt.Errorf("parse include: %w", err)
		}
		delete(fields, "include")
	}
	mapRecord.Extra = fields
	return nil
}

func (mapRecord mapWorkspaceLock) MarshalJSON() ([]byte, error) {
	fields := cloneRawFields(mapRecord.Extra)
	if mapRecord.Include != nil {
		fields["include"] = mustMarshalRaw(mapRecord.Include)
	}
	return json.Marshal(fields)
}

func cloneRawFields(fields map[string]json.RawMessage) map[string]json.RawMessage {
	clone := map[string]json.RawMessage{}
	for name, raw := range fields {
		clone[name] = raw
	}
	return clone
}

func mustMarshalRaw(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func ManifestFromPlan(plan Plan, generated contract.GenerationRecord, metadata map[string]string) contract.TargetManifest {
	return manifestFromFiles(plan.Upstream, plan.Files, generated, metadata)
}

func ManifestFromAcceptedPlan(plan Plan, generated contract.GenerationRecord, metadata map[string]string) contract.TargetManifest {
	files := make(map[string]source.File)
	for _, decision := range plan.Decisions {
		if !acceptedUpstreamDecision(decision) {
			continue
		}
		file, ok := plan.Files[decision.Path]
		if !ok {
			continue
		}
		files[decision.Path] = file
	}
	return manifestFromFiles(plan.Upstream, files, generated, metadata)
}

func acceptedUpstreamDecision(decision Decision) bool {
	if decision.State == StateConflict || decision.State == StateRemovedUpstream {
		return false
	}
	if decision.IsWritable() {
		return true
	}
	return decision.UpstreamChecksum != "" && decision.LocalChecksum == decision.UpstreamChecksum
}

func manifestFromFiles(
	upstream contract.UpstreamRef,
	sourceFiles map[string]source.File,
	generated contract.GenerationRecord,
	metadata map[string]string,
) contract.TargetManifest {
	files := make([]contract.ManifestFile, 0, len(sourceFiles))
	for path, file := range sourceFiles {
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
		Upstream:      upstream,
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
