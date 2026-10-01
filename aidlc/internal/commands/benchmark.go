package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type BenchmarkOptions struct {
	Dir     string
	Queries string
	Output  string
}

type BenchmarkDependencies struct {
	Query    QueryDependencies
	RunQuery func(ctx context.Context, opts QueryOptions, deps QueryDependencies) (QueryOutput, error)
}

type benchmarkQuerySet struct {
	Queries   []benchmarkQueryFixture      `json:"queries"`
	HostUsage *contract.BenchmarkHostUsage `json:"host_usage,omitempty"`
}

type benchmarkQueryFixture struct {
	Query         string              `json:"query"`
	Plan          *model.SearchPlanV1 `json:"plan,omitempty"`
	ExpectedPaths []string            `json:"expected_paths"`
	CriticalPaths []string            `json:"critical_paths,omitempty"`
	RepeatedReads int                 `json:"repeated_reads,omitempty"`
	Retries       int                 `json:"retries,omitempty"`
	BaselineBytes int                 `json:"baseline_bytes,omitempty"`
	Metadata      map[string]any      `json:"metadata,omitempty"`
}

func RunBenchmarkCLI(ctx context.Context, args []string, stdout, stderr io.Writer, deps BenchmarkDependencies) int {
	if isHelpArg(args) {
		printBenchmarkUsage(stdout)
		return contract.ExitOK
	}
	if len(args) == 0 {
		printBenchmarkUsage(stderr)
		return contract.ExitUsage
	}
	switch args[0] {
	case "retrieval":
		return runBenchmarkRetrievalCLI(ctx, args[1:], stdout, stderr, deps)
	case "help", "-h", "--help":
		printBenchmarkUsage(stdout)
		return contract.ExitOK
	default:
		fmt.Fprintf(stderr, "aidlc benchmark: unknown benchmark %q\n", args[0])
		printBenchmarkUsage(stderr)
		return contract.ExitUsage
	}
}

func runBenchmarkRetrievalCLI(ctx context.Context, args []string, stdout, stderr io.Writer, deps BenchmarkDependencies) int {
	opts := BenchmarkOptions{Dir: "."}
	fs := flag.NewFlagSet("aidlc benchmark retrieval", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.Dir, "dir", opts.Dir, "repository directory to benchmark")
	fs.StringVar(&opts.Queries, "queries", "", "JSON query set file")
	fs.StringVar(&opts.Output, "output", "", "optional new local JSON output file")
	fs.Usage = func() { printBenchmarkRetrievalUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return contract.ExitUsage
	}
	record, err := RunBenchmarkRetrieval(ctx, opts, deps)
	if err != nil {
		fmt.Fprintf(stderr, "aidlc benchmark retrieval: %v\n", err)
		return contract.ExitUsage
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "aidlc benchmark retrieval: %v\n", err)
		return contract.ExitUsage
	}
	data = append(data, '\n')
	if strings.TrimSpace(opts.Output) != "" {
		if err := writeBenchmarkOutput(opts.Dir, opts.Output, data); err != nil {
			fmt.Fprintf(stderr, "aidlc benchmark retrieval: %v\n", err)
			return contract.ExitUsage
		}
		fmt.Fprintf(stdout, "benchmark written: %s\n", filepath.ToSlash(opts.Output))
		return contract.ExitOK
	}
	fmt.Fprint(stdout, string(data))
	return contract.ExitOK
}

func RunBenchmarkRetrieval(ctx context.Context, opts BenchmarkOptions, deps BenchmarkDependencies) (contract.BenchmarkRecordV1, error) {
	root, err := resolveScopeRoot(opts.Dir)
	if err != nil {
		return contract.BenchmarkRecordV1{}, err
	}
	queryPath, err := cleanRepoPath("--queries", opts.Queries)
	if err != nil {
		return contract.BenchmarkRecordV1{}, err
	}
	if err := ensurePathReadableInside(root, queryPath); err != nil {
		return contract.BenchmarkRecordV1{}, err
	}
	queryBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(queryPath)))
	if err != nil {
		return contract.BenchmarkRecordV1{}, fmt.Errorf("read --queries: %w", err)
	}
	fixture, err := decodeBenchmarkQuerySet(queryBytes)
	if err != nil {
		return contract.BenchmarkRecordV1{}, err
	}

	record := contract.BenchmarkRecordV1{
		SchemaVersion: contract.BenchmarkRecordSchemaVersion,
		Command:       "aidlc benchmark retrieval --dir . --queries " + queryPath,
		QuerySet:      queryPath,
		HostUsage:     fixture.HostUsage,
	}
	if digest, err := digestOwnedFiles(root, []string{queryPath}); err == nil {
		record.ContentDigest = digest
	}
	if rev, err := resolveGitRevision(ctx, root, "HEAD", ValidateDependencies{}); err == nil {
		record.Revision = rev
	}

	for i, query := range fixture.Queries {
		query.Query = strings.TrimSpace(query.Query)
		if query.Query == "" && query.Plan == nil {
			return contract.BenchmarkRecordV1{}, fmt.Errorf("queries[%d] requires query or plan", i)
		}
		expected, err := cleanExpectedPaths(query.ExpectedPaths)
		if err != nil {
			return contract.BenchmarkRecordV1{}, fmt.Errorf("queries[%d]: %w", i, err)
		}
		critical, err := cleanExpectedPaths(query.CriticalPaths)
		if err != nil {
			return contract.BenchmarkRecordV1{}, fmt.Errorf("queries[%d] critical_paths: %w", i, err)
		}
		runQuery := deps.RunQuery
		if runQuery == nil {
			runQuery = RunQueryDetailed
		}
		output, err := runQuery(ctx, QueryOptions{
			TargetDir: root,
			Query:     query.Query,
			Plan:      query.Plan,
			Limit:     10,
			Format:    "tsv",
		}, deps.Query)
		if err != nil {
			return contract.BenchmarkRecordV1{}, fmt.Errorf("queries[%d] query: %w", i, err)
		}
		matched := benchmarkMatchedPaths(output.Text)
		baseline := query.BaselineBytes
		if baseline == 0 {
			baseline = sourceHeavyBaselineBytes(root, expected)
		}
		result := contract.BenchmarkQueryResult{
			Query:            queryLabel(query),
			ExpectedPaths:    expected,
			MatchedPaths:     matched,
			CriticalMisses:   missingCriticalPaths(critical, matched),
			RecallAt10:       recallAt10(expected, matched),
			PrecisionAt10:    precisionAt10(expected, matched),
			RepeatedReads:    query.RepeatedReads,
			Retries:          query.Retries,
			OutputBytes:      len([]byte(output.Text)),
			OutputWords:      len(strings.Fields(output.Text)),
			BaselineBytes:    baseline,
			CompactReduction: compactReduction(len([]byte(output.Text)), baseline),
		}
		record.Results = append(record.Results, result)
	}
	record.Summary = summarizeBenchmarkResults(record.Results)
	record.Summary.HostUsageOptional = true
	if err := record.Validate(); err != nil {
		return contract.BenchmarkRecordV1{}, err
	}
	return record, nil
}

func decodeBenchmarkQuerySet(data []byte) (benchmarkQuerySet, error) {
	var set benchmarkQuerySet
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&set); err == nil && len(set.Queries) > 0 {
		return set, ensureSingleJSONValue(decoder, "query set")
	}
	var list []benchmarkQueryFixture
	decoder = json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&list); err != nil {
		return benchmarkQuerySet{}, fmt.Errorf("query set JSON: %w", err)
	}
	if err := ensureSingleJSONValue(decoder, "query set"); err != nil {
		return benchmarkQuerySet{}, err
	}
	return benchmarkQuerySet{Queries: list}, nil
}

func ensureSingleJSONValue(decoder *json.Decoder, name string) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%s JSON: multiple JSON values", name)
		}
		return fmt.Errorf("%s JSON: %w", name, err)
	}
	return nil
}

func cleanExpectedPaths(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		rel, err := cleanRepoPath("expected path", value)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		out = append(out, rel)
	}
	return out, nil
}

func benchmarkMatchedPaths(text string) []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) == 0 {
			continue
		}
		rel, err := cleanRepoPath("matched path", fields[0])
		if err != nil {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		paths = append(paths, rel)
		if len(paths) == 10 {
			break
		}
	}
	return paths
}

func sourceHeavyBaselineBytes(root string, paths []string) int {
	total := 0
	for _, rel := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil {
			total += len(data)
		}
	}
	return total
}

func queryLabel(query benchmarkQueryFixture) string {
	if strings.TrimSpace(query.Query) != "" {
		return strings.TrimSpace(query.Query)
	}
	if query.Plan != nil && query.Plan.Question != "" {
		return query.Plan.Question
	}
	return "structured search plan"
}

func missingCriticalPaths(critical, matched []string) []string {
	matchedSet := pathSet(matched)
	var misses []string
	for _, expected := range critical {
		if _, ok := matchedSet[expected]; !ok {
			misses = append(misses, expected)
		}
	}
	return misses
}

func recallAt10(expected, matched []string) float64 {
	if len(expected) == 0 {
		return 0
	}
	matchedSet := pathSet(matched)
	hits := 0
	for _, path := range expected {
		if _, ok := matchedSet[path]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(expected))
}

func precisionAt10(expected, matched []string) float64 {
	if len(matched) == 0 {
		return 0
	}
	expectedSet := pathSet(expected)
	hits := 0
	for _, path := range matched {
		if _, ok := expectedSet[path]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(matched))
}

func compactReduction(outputBytes, baselineBytes int) float64 {
	if baselineBytes <= 0 {
		return 0
	}
	reduction := 1 - float64(outputBytes)/float64(baselineBytes)
	if reduction < 0 {
		return 0
	}
	return reduction
}

func summarizeBenchmarkResults(results []contract.BenchmarkQueryResult) contract.BenchmarkSummary {
	var summary contract.BenchmarkSummary
	summary.QueryCount = len(results)
	for _, result := range results {
		summary.MeanRecallAt10 += result.RecallAt10
		summary.MeanPrecisionAt10 += result.PrecisionAt10
		summary.RepeatedReads += result.RepeatedReads
		summary.Retries += result.Retries
		summary.OutputBytes += result.OutputBytes
		summary.OutputWords += result.OutputWords
		summary.BaselineBytes += result.BaselineBytes
		summary.CriticalMissCount += len(result.CriticalMisses)
	}
	if len(results) > 0 {
		summary.MeanRecallAt10 /= float64(len(results))
		summary.MeanPrecisionAt10 /= float64(len(results))
	}
	summary.CompactReduction = compactReduction(summary.OutputBytes, summary.BaselineBytes)
	return summary
}

func writeBenchmarkOutput(root, output string, data []byte) error {
	scopeRoot, err := resolveScopeRoot(root)
	if err != nil {
		return err
	}
	rel, err := cleanRepoPath("--output", output)
	if err != nil {
		return err
	}
	abs := filepath.Join(scopeRoot, filepath.FromSlash(rel))
	if err := ensureOutputParent(scopeRoot, rel); err != nil {
		return err
	}
	file, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("--output already exists: %s", rel)
		}
		return fmt.Errorf("create --output: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write --output: %w", err)
	}
	return nil
}

func ensureOutputParent(root, rel string) error {
	parent := path.Dir(rel)
	if parent == "." {
		return nil
	}
	if err := ensurePathInside(root, parent); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(parent)), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(parent)))
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("--output parent is a symlink: %s", parent)
	}
	return nil
}

func printBenchmarkUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc benchmark <retrieval> [flags]")
	fmt.Fprintln(w, "  aidlc benchmark retrieval --dir DIR --queries PATH [--output PATH]")
}

func printBenchmarkRetrievalUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc benchmark retrieval [--dir DIR] --queries PATH [--output PATH]")
}

func sortedBenchmarkPaths(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
