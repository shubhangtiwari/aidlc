package contract

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskRecordV1JSONShape(t *testing.T) {
	record := TaskRecordV1{
		SchemaVersion: TaskRecordSchemaVersion,
		ID:            "task-1790867975-token-efficient-harness",
		Spec:          "docs/spec/1790867975-token-efficient-harness.md",
		Risk:          TaskRiskHigh,
		ScopeRoot:     ".",
		BaseRevision:  "HEAD~1",
		OwnedFiles:    []string{"aidlc/internal/commands/query.go"},
		ContentDigest: "sha256:owned",
		Acceptance:    []string{"make aidlc-test passes"},
		Decisions: []TaskDecisionRecord{{
			Summary: "Use bounded rg exact search",
			Sources: []string{"docs/adr/1790867975-token-efficient-harness.md"},
			FreshAt: "2026-10-01",
		}},
		Checks: []TaskCheckRecord{{
			Command:       "make aidlc-test",
			Status:        CheckStatusPassed,
			Revision:      "HEAD",
			ContentDigest: "sha256:owned",
		}},
		Review: &TaskReviewRecord{
			Kind:          ReviewKindIndependent,
			Evidence:      "reviewer finding summary",
			Revision:      "HEAD",
			ContentDigest: "sha256:owned",
		},
		OpenNextSteps: []string{},
	}

	got, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `{"schema_version":1,"id":"task-1790867975-token-efficient-harness","spec":"docs/spec/1790867975-token-efficient-harness.md","risk":"high","scope_root":".","base_revision":"HEAD~1","owned_files":["aidlc/internal/commands/query.go"],"content_digest":"sha256:owned","acceptance":["make aidlc-test passes"],"decisions":[{"summary":"Use bounded rg exact search","sources":["docs/adr/1790867975-token-efficient-harness.md"],"fresh_at":"2026-10-01"}],"checks":[{"command":"make aidlc-test","status":"passed","revision":"HEAD","content_digest":"sha256:owned"}],"review":{"kind":"independent","evidence":"reviewer finding summary","revision":"HEAD","content_digest":"sha256:owned"},"open_next_steps":[]}`
	if string(got) != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDigestTaskContent(t *testing.T) {
	entries := []TaskContentEntry{
		{Path: "b.go", Mode: "0644", Content: []byte("package b\n")},
		{Path: "a.go", Mode: "0644", Missing: true},
	}
	reversed := []TaskContentEntry{entries[1], entries[0]}
	got := DigestTaskContent(entries)
	if got == "" || !strings.HasPrefix(got, "sha256:") {
		t.Fatalf("DigestTaskContent() = %q", got)
	}
	if got != DigestTaskContent(reversed) {
		t.Fatal("DigestTaskContent() should be path-order stable")
	}
	changedMode := []TaskContentEntry{
		{Path: "a.go", Mode: "0644", Missing: true},
		{Path: "b.go", Mode: "0755", Content: []byte("package b\n")},
	}
	if got == DigestTaskContent(changedMode) {
		t.Fatal("DigestTaskContent() should include file modes")
	}
	changedMissing := []TaskContentEntry{
		{Path: "a.go", Mode: "0644", Content: []byte{}},
		{Path: "b.go", Mode: "0644", Content: []byte("package b\n")},
	}
	if got == DigestTaskContent(changedMissing) {
		t.Fatal("DigestTaskContent() should distinguish missing files from empty files")
	}
	collidingShape := []TaskContentEntry{{
		Path:    "a.go",
		Mode:    "0644",
		Content: []byte("b.go\x000644\x00present\x00package b\n\xff"),
	}}
	if DigestTaskContent(collidingShape) == DigestTaskContent([]TaskContentEntry{{Path: "a.go", Mode: "0644", Content: []byte("b.go")}, {Path: "0644", Mode: "present", Content: []byte("package b\n")}}) {
		t.Fatal("DigestTaskContent() should length-prefix entries and fields")
	}
}

func TestTaskRecordV1Validation(t *testing.T) {
	tests := []struct {
		name   string
		record TaskRecordV1
		want   string
	}{
		{
			name: "schema",
			record: TaskRecordV1{
				SchemaVersion: 2,
				ID:            "task",
				Risk:          TaskRiskLow,
				ScopeRoot:     ".",
			},
			want: "unsupported",
		},
		{
			name: "risk",
			record: TaskRecordV1{
				SchemaVersion: 1,
				ID:            "task",
				Risk:          "critical",
				ScopeRoot:     ".",
			},
			want: "risk",
		},
		{
			name: "digest",
			record: TaskRecordV1{
				SchemaVersion: 1,
				ID:            "task",
				Risk:          TaskRiskLow,
				ScopeRoot:     ".",
				ContentDigest: "HEAD",
			},
			want: "sha256:",
		},
		{
			name: "check status",
			record: TaskRecordV1{
				SchemaVersion: 1,
				ID:            "task",
				Risk:          TaskRiskLow,
				ScopeRoot:     ".",
				Checks:        []TaskCheckRecord{{Command: "make aidlc-test", Status: "ok"}},
			},
			want: "status",
		},
		{
			name: "review evidence",
			record: TaskRecordV1{
				SchemaVersion: 1,
				ID:            "task",
				Risk:          TaskRiskHigh,
				ScopeRoot:     ".",
				Review:        &TaskReviewRecord{Kind: ReviewKindIndependent},
			},
			want: "review.evidence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.record.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %q, want containing %q", err, tt.want)
			}
		})
	}
}
