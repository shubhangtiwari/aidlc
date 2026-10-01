package contract

import (
	"fmt"
	"strings"
)

const BenchmarkRecordSchemaVersion = 1

type BenchmarkRecordV1 struct {
	SchemaVersion int                    `json:"schema_version"`
	Command       string                 `json:"command"`
	Revision      string                 `json:"revision,omitempty"`
	ContentDigest string                 `json:"content_digest,omitempty"`
	QuerySet      string                 `json:"query_set"`
	Results       []BenchmarkQueryResult `json:"results,omitempty"`
	Summary       BenchmarkSummary       `json:"summary"`
	HostUsage     *BenchmarkHostUsage    `json:"host_usage,omitempty"`
}

type BenchmarkQueryResult struct {
	Query            string   `json:"query"`
	ExpectedPaths    []string `json:"expected_paths,omitempty"`
	MatchedPaths     []string `json:"matched_paths,omitempty"`
	CriticalMisses   []string `json:"critical_misses,omitempty"`
	RecallAt10       float64  `json:"recall_at_10"`
	PrecisionAt10    float64  `json:"precision_at_10"`
	RepeatedReads    int      `json:"repeated_reads"`
	Retries          int      `json:"retries"`
	OutputBytes      int      `json:"output_bytes"`
	OutputWords      int      `json:"output_words"`
	BaselineBytes    int      `json:"baseline_bytes,omitempty"`
	CompactReduction float64  `json:"compact_reduction,omitempty"`
}

type BenchmarkSummary struct {
	QueryCount        int     `json:"query_count"`
	MeanRecallAt10    float64 `json:"mean_recall_at_10"`
	MeanPrecisionAt10 float64 `json:"mean_precision_at_10"`
	RepeatedReads     int     `json:"repeated_reads"`
	Retries           int     `json:"retries"`
	OutputBytes       int     `json:"output_bytes"`
	OutputWords       int     `json:"output_words"`
	BaselineBytes     int     `json:"baseline_bytes,omitempty"`
	CompactReduction  float64 `json:"compact_reduction,omitempty"`
	CriticalMissCount int     `json:"critical_miss_count,omitempty"`
	HostUsageOptional bool    `json:"host_usage_optional"`
}

type BenchmarkHostUsage struct {
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
	Source       string  `json:"source"`
}

func (r BenchmarkRecordV1) Validate() error {
	if r.SchemaVersion != BenchmarkRecordSchemaVersion {
		return fmt.Errorf("unsupported benchmark record schema_version %d", r.SchemaVersion)
	}
	if strings.TrimSpace(r.Command) == "" {
		return fmt.Errorf("benchmark record command is required")
	}
	if strings.TrimSpace(r.QuerySet) == "" {
		return fmt.Errorf("benchmark record query_set is required")
	}
	if r.ContentDigest != "" && !strings.HasPrefix(r.ContentDigest, "sha256:") {
		return fmt.Errorf("benchmark record content_digest must use sha256:")
	}
	for i, result := range r.Results {
		if strings.TrimSpace(result.Query) == "" {
			return fmt.Errorf("benchmark record results[%d].query is required", i)
		}
		if err := validateUnitInterval("recall_at_10", result.RecallAt10); err != nil {
			return fmt.Errorf("benchmark record results[%d].%w", i, err)
		}
		if err := validateUnitInterval("precision_at_10", result.PrecisionAt10); err != nil {
			return fmt.Errorf("benchmark record results[%d].%w", i, err)
		}
		if result.RepeatedReads < 0 || result.Retries < 0 || result.OutputBytes < 0 || result.OutputWords < 0 {
			return fmt.Errorf("benchmark record results[%d] metrics must be non-negative", i)
		}
	}
	if err := validateUnitInterval("mean_recall_at_10", r.Summary.MeanRecallAt10); err != nil {
		return fmt.Errorf("benchmark record summary.%w", err)
	}
	if err := validateUnitInterval("mean_precision_at_10", r.Summary.MeanPrecisionAt10); err != nil {
		return fmt.Errorf("benchmark record summary.%w", err)
	}
	if r.Summary.QueryCount < 0 || r.Summary.RepeatedReads < 0 || r.Summary.Retries < 0 || r.Summary.OutputBytes < 0 || r.Summary.OutputWords < 0 {
		return fmt.Errorf("benchmark record summary metrics must be non-negative")
	}
	if r.HostUsage != nil && strings.TrimSpace(r.HostUsage.Source) == "" {
		return fmt.Errorf("benchmark record host_usage.source is required")
	}
	return nil
}

func validateUnitInterval(name string, value float64) error {
	if value < 0 || value > 1 {
		return fmt.Errorf("%s must be between 0 and 1", name)
	}
	return nil
}
