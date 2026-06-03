package sync_test

import (
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/source"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
)

func TestInitPlanningIsAdditive(t *testing.T) {
	target := t.TempDir()
	testutil.WriteFile(t, target, ".ai/README.md", "local edits")
	testutil.WriteFile(t, target, ".ai/models.defaults.toml", "models")

	plan, err := templatesync.BuildPlan(templatesync.PlanRequest{
		Mode:      templatesync.ModeInit,
		TargetDir: target,
		Source: source.Snapshot{
			Manifest: manifestWithLicenseMapping(".ai/README.md", ".ai/models.defaults.toml"),
			Files: []source.File{
				file(".ai/README.md", "upstream"),
				file(".ai/models.defaults.toml", "models"),
				file("licenses/aidlc.md", "license"),
			},
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	states := statesByPath(plan)
	if states[".ai/README.md"] != templatesync.StateConflict {
		t.Fatalf("README state = %s, want conflict", states[".ai/README.md"])
	}
	if states[".ai/models.defaults.toml"] != templatesync.StateSkip {
		t.Fatalf("models state = %s, want skip", states[".ai/models.defaults.toml"])
	}
	if _, ok := states["LICENSE"]; ok {
		t.Fatalf("root LICENSE was planned: %#v", states)
	}
	if states["licenses/aidlc.md"] != templatesync.StateCreate {
		t.Fatalf("licenses/aidlc.md state = %s, want create", states["licenses/aidlc.md"])
	}
	if _, err := templatesync.ApplyPlan(target, plan); err != nil {
		t.Fatalf("apply plan: %v", err)
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "local edits" {
		t.Fatalf("divergent file overwritten: %q", got)
	}
	if got := testutil.ReadFile(t, target, "licenses/aidlc.md"); got != "license" {
		t.Fatalf("created file = %q", got)
	}
}

func TestUpdatePlanningTreatsHistoricalRootLicenseAsRemovedUpstream(t *testing.T) {
	target := t.TempDir()
	testutil.WriteFile(t, target, "LICENSE", "consumer license")

	previous := &contract.TargetManifest{Files: []contract.ManifestFile{
		{Path: "LICENSE", Checksum: templatesync.BytesChecksum([]byte("old aidlc license"))},
	}}
	plan, err := templatesync.BuildPlan(templatesync.PlanRequest{
		Mode:             templatesync.ModeUpdate,
		TargetDir:        target,
		PreviousManifest: previous,
		Source: source.Snapshot{
			Manifest: manifestWithLicenseMapping(),
			Files:    []source.File{file("licenses/aidlc.md", "new aidlc license")},
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	states := statesByPath(plan)
	if states["LICENSE"] != templatesync.StateRemovedUpstream {
		t.Fatalf("LICENSE state = %s, want removed-upstream", states["LICENSE"])
	}
	if states["licenses/aidlc.md"] != templatesync.StateCreate {
		t.Fatalf("licenses/aidlc.md state = %s, want create", states["licenses/aidlc.md"])
	}
	if _, err := templatesync.ApplyPlan(target, plan); err != nil {
		t.Fatalf("apply plan: %v", err)
	}
	if got := testutil.ReadFile(t, target, "LICENSE"); got != "consumer license" {
		t.Fatalf("root LICENSE changed: %q", got)
	}
	if got := testutil.ReadFile(t, target, "licenses/aidlc.md"); got != "new aidlc license" {
		t.Fatalf("mapped license not written: %q", got)
	}
}

func TestUpdatePlanningComparesPreviousLocalAndUpstreamChecksums(t *testing.T) {
	target := t.TempDir()
	testutil.WriteFile(t, target, ".ai/README.md", "old")
	testutil.WriteFile(t, target, ".ai/models.defaults.toml", "local edits")
	testutil.WriteFile(t, target, ".ai/personas/architect.md", "same")
	testutil.WriteFile(t, target, "docs/spec/README.md", "starter")

	previous := &contract.TargetManifest{Files: []contract.ManifestFile{
		{Path: ".ai/README.md", Checksum: templatesync.BytesChecksum([]byte("old"))},
		{Path: ".ai/models.defaults.toml", Checksum: templatesync.BytesChecksum([]byte("old models"))},
		{Path: "docs/spec/README.md", Checksum: templatesync.BytesChecksum([]byte("starter"))},
	}}
	plan, err := templatesync.BuildPlan(templatesync.PlanRequest{
		Mode:             templatesync.ModeUpdate,
		TargetDir:        target,
		PreviousManifest: previous,
		Source: source.Snapshot{
			Manifest: manifest(".ai/README.md", ".ai/models.defaults.toml", ".ai/personas/architect.md"),
			Files: []source.File{
				file(".ai/README.md", "new"),
				file(".ai/models.defaults.toml", "new models"),
				file(".ai/personas/architect.md", "same"),
			},
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	states := statesByPath(plan)
	want := map[string]templatesync.DecisionState{
		".ai/README.md":             templatesync.StateUpdateClean,
		".ai/models.defaults.toml":  templatesync.StateConflict,
		".ai/personas/architect.md": templatesync.StateSkip,
		"docs/spec/README.md":       templatesync.StateRemovedUpstream,
	}
	for name, state := range want {
		if states[name] != state {
			t.Fatalf("%s state = %s, want %s", name, states[name], state)
		}
	}
	if _, err := templatesync.ApplyPlan(target, plan); err != nil {
		t.Fatalf("apply plan: %v", err)
	}
	if got := testutil.ReadFile(t, target, ".ai/README.md"); got != "new" {
		t.Fatalf("clean update not applied: %q", got)
	}
	if got := testutil.ReadFile(t, target, ".ai/models.defaults.toml"); got != "local edits" {
		t.Fatalf("conflict file overwritten: %q", got)
	}
	if got := testutil.ReadFile(t, target, "docs/spec/README.md"); got != "starter" {
		t.Fatalf("removed-upstream file changed: %q", got)
	}
}

func TestPlannerRejectsSourcePathNotInPublicTemplateManifest(t *testing.T) {
	_, err := templatesync.BuildPlan(templatesync.PlanRequest{
		Mode:      templatesync.ModeInit,
		TargetDir: t.TempDir(),
		Source: source.Snapshot{
			Manifest: manifest(".ai/README.md"),
			Files: []source.File{
				file(".ai/README.md", "readme"),
				file("aidlc/internal/source/source.go", "private"),
			},
		},
	})
	if err == nil {
		t.Fatal("expected rejected private source path")
	}
}

func TestPlannerRejectsPrivateManifestInclude(t *testing.T) {
	_, err := templatesync.BuildPlan(templatesync.PlanRequest{
		Mode:      templatesync.ModeInit,
		TargetDir: t.TempDir(),
		Source: source.Snapshot{
			Manifest: manifest("docs/ARCHITECTURE.md"),
			Files:    []source.File{file("docs/ARCHITECTURE.md", "private")},
		},
	})
	if err == nil {
		t.Fatal("expected rejected private manifest include")
	}
}

func statesByPath(plan templatesync.Plan) map[string]templatesync.DecisionState {
	states := make(map[string]templatesync.DecisionState, len(plan.Decisions))
	for _, decision := range plan.Decisions {
		states[decision.Path] = decision.State
	}
	return states
}

func manifest(include ...string) contract.TemplateManifest {
	return contract.TemplateManifest{
		SchemaVersion: contract.TemplateManifestV1,
		Payload: contract.TemplatePayload{
			Include: include,
		},
		Policy: contract.TemplateManifestPolicy{
			AllowBroadDirectories:    false,
			PublicDocsMustBeExplicit: true,
			RejectAbsolutePaths:      true,
			RejectParentTraversal:    true,
		},
	}
}

func manifestWithLicenseMapping(include ...string) contract.TemplateManifest {
	template := manifest(include...)
	template.Payload.IncludeMappings = []contract.TemplatePayloadMapping{
		{Source: "LICENSE", Target: "licenses/aidlc.md"},
	}
	return template
}

func file(name, content string) source.File {
	return source.File{Path: name, Content: []byte(content), Mode: 0o644}
}
