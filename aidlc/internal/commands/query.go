package commands

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type QueryDependencies struct {
	NewCacheQuerier func(mapDir string) model.Querier
	ExactSearcher   model.ExactSearcher
}

func RunQueryCLI(ctx context.Context, args []string, stdout, stderr io.Writer, deps QueryDependencies) int {
	if isHelpArg(args) {
		printQueryUsage(stdout)
		return contract.ExitOK
	}

	var dir string
	var shard string
	var limit int
	var planJSON string
	var planFile string
	var diagnostics bool
	var format string
	fs := flag.NewFlagSet("aidlc query", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&dir, "dir", ".", "repository root to query")
	fs.IntVar(&limit, "limit", 10, "maximum number of results")
	fs.StringVar(&shard, "shard", "", "restrict query to one JSONL shard")
	fs.StringVar(&planJSON, "plan-json", "", "SearchPlanV1 JSON to execute")
	fs.StringVar(&planFile, "plan-file", "", "path to a SearchPlanV1 JSON file to execute")
	fs.BoolVar(&diagnostics, "diagnostics", false, "write retrieval diagnostics to stderr")
	fs.StringVar(&format, "format", "tsv", "output format: tsv or json")
	fs.Usage = func() { printQueryUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	query := strings.Join(fs.Args(), " ")
	plan, hasPlan, err := queryPlanInput(queryPlanInputOptions{
		rawQuery: query,
		limit:    limit,
		shard:    shard,
		json:     planJSON,
		file:     planFile,
		args:     fs.Args(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "aidlc query: %v\n", err)
		fs.Usage()
		return contract.ExitUsage
	}

	opts := QueryOptions{
		TargetDir:   dir,
		Query:       query,
		Limit:       limit,
		Shard:       shard,
		Diagnostics: diagnostics,
		Format:      format,
	}
	if hasPlan {
		opts.Plan = &plan
	}
	output, err := RunQueryDetailed(ctx, opts, deps)
	if err != nil {
		fmt.Fprintf(stderr, "aidlc query: %v\n", err)
		return contract.ExitUsage
	}
	if diagnostics {
		for _, diagnostic := range output.Diagnostics {
			fmt.Fprintf(stderr, "aidlc query: %s\n", diagnostic)
		}
	}
	fmt.Fprint(stdout, output.Text)
	return contract.ExitOK
}

type QueryOptions struct {
	TargetDir   string
	Query       string
	Limit       int
	Shard       string
	Plan        *model.SearchPlanV1
	Diagnostics bool
	Format      string
}

func RunQuery(ctx context.Context, opts QueryOptions, deps QueryDependencies) (string, error) {
	output, err := RunQueryDetailed(ctx, opts, deps)
	if err != nil {
		return "", err
	}
	return output.Text, nil
}

type QueryOutput struct {
	Text        string
	Diagnostics []string
}

func RunQueryDetailed(ctx context.Context, opts QueryOptions, deps QueryDependencies) (QueryOutput, error) {
	if opts.TargetDir == "" {
		opts.TargetDir = "."
	}
	if opts.Limit < 0 {
		return QueryOutput{}, fmt.Errorf("--limit must be non-negative")
	}
	format := strings.TrimSpace(strings.ToLower(opts.Format))
	if format == "" {
		format = "tsv"
	}
	if format != "tsv" && format != "json" {
		return QueryOutput{}, fmt.Errorf("--format must be tsv or json")
	}
	mapDir := filepath.Join(opts.TargetDir, filepath.FromSlash(model.MapDir))
	querier := queryQuerier(mapDir, opts.Shard, deps)
	engine := repomap.NewQueryEngine(querier)
	plan := opts.Plan
	rawPlan := false
	if plan == nil {
		compiled, err := model.CompileRawSearchPlan(opts.Query, opts.Limit, opts.Shard)
		if err != nil {
			return QueryOutput{}, err
		}
		if compiled.ExactSearch == nil {
			shard := ""
			if len(compiled.Shards) > 0 {
				shard = compiled.Shards[0]
			}
			results, err := engine.Query(ctx, compiled.Question, compiled.Limit, shard)
			if err != nil {
				return QueryOutput{}, err
			}
			if format == "json" {
				text, err := formatQueryJSON(results, nil)
				if err != nil {
					return QueryOutput{}, err
				}
				return QueryOutput{Text: text}, nil
			}
			return QueryOutput{Text: repomap.FormatQueryResults(results)}, nil
		}
		plan = &compiled
		rawPlan = true
	}
	normalized, err := plan.Normalize()
	if err != nil {
		return QueryOutput{}, err
	}
	results, err := engine.QueryPlan(ctx, normalized)
	if err != nil {
		return QueryOutput{}, err
	}
	var diagnostics []string
	results, diagnostics = augmentWithExactSearch(ctx, opts.TargetDir, normalized, results, deps, opts.Diagnostics || format == "json")
	if rawPlan && len(results) > normalized.Limit {
		results = results[:normalized.Limit]
	}
	if format == "json" {
		text, err := formatQueryJSON(results, diagnostics)
		if err != nil {
			return QueryOutput{}, err
		}
		return QueryOutput{Text: text, Diagnostics: diagnostics}, nil
	}
	return QueryOutput{Text: repomap.FormatQueryResults(results), Diagnostics: diagnostics}, nil
}

type queryPlanInputOptions struct {
	rawQuery string
	limit    int
	shard    string
	json     string
	file     string
	args     []string
}

func queryPlanInput(opts queryPlanInputOptions) (model.SearchPlanV1, bool, error) {
	hasJSON := strings.TrimSpace(opts.json) != ""
	hasFile := strings.TrimSpace(opts.file) != ""
	if hasJSON && hasFile {
		return model.SearchPlanV1{}, false, fmt.Errorf("--plan-json and --plan-file are mutually exclusive")
	}
	if hasJSON || hasFile {
		if len(opts.args) != 0 {
			return model.SearchPlanV1{}, false, fmt.Errorf("raw search terms cannot be combined with --plan-json or --plan-file")
		}
		if strings.TrimSpace(opts.shard) != "" {
			return model.SearchPlanV1{}, false, fmt.Errorf("--shard cannot be combined with --plan-json or --plan-file; set shards in the plan")
		}
		var data []byte
		var source string
		if hasJSON {
			data = []byte(opts.json)
			source = "--plan-json"
		} else {
			source = "--plan-file"
			read, err := os.ReadFile(opts.file)
			if err != nil {
				return model.SearchPlanV1{}, false, fmt.Errorf("%s: %w", source, err)
			}
			data = read
		}
		plan, err := decodeSearchPlan(source, data)
		if err != nil {
			return model.SearchPlanV1{}, false, err
		}
		return plan, true, nil
	}
	if strings.TrimSpace(opts.rawQuery) == "" || opts.limit < 0 {
		return model.SearchPlanV1{}, false, fmt.Errorf("search terms are required and --limit must be non-negative")
	}
	plan, err := model.CompileRawSearchPlan(opts.rawQuery, opts.limit, opts.shard)
	return plan, false, err
}

func decodeSearchPlan(source string, data []byte) (model.SearchPlanV1, error) {
	var plan model.SearchPlanV1
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&plan); err != nil {
		return model.SearchPlanV1{}, fmt.Errorf("%s: malformed JSON: %w", source, err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return model.SearchPlanV1{}, fmt.Errorf("%s: malformed JSON: multiple JSON values", source)
		}
		return model.SearchPlanV1{}, fmt.Errorf("%s: malformed JSON: %w", source, err)
	}
	normalized, err := plan.Normalize()
	if err != nil {
		return model.SearchPlanV1{}, fmt.Errorf("%s: invalid search plan: %w", source, err)
	}
	return normalized, nil
}

func queryQuerier(mapDir, shard string, deps QueryDependencies) model.Querier {
	fallback := repomap.NewFallbackQuerier(mapDir)
	if strings.TrimSpace(shard) != "" {
		return fallback
	}
	if _, err := os.Stat(filepath.Join(mapDir, model.SQLiteFilename)); err == nil && deps.NewCacheQuerier != nil {
		return cacheFallbackQuerier{
			primary:  deps.NewCacheQuerier(mapDir),
			fallback: fallback,
			mapDir:   mapDir,
		}
	}
	return fallback
}

type cacheFallbackQuerier struct {
	primary  model.Querier
	fallback model.Querier
	mapDir   string
}

func (q cacheFallbackQuerier) Query(ctx context.Context, query string, limit int) ([]model.QueryResult, error) {
	results, err := q.primary.Query(ctx, query, limit)
	if err == nil && len(results) > 0 {
		return results, err
	}
	if err == nil {
		return q.fallback.Query(ctx, query, limit)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return results, err
	}
	return q.fallback.Query(ctx, query, limit)
}

func (q cacheFallbackQuerier) QueryShard(ctx context.Context, query string, limit int, shard string) ([]model.QueryResult, error) {
	filtered, ok := q.fallback.(interface {
		QueryShard(context.Context, string, int, string) ([]model.QueryResult, error)
	})
	if !ok {
		return nil, fmt.Errorf("repo-map querier does not support shard filtering")
	}
	return filtered.QueryShard(ctx, query, limit, shard)
}

func (q cacheFallbackQuerier) QueryPlan(ctx context.Context, plan model.SearchPlanV1) ([]model.QueryResult, error) {
	normalized, err := plan.Normalize()
	if err != nil {
		return nil, err
	}
	planned, ok := q.fallback.(interface {
		QueryPlan(context.Context, model.SearchPlanV1) ([]model.QueryResult, error)
	})
	if !ok {
		return nil, fmt.Errorf("repo-map querier does not support search plans")
	}
	fallbackResults, fallbackErr := planned.QueryPlan(ctx, normalized)
	if fallbackErr != nil {
		return nil, fallbackErr
	}

	var cacheResults []model.QueryResult
	cachePlan := normalized
	cachePlan.Shards = nil
	textResults, textErr := repomap.NewQueryEngine(q.primary).QueryPlan(ctx, cachePlan)
	switch {
	case textErr == nil:
		cacheResults = textResults
	case errors.Is(textErr, context.Canceled), errors.Is(textErr, context.DeadlineExceeded):
		return nil, textErr
	}
	if len(cacheResults) > 0 && cachePlanNeedsRecordFilter(normalized) {
		allowedPaths, err := queryPlanAllowedPaths(q.mapDir, normalized)
		if err != nil {
			return nil, err
		}
		cacheResults = filterQueryResultsByPath(cacheResults, allowedPaths)
	}
	if len(cacheResults) == 0 {
		return fallbackResults, nil
	}
	return fuseCacheFallbackPlanResults(normalized.Limit, cacheResults, fallbackResults), nil
}

func cachePlanNeedsRecordFilter(plan model.SearchPlanV1) bool {
	return len(plan.Shards) > 0 || len(plan.Languages) > 0 || (plan.IncludeTests != nil && !*plan.IncludeTests)
}

func queryPlanAllowedPaths(mapDir string, plan model.SearchPlanV1) (map[string]struct{}, error) {
	shards := plan.Shards
	if len(shards) == 0 {
		shards = []string{
			model.FilesShard,
			model.ImportsShard,
			model.TestsShard,
			model.BlueprintsShard,
			model.DocsShard,
			model.ChangesShard,
			model.SourceChunksShard,
			model.SymbolsShard,
		}
	}
	paths := map[string]struct{}{}
	for _, shard := range shards {
		entries, err := queryShardRecordEntries(mapDir, shard)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if queryPlanAllowsRecord(plan, entry.path, entry.language) {
				paths[entry.path] = struct{}{}
			}
		}
	}
	return paths, nil
}

type queryRecordEntry struct {
	path     string
	language string
}

func queryShardRecordEntries(mapDir, shard string) ([]queryRecordEntry, error) {
	switch shard {
	case model.FilesShard:
		return queryRecordEntries(mapDir, shard, func(record model.FileRecord) queryRecordEntry {
			return queryRecordEntry{path: record.Path, language: record.Language}
		})
	case model.ImportsShard:
		return queryRecordEntries(mapDir, shard, func(record model.ImportRecord) queryRecordEntry {
			return queryRecordEntry{path: record.Path, language: record.Language}
		})
	case model.TestsShard:
		return queryRecordEntries(mapDir, shard, func(record model.TestRecord) queryRecordEntry {
			return queryRecordEntry{path: record.Path, language: record.Language}
		})
	case model.BlueprintsShard:
		return queryRecordEntries(mapDir, shard, func(record model.BlueprintRecord) queryRecordEntry {
			return queryRecordEntry{path: record.Path}
		})
	case model.DocsShard:
		return queryRecordEntries(mapDir, shard, func(record model.DocRecord) queryRecordEntry {
			return queryRecordEntry{path: record.Path}
		})
	case model.ChangesShard:
		return queryRecordEntries(mapDir, shard, func(record model.ChangeRecord) queryRecordEntry {
			return queryRecordEntry{path: record.Path}
		})
	case model.SourceChunksShard:
		return queryRecordEntries(mapDir, shard, func(record model.SourceChunkRecord) queryRecordEntry {
			return queryRecordEntry{path: record.Path, language: record.Language}
		})
	case model.SymbolsShard:
		return queryRecordEntries(mapDir, shard, func(record model.SymbolRecord) queryRecordEntry {
			return queryRecordEntry{path: record.Path, language: record.Language}
		})
	default:
		return nil, fmt.Errorf("unsupported repo-map shard %q", shard)
	}
}

func queryRecordEntries[T any](mapDir, shard string, entryOf func(T) queryRecordEntry) ([]queryRecordEntry, error) {
	file, err := os.Open(filepath.Join(mapDir, shard))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", shard, err)
	}
	defer file.Close()

	records, err := model.ReadJSONL[T](file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", shard, err)
	}
	entries := make([]queryRecordEntry, 0, len(records))
	for _, record := range records {
		entry := entryOf(record)
		entry.path = strings.TrimSpace(entry.path)
		entry.language = strings.TrimSpace(strings.ToLower(entry.language))
		if entry.path != "" {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func queryPlanAllowsRecord(plan model.SearchPlanV1, recordPath, language string) bool {
	if plan.IncludeTests != nil && !*plan.IncludeTests && queryLikelyTestPath(recordPath) {
		return false
	}
	if len(plan.Languages) > 0 && language != "" {
		for _, allowed := range plan.Languages {
			if language == allowed {
				return true
			}
		}
		return false
	}
	return true
}

func queryLikelyTestPath(candidate string) bool {
	base := path.Base(candidate)
	return strings.Contains(base, "_test.") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.HasPrefix(base, "test_")
}

func filterQueryResultsByPath(results []model.QueryResult, allowed map[string]struct{}) []model.QueryResult {
	if len(allowed) == 0 {
		return nil
	}
	filtered := results[:0]
	for _, result := range results {
		if _, ok := allowed[result.Path]; ok {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

type weightedQueryResult struct {
	path    string
	score   float64
	snippet string
}

func fuseCacheFallbackPlanResults(limit int, cacheResults, fallbackResults []model.QueryResult) []model.QueryResult {
	if limit <= 0 {
		return nil
	}
	candidates := map[string]weightedQueryResult{}
	addWeightedPlanResults(candidates, cacheResults, 1.0)
	addWeightedPlanResults(candidates, fallbackResults, 2.0)

	results := make([]model.QueryResult, 0, len(candidates))
	for _, candidate := range candidates {
		results = append(results, model.QueryResult{
			Path:    candidate.path,
			Score:   candidate.score,
			Snippet: repomap.CompactSnippet(candidate.snippet),
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Path < results[j].Path
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func addWeightedPlanResults(candidates map[string]weightedQueryResult, results []model.QueryResult, weight float64) {
	seen := map[string]struct{}{}
	for rank, result := range results {
		path := strings.TrimSpace(result.Path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		candidate := candidates[path]
		candidate.path = path
		candidate.score += weight / float64(rank+1)
		if result.Score > 0 {
			candidate.score += weight * result.Score * 0.001
		}
		if candidate.snippet == "" || (result.Snippet != "" && len(result.Snippet) < len(candidate.snippet)) {
			candidate.snippet = result.Snippet
		}
		candidates[path] = candidate
	}
}

func augmentWithExactSearch(ctx context.Context, root string, plan model.SearchPlanV1, mapResults []model.QueryResult, deps QueryDependencies, includeDiagnostics bool) ([]model.QueryResult, []string) {
	if plan.ExactSearch == nil || !plan.ExactSearch.Enabled {
		return mapResults, nil
	}
	var diagnostics []string
	if deps.ExactSearcher == nil {
		if includeDiagnostics {
			diagnostics = append(diagnostics, "exact search unavailable; using repo-map results only")
		}
		return mapResults, diagnostics
	}
	response, err := deps.ExactSearcher.SearchExact(ctx, model.ExactSearchRequest{
		Root:       root,
		Literals:   plan.ExactSearch.Literals,
		Paths:      exactSearchPaths(plan),
		MaxResults: plan.ExactSearch.MaxResults,
		MaxBytes:   plan.ExactSearch.MaxBytes,
		TimeoutMS:  plan.ExactSearch.TimeoutMS,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			diagnostics = append(diagnostics, "exact search timed out; using repo-map results only")
		} else if includeDiagnostics {
			diagnostics = append(diagnostics, fmt.Sprintf("exact search failed: %v", err))
		}
		return mapResults, diagnostics
	}
	if includeDiagnostics {
		diagnostics = append(diagnostics, response.Diagnostics...)
		if response.Truncated {
			diagnostics = append(diagnostics, "exact search truncated by configured limits")
		}
	}
	exactResults := exactSearchQueryResults(response.Results)
	if len(exactResults) == 0 {
		return mapResults, diagnostics
	}
	return fuseCacheFallbackPlanResults(plan.Limit, mapResults, exactResults), diagnostics
}

func exactSearchPaths(plan model.SearchPlanV1) []string {
	if plan.ExactSearch == nil {
		return nil
	}
	paths := append([]string{}, plan.ExactSearch.Paths...)
	paths = append(paths, plan.Paths...)
	return compactStrings(paths)
}

func exactSearchQueryResults(results []model.ExactSearchResult) []model.QueryResult {
	byPath := map[string]model.QueryResult{}
	for _, result := range results {
		path := strings.TrimSpace(result.Path)
		if path == "" {
			continue
		}
		snippet := result.Snippet
		if snippet == "" {
			snippet = result.Match
		}
		if result.Line > 0 {
			snippet = fmt.Sprintf("L%d %s", result.Line, snippet)
		}
		existing, ok := byPath[path]
		if !ok {
			byPath[path] = model.QueryResult{Path: path, Score: 1, Snippet: snippet}
			continue
		}
		existing.Score++
		if existing.Snippet == "" || (snippet != "" && len(snippet) < len(existing.Snippet)) {
			existing.Snippet = snippet
		}
		byPath[path] = existing
	}
	resultsOut := make([]model.QueryResult, 0, len(byPath))
	for _, result := range byPath {
		resultsOut = append(resultsOut, result)
	}
	sort.Slice(resultsOut, func(i, j int) bool {
		if resultsOut[i].Score == resultsOut[j].Score {
			return resultsOut[i].Path < resultsOut[j].Path
		}
		return resultsOut[i].Score > resultsOut[j].Score
	})
	return resultsOut
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func formatQueryJSON(results []model.QueryResult, diagnostics []string) (string, error) {
	payload := struct {
		Results     []model.QueryResult `json:"results"`
		Diagnostics []string            `json:"diagnostics,omitempty"`
	}{
		Results:     results,
		Diagnostics: diagnostics,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

func printQueryUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc query [flags] <search terms>")
	fmt.Fprintln(w, "       aidlc query [flags] --plan-json JSON")
	fmt.Fprintln(w, "       aidlc query [flags] --plan-file PATH")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --diagnostics   Write retrieval diagnostics to stderr")
	fmt.Fprintln(w, "  --dir DIR        Repository root (default .)")
	fmt.Fprintln(w, "  --format FORMAT  Output format: tsv or json (default tsv)")
	fmt.Fprintln(w, "  --limit N        Maximum number of results (default 10)")
	fmt.Fprintln(w, "  --plan-file PATH Execute SearchPlanV1 JSON from PATH")
	fmt.Fprintln(w, "  --plan-json JSON Execute SearchPlanV1 JSON")
	fmt.Fprintln(w, "  --shard NAME     Restrict query to one JSONL shard")
}
