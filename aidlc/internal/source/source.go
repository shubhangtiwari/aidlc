package source

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/payload"
)

type File struct {
	Path    string
	Content []byte
	Mode    fs.FileMode
}

type Snapshot struct {
	Manifest contract.TemplateManifest
	Upstream contract.UpstreamRef
	Files    []File
}

type Provider interface {
	Snapshot(ctx context.Context) (Snapshot, error)
}

type ManifestInclude struct {
	SourcePath string
	TargetPath string
}

func ValidateSnapshot(snapshot Snapshot) error {
	entries, err := ManifestIncludeEntries(snapshot.Manifest)
	if err != nil {
		return err
	}

	includedTargets := make(map[string]struct{}, len(entries))
	sourceTargets := make(map[string]string, len(entries))
	for _, entry := range entries {
		if _, ok := includedTargets[entry.TargetPath]; ok {
			return fmt.Errorf("duplicate template target path %q", entry.TargetPath)
		}
		includedTargets[entry.TargetPath] = struct{}{}
		sourceTargets[entry.SourcePath] = entry.TargetPath
	}

	for _, raw := range snapshot.Manifest.Payload.Exclude {
		normalized, err := payload.NormalizeRelativePath(raw)
		if err != nil {
			return fmt.Errorf("invalid template exclude %q: %w", raw, err)
		}
		for _, entry := range entries {
			if normalized == entry.SourcePath || normalized == entry.TargetPath {
				return fmt.Errorf("template path %q is both included and excluded", normalized)
			}
		}
		if _, ok := includedTargets[normalized]; ok {
			return fmt.Errorf("template path %q is both included and excluded", normalized)
		}
	}

	seen := make(map[string]struct{}, len(snapshot.Files))
	for i := range snapshot.Files {
		normalized, err := payload.NormalizeRelativePath(snapshot.Files[i].Path)
		if err != nil {
			return fmt.Errorf("invalid source path %q: %w", snapshot.Files[i].Path, err)
		}
		if targetPath, ok := sourceTargets[normalized]; ok {
			normalized = targetPath
		}
		if _, ok := includedTargets[normalized]; !ok {
			return fmt.Errorf("source path %q is not included by the public template manifest", normalized)
		}
		if _, ok := seen[normalized]; ok {
			return fmt.Errorf("duplicate source path %q", normalized)
		}
		seen[normalized] = struct{}{}
		snapshot.Files[i].Path = normalized
	}

	return nil
}

func ManifestIncludes(manifest contract.TemplateManifest) ([]string, error) {
	entries, err := ManifestIncludeEntries(manifest)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.SourcePath)
	}
	return paths, nil
}

func ManifestIncludeEntries(manifest contract.TemplateManifest) ([]ManifestInclude, error) {
	entries := make([]ManifestInclude, 0, len(manifest.Payload.Include)+len(manifest.Payload.IncludeMappings))
	seenTargets := make(map[string]struct{}, len(manifest.Payload.Include)+len(manifest.Payload.IncludeMappings))

	for _, raw := range manifest.Payload.Include {
		normalized, err := normalizeManifestPath("template include", raw, manifest.Policy.AllowBroadDirectories)
		if err != nil {
			return nil, err
		}
		if err := validatePublicTarget("template include", normalized); err != nil {
			return nil, err
		}
		if _, ok := seenTargets[normalized]; ok {
			return nil, fmt.Errorf("duplicate template target path %q", normalized)
		}
		seenTargets[normalized] = struct{}{}
		entries = append(entries, ManifestInclude{SourcePath: normalized, TargetPath: normalized})
	}

	for _, mapping := range manifest.Payload.IncludeMappings {
		sourcePath, err := normalizeManifestPath("template include source", mapping.Source, false)
		if err != nil {
			return nil, err
		}
		if err := validatePublicTarget("template include source", sourcePath); err != nil {
			return nil, err
		}
		targetPath, err := normalizeManifestPath("template include target", mapping.Target, false)
		if err != nil {
			return nil, err
		}
		if err := validatePublicTarget("template include target", targetPath); err != nil {
			return nil, err
		}
		if _, ok := seenTargets[targetPath]; ok {
			return nil, fmt.Errorf("duplicate template target path %q", targetPath)
		}
		seenTargets[targetPath] = struct{}{}
		entries = append(entries, ManifestInclude{SourcePath: sourcePath, TargetPath: targetPath})
	}

	return entries, nil
}

func ParseTemplateManifest(data []byte) (contract.TemplateManifest, error) {
	var manifest contract.TemplateManifest
	section := ""
	list := ""
	pendingMapping := -1

	for lineNo, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "schema_version:"):
			value := strings.TrimSpace(strings.TrimPrefix(line, "schema_version:"))
			if value != "1" {
				return manifest, fmt.Errorf("unsupported template manifest schema_version %q", value)
			}
			manifest.SchemaVersion = contract.TemplateManifestV1
		case line == "payload:":
			section = "payload"
			list = ""
		case line == "policy:":
			section = "policy"
			list = ""
		case section == "payload" && line == "include:":
			list = "include"
			pendingMapping = -1
		case section == "payload" && line == "exclude:":
			list = "exclude"
			pendingMapping = -1
		case section == "payload" && strings.HasPrefix(line, "- "):
			value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "- ")), `"'`)
			if list == "include" {
				if source, ok := parseInlineMappingField(value, "source"); ok {
					manifest.Payload.IncludeMappings = append(manifest.Payload.IncludeMappings, contract.TemplatePayloadMapping{Source: source})
					pendingMapping = len(manifest.Payload.IncludeMappings) - 1
				} else {
					manifest.Payload.Include = append(manifest.Payload.Include, value)
					pendingMapping = -1
				}
			} else if list == "exclude" {
				manifest.Payload.Exclude = append(manifest.Payload.Exclude, value)
				pendingMapping = -1
			} else {
				return manifest, fmt.Errorf("manifest list item on line %d has no active list", lineNo+1)
			}
		case section == "payload" && list == "include" && pendingMapping >= 0 && strings.HasPrefix(line, "target:"):
			target := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "target:")), `"'`)
			manifest.Payload.IncludeMappings[pendingMapping].Target = target
			pendingMapping = -1
		case section == "policy" && strings.Contains(line, ":"):
			key, value, _ := strings.Cut(line, ":")
			boolValue, err := parseBool(strings.TrimSpace(value))
			if err != nil {
				return manifest, fmt.Errorf("invalid policy %q on line %d: %w", key, lineNo+1, err)
			}
			switch strings.TrimSpace(key) {
			case "allow_broad_directories":
				manifest.Policy.AllowBroadDirectories = boolValue
			case "public_docs_must_be_explicit":
				manifest.Policy.PublicDocsMustBeExplicit = boolValue
			case "reject_absolute_paths":
				manifest.Policy.RejectAbsolutePaths = boolValue
			case "reject_parent_traversal":
				manifest.Policy.RejectParentTraversal = boolValue
			default:
				return manifest, fmt.Errorf("unknown policy key %q on line %d", key, lineNo+1)
			}
		default:
			return manifest, fmt.Errorf("unsupported template manifest line %d: %s", lineNo+1, line)
		}
	}

	if manifest.SchemaVersion == 0 {
		return manifest, fmt.Errorf("template manifest schema_version is required")
	}
	if len(manifest.Payload.Include)+len(manifest.Payload.IncludeMappings) == 0 {
		return manifest, fmt.Errorf("template manifest payload.include is required")
	}
	for _, mapping := range manifest.Payload.IncludeMappings {
		if mapping.Source == "" || mapping.Target == "" {
			return manifest, fmt.Errorf("template manifest mapped include requires source and target")
		}
	}
	return manifest, nil
}

func parseBool(value string) (bool, error) {
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("expected true or false")
	}
}

func isBroadDirectory(value string) bool {
	if strings.HasSuffix(value, "/**") {
		return true
	}
	return strings.Contains(path.Base(value), "*")
}

func normalizeManifestPath(label, raw string, allowBroadDirectories bool) (string, error) {
	normalized, err := payload.NormalizeRelativePath(raw)
	if err != nil {
		return "", fmt.Errorf("invalid %s %q: %w", label, raw, err)
	}
	if isBroadDirectory(normalized) && !allowBroadDirectories {
		return "", fmt.Errorf("%s %q is a broad directory", label, normalized)
	}
	return normalized, nil
}

func validatePublicTarget(label, normalized string) error {
	if payload.IsPrivatePath(normalized) {
		return fmt.Errorf("%s %q is private", label, normalized)
	}
	return nil
}

func parseInlineMappingField(value, key string) (string, bool) {
	field, raw, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(field) != key {
		return "", false
	}
	return strings.Trim(strings.TrimSpace(raw), `"'`), true
}
