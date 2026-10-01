package contract

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBenchmarkRecordV1JSONShape(t *testing.T) {
	record := BenchmarkRecordV1{
		SchemaVersion: BenchmarkRecordSchemaVersion,
		Command:       "aidlc benchmark retrieval --dir . --queries queries.json",
		Revision:      "HEAD",
		ContentDigest: "sha256:owned",
		QuerySet:      "aidlc/testdata/repomap/unknown_location_queries.go",
		Results: []BenchmarkQueryResult{{
			Query:            "where is compact guidance generated",
			ExpectedPaths:    []string{"aidlc/internal/generator/generator.go"},
			MatchedPaths:     []string{"aidlc/internal/generator/generator.go"},
			RecallAt10:       1,
			PrecisionAt10:    0.1,
			RepeatedReads:    2,
			Retries:          1,
			OutputBytes:      1200,
			OutputWords:      160,
			BaselineBytes:    2400,
			CompactReduction: 0.5,
		}},
		Summary: BenchmarkSummary{
			QueryCount:        1,
			MeanRecallAt10:    1,
			MeanPrecisionAt10: 0.1,
			RepeatedReads:     2,
			Retries:           1,
			OutputBytes:       1200,
			OutputWords:       160,
			BaselineBytes:     2400,
			CompactReduction:  0.5,
			HostUsageOptional: true,
		},
		HostUsage: &BenchmarkHostUsage{
			InputTokens:  100,
			OutputTokens: 20,
			CostUSD:      0.01,
			Source:       "host activity log",
		},
	}

	got, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `{"schema_version":1,"command":"aidlc benchmark retrieval --dir . --queries queries.json","revision":"HEAD","content_digest":"sha256:owned","query_set":"aidlc/testdata/repomap/unknown_location_queries.go","results":[{"query":"where is compact guidance generated","expected_paths":["aidlc/internal/generator/generator.go"],"matched_paths":["aidlc/internal/generator/generator.go"],"recall_at_10":1,"precision_at_10":0.1,"repeated_reads":2,"retries":1,"output_bytes":1200,"output_words":160,"baseline_bytes":2400,"compact_reduction":0.5}],"summary":{"query_count":1,"mean_recall_at_10":1,"mean_precision_at_10":0.1,"repeated_reads":2,"retries":1,"output_bytes":1200,"output_words":160,"baseline_bytes":2400,"compact_reduction":0.5,"host_usage_optional":true},"host_usage":{"input_tokens":100,"output_tokens":20,"cost_usd":0.01,"source":"host activity log"}}`
	if string(got) != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestBenchmarkRecordV1Validation(t *testing.T) {
	tests := []struct {
		name   string
		record BenchmarkRecordV1
		want   string
	}{
		{
			name:   "schema",
			record: BenchmarkRecordV1{SchemaVersion: 2, Command: "aidlc benchmark", QuerySet: "queries.json"},
			want:   "unsupported",
		},
		{
			name:   "digest",
			record: BenchmarkRecordV1{SchemaVersion: 1, Command: "aidlc benchmark", QuerySet: "queries.json", ContentDigest: "HEAD"},
			want:   "sha256:",
		},
		{
			name:   "recall",
			record: BenchmarkRecordV1{SchemaVersion: 1, Command: "aidlc benchmark", QuerySet: "queries.json", Results: []BenchmarkQueryResult{{Query: "q", RecallAt10: 1.1}}},
			want:   "recall_at_10",
		},
		{
			name:   "negative metrics",
			record: BenchmarkRecordV1{SchemaVersion: 1, Command: "aidlc benchmark", QuerySet: "queries.json", Results: []BenchmarkQueryResult{{Query: "q", RepeatedReads: -1}}},
			want:   "non-negative",
		},
		{
			name:   "host usage source",
			record: BenchmarkRecordV1{SchemaVersion: 1, Command: "aidlc benchmark", QuerySet: "queries.json", HostUsage: &BenchmarkHostUsage{InputTokens: 1}},
			want:   "host_usage.source",
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
