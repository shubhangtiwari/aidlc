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

func ValidateSnapshot(snapshot Snapshot) error {
	included := make(map[string]struct{}, len(snapshot.Manifest.Payload.Include))
	for _, raw := range snapshot.Manifest.Payload.Include {
		normalized, err := payload.NormalizeRelativePath(raw)
		if err != nil {
			return fmt.Errorf("invalid template include %q: %w", raw, err)
		}
		if payload.IsPrivatePath(normalized) {
			return fmt.Errorf("template include %q is private", normalized)
		}
		if isBroadDirectory(normalized) && !snapshot.Manifest.Policy.AllowBroadDirectories {
			return fmt.Errorf("template include %q is a broad directory", normalized)
		}
		included[normalized] = struct{}{}
	}

	for _, raw := range snapshot.Manifest.Payload.Exclude {
		normalized, err := payload.NormalizeRelativePath(raw)
		if err != nil {
			return fmt.Errorf("invalid template exclude %q: %w", raw, err)
		}
		if _, ok := included[normalized]; ok {
			return fmt.Errorf("template path %q is both included and excluded", normalized)
		}
	}

	seen := make(map[string]struct{}, len(snapshot.Files))
	for i := range snapshot.Files {
		normalized, err := payload.NormalizeRelativePath(snapshot.Files[i].Path)
		if err != nil {
			return fmt.Errorf("invalid source path %q: %w", snapshot.Files[i].Path, err)
		}
		if _, ok := included[normalized]; !ok {
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
	paths := make([]string, 0, len(manifest.Payload.Include))
	for _, raw := range manifest.Payload.Include {
		normalized, err := payload.NormalizeRelativePath(raw)
		if err != nil {
			return nil, err
		}
		if isBroadDirectory(normalized) && !manifest.Policy.AllowBroadDirectories {
			return nil, fmt.Errorf("template include %q is a broad directory", normalized)
		}
		paths = append(paths, normalized)
	}
	return paths, nil
}

func ParseTemplateManifest(data []byte) (contract.TemplateManifest, error) {
	var manifest contract.TemplateManifest
	section := ""
	list := ""

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
		case section == "payload" && line == "exclude:":
			list = "exclude"
		case section == "payload" && strings.HasPrefix(line, "- "):
			value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "- ")), `"'`)
			if list == "include" {
				manifest.Payload.Include = append(manifest.Payload.Include, value)
			} else if list == "exclude" {
				manifest.Payload.Exclude = append(manifest.Payload.Exclude, value)
			} else {
				return manifest, fmt.Errorf("manifest list item on line %d has no active list", lineNo+1)
			}
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
	if len(manifest.Payload.Include) == 0 {
		return manifest, fmt.Errorf("template manifest payload.include is required")
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
