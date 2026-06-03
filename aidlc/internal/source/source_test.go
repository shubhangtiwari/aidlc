package source

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

func TestParseTemplateManifestSupportsSamePathAndMappedIncludes(t *testing.T) {
	manifest, err := ParseTemplateManifest([]byte(`schema_version: 1
payload:
  include:
    - .ai/README.md
    - source: LICENSE
      target: licenses/aidlc.md
  exclude:
    - docs/spec/[0-9]*-*.md
policy:
  allow_broad_directories: false
  public_docs_must_be_explicit: true
  reject_absolute_paths: true
  reject_parent_traversal: true
`))
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	entries, err := ManifestIncludeEntries(manifest)
	if err != nil {
		t.Fatalf("manifest include entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	assertEntry(t, entries[0], ".ai/README.md", ".ai/README.md")
	assertEntry(t, entries[1], "LICENSE", "licenses/aidlc.md")

	sourcePaths, err := ManifestIncludes(manifest)
	if err != nil {
		t.Fatalf("manifest includes: %v", err)
	}
	if strings.Join(sourcePaths, ",") != ".ai/README.md,LICENSE" {
		t.Fatalf("source paths = %v, want same-path source then mapped source", sourcePaths)
	}
}

func TestValidateSnapshotAcceptsMappedTargetPathAndSamePathIncludes(t *testing.T) {
	snapshot := Snapshot{
		Manifest: manifestWithIncludes(
			[]string{".ai/README.md"},
			[]contract.TemplatePayloadMapping{{Source: "LICENSE", Target: "licenses/aidlc.md"}},
		),
		Files: []File{
			file(".ai/README.md"),
			file("LICENSE"),
		},
	}

	if err := ValidateSnapshot(snapshot); err != nil {
		t.Fatalf("validate snapshot: %v", err)
	}
	if snapshot.Files[1].Path != "licenses/aidlc.md" {
		t.Fatalf("mapped file path = %q, want target path", snapshot.Files[1].Path)
	}
}

func TestValidateSnapshotRejectsInvalidMappedPayload(t *testing.T) {
	tests := []struct {
		name     string
		include  []string
		mappings []contract.TemplatePayloadMapping
		files    []File
		want     string
	}{
		{
			name:     "absolute source",
			mappings: []contract.TemplatePayloadMapping{{Source: "/LICENSE", Target: "licenses/aidlc.md"}},
			files:    []File{file("licenses/aidlc.md")},
			want:     "must be relative",
		},
		{
			name:     "parent traversal target",
			mappings: []contract.TemplatePayloadMapping{{Source: "LICENSE", Target: "../licenses/aidlc.md"}},
			files:    []File{file("licenses/aidlc.md")},
			want:     "escapes the payload root",
		},
		{
			name:     "private target",
			mappings: []contract.TemplatePayloadMapping{{Source: "LICENSE", Target: "aidlc/internal/source/source.go"}},
			files:    []File{file("aidlc/internal/source/source.go")},
			want:     "private",
		},
		{
			name:     "broad target glob",
			mappings: []contract.TemplatePayloadMapping{{Source: "LICENSE", Target: "licenses/**"}},
			files:    []File{file("licenses/aidlc.md")},
			want:     "broad directory",
		},
		{
			name:    "duplicate target from same path",
			include: []string{"licenses/aidlc.md"},
			mappings: []contract.TemplatePayloadMapping{
				{Source: "LICENSE", Target: "licenses/aidlc.md"},
			},
			files: []File{file("licenses/aidlc.md")},
			want:  "duplicate template target path",
		},
		{
			name:     "unlisted snapshot file",
			mappings: []contract.TemplatePayloadMapping{{Source: "LICENSE", Target: "licenses/aidlc.md"}},
			files:    []File{file("README.md")},
			want:     "not included by the public template manifest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSnapshot(Snapshot{
				Manifest: manifestWithIncludes(tt.include, tt.mappings),
				Files:    tt.files,
			})
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateSnapshotKeepsSamePathIncludesBackwardCompatible(t *testing.T) {
	snapshot := Snapshot{
		Manifest: manifestWithIncludes([]string{".ai/README.md", "LICENSE"}, nil),
		Files: []File{
			file(".ai/README.md"),
			file("LICENSE"),
		},
	}

	if err := ValidateSnapshot(snapshot); err != nil {
		t.Fatalf("validate same-path snapshot: %v", err)
	}
}

func TestLocalSnapshotReadsMappedSourceAndEmitsTargetPath(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, ".ai/template-manifest.yaml", `schema_version: 1
payload:
  include:
    - source: source\LICENSE.txt
      target: licenses\aidlc.md
policy:
  allow_broad_directories: false
  public_docs_must_be_explicit: true
  reject_absolute_paths: true
  reject_parent_traversal: true
`, 0o644)
	writeSourceFile(t, root, "source/LICENSE.txt", "license from source\n", 0o600)

	snapshot, err := Local{Root: root}.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("local snapshot: %v", err)
	}

	assertSnapshotFile(t, snapshot, "licenses/aidlc.md", "license from source\n", 0o600)
}

func TestArchiveSnapshotReadsMappedSourceAndEmitsTargetPath(t *testing.T) {
	data := zipSource(t, map[string]zipEntry{
		"repo-main/.ai/template-manifest.yaml": {
			content: `schema_version: 1
payload:
  include:
    - source: LICENSE
      target: licenses/aidlc.md
policy:
  allow_broad_directories: false
  public_docs_must_be_explicit: true
  reject_absolute_paths: true
  reject_parent_traversal: true
`,
			mode: 0o644,
		},
		"repo-main/LICENSE": {
			content: "archive license\n",
			mode:    0o600,
		},
	})

	snapshot, err := Archive{Data: data}.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("archive snapshot: %v", err)
	}

	assertSnapshotFile(t, snapshot, "licenses/aidlc.md", "archive license\n", 0o600)
}

func assertEntry(t testing.TB, got ManifestInclude, sourcePath, targetPath string) {
	t.Helper()

	if got.SourcePath != sourcePath || got.TargetPath != targetPath {
		t.Fatalf("entry = %#v, want source %q target %q", got, sourcePath, targetPath)
	}
}

func manifestWithIncludes(include []string, mappings []contract.TemplatePayloadMapping) contract.TemplateManifest {
	return contract.TemplateManifest{
		SchemaVersion: contract.TemplateManifestV1,
		Payload: contract.TemplatePayload{
			Include:         include,
			IncludeMappings: mappings,
		},
		Policy: contract.TemplateManifestPolicy{
			AllowBroadDirectories:    false,
			PublicDocsMustBeExplicit: true,
			RejectAbsolutePaths:      true,
			RejectParentTraversal:    true,
		},
	}
}

func file(name string) File {
	return File{Path: name, Content: []byte(name), Mode: 0o644}
}

func writeSourceFile(t testing.TB, root, name, content string, mode os.FileMode) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", name, err)
	}
}

func assertSnapshotFile(t testing.TB, snapshot Snapshot, path, content string, mode os.FileMode) {
	t.Helper()

	if len(snapshot.Files) != 1 {
		t.Fatalf("snapshot files len = %d, want 1: %#v", len(snapshot.Files), snapshot.Files)
	}
	file := snapshot.Files[0]
	if file.Path != path {
		t.Fatalf("snapshot file path = %q, want %q", file.Path, path)
	}
	if string(file.Content) != content {
		t.Fatalf("snapshot file content = %q, want %q", file.Content, content)
	}
	if file.Mode != mode {
		t.Fatalf("snapshot file mode = %v, want %v", file.Mode, mode)
	}
}

type zipEntry struct {
	content string
	mode    os.FileMode
}

func zipSource(t testing.TB, entries map[string]zipEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, entry := range entries {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := file.Write([]byte(entry.content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}
