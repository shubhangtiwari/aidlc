package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

func TestRunBenchmarkRetrievalComputesProxyMetrics(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "aidlc", "internal", "commands", "query.go"), strings.Repeat("source heavy\n", 20))
	writeBenchmarkQueries(t, root, `{
	  "queries": [
	    {
	      "query": "where is query command",
	      "expected_paths": ["aidlc/internal/commands/query.go"],
	      "critical_paths": ["aidlc/internal/commands/query.go"],
	      "repeated_reads": 2,
	      "retries": 1
	    }
	  ],
	  "host_usage": {"input_tokens": 10, "source": "host activity log"}
	}`)

	record, err := RunBenchmarkRetrieval(context.Background(), BenchmarkOptions{
		Dir:     root,
		Queries: "docs/tasks/queries.json",
	}, BenchmarkDependencies{RunQuery: func(ctx context.Context, opts QueryOptions, deps QueryDependencies) (QueryOutput, error) {
		return QueryOutput{Text: "aidlc/internal/commands/query.go\t1\tquery command\nother.go\t0.1\tother\n"}, nil
	}})
	if err != nil {
		t.Fatalf("RunBenchmarkRetrieval() error = %v", err)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("record.Validate() error = %v", err)
	}
	if record.Summary.QueryCount != 1 || record.Summary.MeanRecallAt10 != 1 || record.Summary.MeanPrecisionAt10 != 0.5 {
		t.Fatalf("summary = %#v", record.Summary)
	}
	if record.Summary.RepeatedReads != 2 || record.Summary.Retries != 1 {
		t.Fatalf("script metrics not preserved: %#v", record.Summary)
	}
	if record.Summary.BaselineBytes == 0 || record.Summary.OutputBytes == 0 {
		t.Fatalf("expected source-heavy baseline and output bytes: %#v", record.Summary)
	}
	if record.HostUsage == nil || record.HostUsage.Source != "host activity log" {
		t.Fatalf("host usage = %#v", record.HostUsage)
	}
}

func TestRunBenchmarkRetrievalRejectsInvalidQuerySet(t *testing.T) {
	root := t.TempDir()
	writeBenchmarkQueries(t, root, `[{"query": "bad", "expected_paths": ["../escape"]}]`)

	_, err := RunBenchmarkRetrieval(context.Background(), BenchmarkOptions{
		Dir:     root,
		Queries: "docs/tasks/queries.json",
	}, BenchmarkDependencies{RunQuery: func(ctx context.Context, opts QueryOptions, deps QueryDependencies) (QueryOutput, error) {
		return QueryOutput{}, nil
	}})
	if err == nil {
		t.Fatal("RunBenchmarkRetrieval() error = nil")
	}
	if !strings.Contains(err.Error(), "expected path") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunBenchmarkCLIWritesNewOutputOnly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "aidlc", "internal", "commands", "query.go"), "package commands\n")
	writeBenchmarkQueries(t, root, `[{"query": "query", "expected_paths": ["aidlc/internal/commands/query.go"]}]`)

	deps := BenchmarkDependencies{RunQuery: func(ctx context.Context, opts QueryOptions, deps QueryDependencies) (QueryOutput, error) {
		return QueryOutput{Text: "aidlc/internal/commands/query.go\t1\tquery\n"}, nil
	}}
	var stdout, stderr bytes.Buffer
	code := RunBenchmarkCLI(context.Background(), []string{
		"retrieval",
		"--dir", root,
		"--queries", "docs/tasks/queries.json",
		"--output", "docs/tasks/result.benchmark.json",
	}, &stdout, &stderr, deps)
	if code != contract.ExitOK {
		t.Fatalf("code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	data, err := osReadFile(filepath.Join(root, "docs", "tasks", "result.benchmark.json"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var record contract.BenchmarkRecordV1
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if record.Summary.QueryCount != 1 {
		t.Fatalf("query count = %d", record.Summary.QueryCount)
	}

	stdout.Reset()
	stderr.Reset()
	code = RunBenchmarkCLI(context.Background(), []string{
		"retrieval",
		"--dir", root,
		"--queries", "docs/tasks/queries.json",
		"--output", "docs/tasks/result.benchmark.json",
	}, &stdout, &stderr, deps)
	if code != contract.ExitUsage {
		t.Fatalf("overwrite code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("stderr missing overwrite refusal: %q", stderr.String())
	}
}

func writeBenchmarkQueries(t testing.TB, root, content string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "docs", "tasks", "queries.json"), content)
}

func osReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
