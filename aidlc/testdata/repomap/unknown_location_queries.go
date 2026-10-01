package repomaptestdata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type UnknownLocationQuery struct {
	Query         string            `json:"query"`
	ExpectedPaths []string          `json:"expected_paths"`
	CriticalPaths []string          `json:"critical_paths,omitempty"`
	RepeatedReads int               `json:"repeated_reads,omitempty"`
	Retries       int               `json:"retries,omitempty"`
	BaselineBytes int               `json:"baseline_bytes,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Activity      []ActivityStep    `json:"-"`
}

type ActivityStep struct {
	Action string
	Target string
}

var UnknownLocationQueries = []UnknownLocationQuery{
	{
		Query:         "How is a user's greeting authorized before it reaches the core formatter?",
		ExpectedPaths: []string{"internal/auth/auth.go", "internal/core/core.go"},
		CriticalPaths: []string{"internal/auth/auth.go"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "docs/blueprints/core.md"},
			ActivityStep{Action: "read", Target: "internal/auth/auth.go"},
		),
	},
	{
		Query:         "Which code normalizes a principal name before composing the greeting?",
		ExpectedPaths: []string{"internal/auth/auth.go", "internal/core/core.go"},
		CriticalPaths: []string{"internal/auth/auth.go"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "internal/auth/auth.go"},
			ActivityStep{Action: "read", Target: "internal/core/core.go"},
		),
	},
	{
		Query:         "Where is the SessionPolicy allowed role data shape declared?",
		ExpectedPaths: []string{"internal/auth/auth.go"},
		CriticalPaths: []string{"internal/auth/auth.go"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "internal/auth/auth.go"},
		),
	},
	{
		Query:         "Find the Greet implementation that trims display names.",
		ExpectedPaths: []string{"internal/core/core.go"},
		CriticalPaths: []string{"internal/core/core.go"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "internal/core/core.go"},
		),
	},
	{
		Query:         "What module owns the public contract for formatting greeting display strings?",
		ExpectedPaths: []string{"docs/blueprints/core.md"},
		CriticalPaths: []string{"docs/blueprints/core.md"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "docs/blueprints/core.md"},
		),
	},
	{
		Query:         "Which ADR says the repo-map cache is a local derived SQLite artifact?",
		ExpectedPaths: []string{"docs/adr/1000000001-use-sqlite.md"},
		CriticalPaths: []string{"docs/adr/1000000001-use-sqlite.md"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "docs/adr/1000000001-use-sqlite.md"},
		),
	},
	{
		Query:         "Where is scanner extraction for approved specs described?",
		ExpectedPaths: []string{"docs/spec/1000000000-add-auth.md"},
		CriticalPaths: []string{"docs/spec/1000000000-add-auth.md"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "docs/spec/1000000000-add-auth.md"},
		),
	},
	{
		Query:         "Find code for StableKey combining parts into a deterministic string.",
		ExpectedPaths: []string{"pkg/util/util.go"},
		CriticalPaths: []string{"pkg/util/util.go"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "pkg/util/util.go"},
		),
	},
	{
		Query:         "Where is the authorization wrapper that calls NormalizePrincipal and Greet?",
		ExpectedPaths: []string{"internal/auth/auth.go"},
		CriticalPaths: []string{"internal/auth/auth.go"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "internal/auth/auth.go"},
		),
	},
	{
		Query:         "Locate NormalizeGreetingName and its strings.TrimSpace behavior.",
		ExpectedPaths: []string{"internal/core/core.go"},
		CriticalPaths: []string{"internal/core/core.go"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "internal/core/core.go"},
		),
	},
	{
		Query:         "Which tests cover auth Authorize and NormalizePrincipal behavior?",
		ExpectedPaths: []string{"internal/auth/auth_test.go"},
		CriticalPaths: []string{"internal/auth/auth_test.go"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "internal/auth/auth_test.go"},
		),
	},
	{
		Query:         "Find the util package test for StableKey output ordering.",
		ExpectedPaths: []string{"pkg/util/util_test.go"},
		CriticalPaths: []string{"pkg/util/util_test.go"},
		Activity: sourceHeavyActivity(
			ActivityStep{Action: "read", Target: "pkg/util/util_test.go"},
		),
	},
}

func UnknownLocationQuerySetJSON(root string) ([]byte, error) {
	queries := make([]UnknownLocationQuery, len(UnknownLocationQueries))
	for i, query := range UnknownLocationQueries {
		baseline, repeated, err := baselineMetrics(root, query.Activity)
		if err != nil {
			return nil, fmt.Errorf("unknown-location query %d baseline: %w", i, err)
		}
		query.RepeatedReads = repeated
		query.Retries = 0
		query.BaselineBytes = baseline
		query.Activity = nil
		query.Metadata = map[string]string{
			"source": "executed fixture file-read script",
		}
		queries[i] = query
	}
	return json.MarshalIndent(struct {
		Queries []UnknownLocationQuery `json:"queries"`
	}{
		Queries: queries,
	}, "", "  ")
}

func UnknownLocationActivityTotals() (int, int) {
	var reads, retries int
	for _, query := range UnknownLocationQueries {
		_, repeated, err := baselineMetrics("", query.Activity)
		if err == nil {
			reads += repeated
		}
	}
	return reads, retries
}

func baselineMetrics(root string, activity []ActivityStep) (int, int, error) {
	var total int
	var repeated int
	seen := map[string]struct{}{}
	for _, step := range activity {
		if step.Action != "read" {
			continue
		}
		if _, ok := seen[step.Target]; ok {
			repeated++
		}
		seen[step.Target] = struct{}{}
		if root == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(step.Target)))
		if err != nil {
			return 0, 0, err
		}
		total += len(data)
	}
	return total, repeated, nil
}

func sourceHeavyActivity(extra ...ActivityStep) []ActivityStep {
	activity := []ActivityStep{
		{Action: "read", Target: "docs/ARCHITECTURE.md"},
		{Action: "read", Target: "docs/spec/1000000000-add-auth.md"},
		{Action: "read", Target: "docs/adr/1000000001-use-sqlite.md"},
		{Action: "read", Target: "docs/blueprints/core.md"},
		{Action: "read", Target: "internal/auth/auth.go"},
		{Action: "read", Target: "internal/auth/auth_test.go"},
		{Action: "read", Target: "internal/core/core.go"},
		{Action: "read", Target: "internal/core/core_test.go"},
		{Action: "read", Target: "pkg/util/util.go"},
		{Action: "read", Target: "pkg/util/util_test.go"},
	}
	return append(activity, extra...)
}
