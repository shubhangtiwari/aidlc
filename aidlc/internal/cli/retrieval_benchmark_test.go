package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/commands"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/cache"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/search"
	repomaptestdata "github.com/shubhangtiwari/aidlc/aidlc/testdata/repomap"
)

func TestRetrievalBenchmarkUnknownLocationQueries(t *testing.T) {
	root := copyRepoMapFixture(t)
	data, err := repomaptestdata.UnknownLocationQuerySetJSON(root)
	if err != nil {
		t.Fatalf("marshal unknown-location query set: %v", err)
	}
	writeFile(t, filepath.Join(root, "docs", "tasks", "unknown-location-queries.json"), string(data))

	var stdout, stderr discardBuffer
	code := Run(context.Background(), []string{"map", "--dir", root, "--include", "docs,internal,pkg"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("aidlc map code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}

	record, err := commands.RunBenchmarkRetrieval(context.Background(), commands.BenchmarkOptions{
		Dir:     root,
		Queries: "docs/tasks/unknown-location-queries.json",
	}, commands.BenchmarkDependencies{
		Query: commands.QueryDependencies{
			NewCacheQuerier: func(mapDir string) model.Querier {
				return cache.NewQuerier(mapDir)
			},
			ExactSearcher: search.NewExactSearcher(),
		},
	})
	if err != nil {
		t.Fatalf("RunBenchmarkRetrieval() error = %v", err)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("record.Validate() error = %v", err)
	}

	wantReads, wantRetries := repomaptestdata.UnknownLocationActivityTotals()
	if record.Summary.QueryCount < 10 {
		t.Fatalf("query count = %d, want at least 10", record.Summary.QueryCount)
	}
	if record.Summary.RepeatedReads != wantReads || record.Summary.Retries != wantRetries {
		t.Fatalf("activity metrics = reads %d retries %d, want reads %d retries %d", record.Summary.RepeatedReads, record.Summary.Retries, wantReads, wantRetries)
	}
	if record.Summary.MeanRecallAt10 < 0.80 {
		t.Fatalf("mean recall@10 = %.3f, want >= 0.80; misses: %s", record.Summary.MeanRecallAt10, benchmarkMissReport(record.Results))
	}
	if record.Summary.CriticalMissCount > 1 {
		t.Fatalf("critical misses = %d, want <= 1; misses: %s", record.Summary.CriticalMissCount, benchmarkMissReport(record.Results))
	}
	if record.Summary.OutputBytes == 0 || record.Summary.BaselineBytes == 0 {
		t.Fatalf("bytes = output %d baseline %d, want non-zero", record.Summary.OutputBytes, record.Summary.BaselineBytes)
	}
	if record.Summary.CompactReduction < 0.30 {
		t.Fatalf("compact reduction = %.3f, want >= 0.30 from output bytes %d and source-heavy baseline bytes %d", record.Summary.CompactReduction, record.Summary.OutputBytes, record.Summary.BaselineBytes)
	}
	t.Logf("unknown-location benchmark: queries=%d mean_recall@10=%.3f critical_misses=%d output_bytes=%d source_heavy_baseline_bytes=%d reduction=%.1f%% repeated_reads=%d retries=%d",
		record.Summary.QueryCount,
		record.Summary.MeanRecallAt10,
		record.Summary.CriticalMissCount,
		record.Summary.OutputBytes,
		record.Summary.BaselineBytes,
		100*record.Summary.CompactReduction,
		record.Summary.RepeatedReads,
		record.Summary.Retries,
	)
}

func benchmarkMissReport(results []contract.BenchmarkQueryResult) string {
	report := ""
	for _, result := range results {
		if result.RecallAt10 == 1 && len(result.CriticalMisses) == 0 {
			continue
		}
		if report != "" {
			report += "; "
		}
		report += result.Query + " matched="
		report += joinPaths(result.MatchedPaths)
		report += " critical_misses="
		report += joinPaths(result.CriticalMisses)
	}
	return report
}

func joinPaths(paths []string) string {
	if len(paths) == 0 {
		return "[]"
	}
	out := "["
	for i, path := range paths {
		if i > 0 {
			out += " "
		}
		out += path
	}
	return out + "]"
}

type discardBuffer struct{}

func (discardBuffer) Write(p []byte) (int, error) { return len(p), nil }
func (discardBuffer) String() string              { return "" }

func writeFile(t testing.TB, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
