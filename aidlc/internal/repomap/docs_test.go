package repomap

import "testing"

func TestScanDocIndexesDocsChangesAndBlueprintSections(t *testing.T) {
	specDocs, specChanges, specBlueprints := ScanDoc("docs/spec/1000000000-add-auth.md", `---
status: approved
---
# Add Auth

Spec text.
`)
	if len(specDocs) != 1 || specDocs[0].Kind != "spec" || specDocs[0].Title != "Add Auth" {
		t.Fatalf("spec docs = %#v", specDocs)
	}
	if len(specChanges) != 1 || specChanges[0].ID != "1000000000-add-auth" || specChanges[0].Status != "approved" {
		t.Fatalf("spec changes = %#v", specChanges)
	}
	if len(specBlueprints) != 0 {
		t.Fatalf("spec blueprints = %#v, want none", specBlueprints)
	}

	_, _, blueprints := ScanDoc("docs/blueprints/core.md", `# Core

Overview.

## Public Contract

API details.
`)
	if len(blueprints) != 2 {
		t.Fatalf("blueprints len = %d, want 2: %#v", len(blueprints), blueprints)
	}
	if blueprints[0].Module != "core" || blueprints[1].Section != "Public Contract" {
		t.Fatalf("blueprints = %#v", blueprints)
	}
}
