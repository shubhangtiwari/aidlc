package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

const (
	TaskRecordSchemaVersion = 1
	TaskRiskLow             = "low"
	TaskRiskMedium          = "medium"
	TaskRiskHigh            = "high"
	CheckStatusPassed       = "passed"
	CheckStatusFailed       = "failed"
	CheckStatusSkipped      = "skipped"
	ReviewKindIndependent   = "independent"
)

type TaskRecordV1 struct {
	SchemaVersion int                  `json:"schema_version"`
	ID            string               `json:"id"`
	Spec          string               `json:"spec,omitempty"`
	Risk          string               `json:"risk"`
	ScopeRoot     string               `json:"scope_root"`
	BaseRevision  string               `json:"base_revision,omitempty"`
	OwnedFiles    []string             `json:"owned_files,omitempty"`
	ContentDigest string               `json:"content_digest,omitempty"`
	Acceptance    []string             `json:"acceptance,omitempty"`
	Decisions     []TaskDecisionRecord `json:"decisions,omitempty"`
	Checks        []TaskCheckRecord    `json:"checks,omitempty"`
	Review        *TaskReviewRecord    `json:"review,omitempty"`
	OpenNextSteps []string             `json:"open_next_steps"`
}

type TaskDecisionRecord struct {
	Summary string   `json:"summary"`
	Sources []string `json:"sources,omitempty"`
	FreshAt string   `json:"fresh_at,omitempty"`
}

type TaskCheckRecord struct {
	Command       string `json:"command"`
	Status        string `json:"status"`
	Revision      string `json:"revision,omitempty"`
	ContentDigest string `json:"content_digest,omitempty"`
}

type TaskReviewRecord struct {
	Kind          string `json:"kind"`
	Evidence      string `json:"evidence"`
	Revision      string `json:"revision,omitempty"`
	ContentDigest string `json:"content_digest,omitempty"`
}

type TaskContentEntry struct {
	Path    string
	Mode    string
	Content []byte
	Missing bool
}

func DigestTaskContent(entries []TaskContentEntry) string {
	sorted := append([]TaskContentEntry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	hash := sha256.New()
	writeDigestField(hash, []byte(fmt.Sprintf("entries:%d", len(sorted))))
	for _, entry := range sorted {
		status := "present"
		if entry.Missing {
			status = "missing"
		}
		writeDigestField(hash, []byte(entry.Path))
		writeDigestField(hash, []byte(entry.Mode))
		writeDigestField(hash, []byte(status))
		if !entry.Missing {
			writeDigestField(hash, entry.Content)
		} else {
			writeDigestField(hash, nil)
		}
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

type digestWriter interface {
	Write([]byte) (int, error)
}

func writeDigestField(hash digestWriter, value []byte) {
	hash.Write([]byte(fmt.Sprintf("%d:", len(value))))
	hash.Write(value)
	hash.Write([]byte(";"))
}

func (r TaskRecordV1) Validate() error {
	if r.SchemaVersion != TaskRecordSchemaVersion {
		return fmt.Errorf("unsupported task record schema_version %d", r.SchemaVersion)
	}
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("task record id is required")
	}
	if !validTaskRisk(r.Risk) {
		return fmt.Errorf("task record risk %q is invalid", r.Risk)
	}
	if strings.TrimSpace(r.ScopeRoot) == "" {
		return fmt.Errorf("task record scope_root is required")
	}
	if r.ContentDigest != "" && !strings.HasPrefix(r.ContentDigest, "sha256:") {
		return fmt.Errorf("task record content_digest must use sha256:")
	}
	for i, decision := range r.Decisions {
		if strings.TrimSpace(decision.Summary) == "" {
			return fmt.Errorf("task record decisions[%d].summary is required", i)
		}
	}
	for i, check := range r.Checks {
		if strings.TrimSpace(check.Command) == "" {
			return fmt.Errorf("task record checks[%d].command is required", i)
		}
		if !validCheckStatus(check.Status) {
			return fmt.Errorf("task record checks[%d].status %q is invalid", i, check.Status)
		}
		if check.ContentDigest != "" && !strings.HasPrefix(check.ContentDigest, "sha256:") {
			return fmt.Errorf("task record checks[%d].content_digest must use sha256:", i)
		}
	}
	if r.Review != nil {
		if strings.TrimSpace(r.Review.Kind) == "" {
			return fmt.Errorf("task record review.kind is required")
		}
		if strings.TrimSpace(r.Review.Evidence) == "" {
			return fmt.Errorf("task record review.evidence is required")
		}
		if r.Review.ContentDigest != "" && !strings.HasPrefix(r.Review.ContentDigest, "sha256:") {
			return fmt.Errorf("task record review.content_digest must use sha256:")
		}
	}
	return nil
}

func validTaskRisk(risk string) bool {
	switch risk {
	case TaskRiskLow, TaskRiskMedium, TaskRiskHigh:
		return true
	default:
		return false
	}
}

func validCheckStatus(status string) bool {
	switch status {
	case CheckStatusPassed, CheckStatusFailed, CheckStatusSkipped:
		return true
	default:
		return false
	}
}
