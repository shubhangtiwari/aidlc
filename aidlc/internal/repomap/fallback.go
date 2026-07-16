package repomap

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type FallbackQuerier struct {
	mapDir string
}

var _ model.Querier = (*FallbackQuerier)(nil)

func NewFallbackQuerier(mapDir string) *FallbackQuerier {
	return &FallbackQuerier{mapDir: mapDir}
}

func (q *FallbackQuerier) Query(ctx context.Context, query string, limit int) ([]model.QueryResult, error) {
	return q.QueryShard(ctx, query, limit, "")
}

func (q *FallbackQuerier) QueryShard(ctx context.Context, query string, limit int, shard string) ([]model.QueryResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, nil
	}
	shards, err := selectedShards(shard)
	if err != nil {
		return nil, err
	}
	terms := queryTerms(query)
	if len(terms) == 0 {
		return nil, nil
	}

	matches := map[string]model.QueryResult{}
	for _, name := range shards {
		entries, err := q.loadShard(name)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			score := matchScore(entry.SearchText, terms)
			if score == 0 {
				continue
			}
			existing, ok := matches[entry.Path]
			if ok {
				if score > existing.Score {
					existing.Score = score
				}
				if existing.Snippet == "" {
					existing.Snippet = entry.Snippet
				}
				matches[entry.Path] = existing
				continue
			}
			matches[entry.Path] = model.QueryResult{
				Path:    entry.Path,
				Score:   score,
				Snippet: CompactSnippet(entry.Snippet),
			}
		}
	}

	results := make([]model.QueryResult, 0, len(matches))
	for _, result := range matches {
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (q *FallbackQuerier) QueryPlan(ctx context.Context, plan model.SearchPlanV1) ([]model.QueryResult, error) {
	normalized, err := plan.Normalize()
	if err != nil {
		return nil, err
	}
	if normalized.Limit <= 0 {
		return nil, nil
	}
	entries, err := q.planEntries(normalized)
	if err != nil {
		return nil, err
	}
	terms := planTerms(normalized)
	lists := []rankedList{
		{weight: 1.00, results: textMatches(entries, terms, normalized)},
		{weight: 1.25, results: phraseMatches(entries, normalized)},
		{weight: 1.75, results: pathMatches(entries, normalized)},
		{weight: 2.00, results: symbolMatches(entries, normalized)},
	}
	if normalized.RelationshipDepth > 0 {
		related, err := q.relationshipMatches(ctx, entries, normalized, lists)
		if err != nil {
			return nil, err
		}
		lists = append(lists, rankedList{weight: directRelationshipWeight, results: related})
	}
	return fuseRankedLists(normalized.Limit, lists...), nil
}

type fallbackEntry struct {
	Path       string
	SearchText string
	Snippet    string
	Shard      string
	Language   string
	LineStart  int
	LineEnd    int
	SymbolName string
	SymbolKind string
}

func (q *FallbackQuerier) loadShard(name string) ([]fallbackEntry, error) {
	switch name {
	case model.FilesShard:
		records, err := readFallbackShard[model.FileRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: model.SearchText(record.Path, record.Language, record.ContentHash),
				Snippet:    strings.Join([]string{record.Language, record.ContentHash}, " "),
				Shard:      name,
				Language:   record.Language,
			})
		}
		return entries, nil
	case model.ImportsShard:
		records, err := readFallbackShard[model.ImportRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: model.SearchText(record.Path, record.Language, record.ImportPath),
				Snippet:    record.ImportPath,
				Shard:      name,
				Language:   record.Language,
			})
		}
		return entries, nil
	case model.TestsShard:
		records, err := readFallbackShard[model.TestRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: model.SearchText(record.Path, record.Language, record.TargetPath),
				Snippet:    record.TargetPath,
				Shard:      name,
				Language:   record.Language,
			})
		}
		return entries, nil
	case model.BlueprintsShard:
		records, err := readFallbackShard[model.BlueprintRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: model.SearchText(record.Path, record.Module, record.Section, record.Text),
				Snippet:    strings.Join([]string{record.Module, record.Section}, " "),
				Shard:      name,
			})
		}
		return entries, nil
	case model.DocsShard:
		records, err := readFallbackShard[model.DocRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: model.SearchText(record.Path, record.Kind, record.Title, record.Text),
				Snippet:    record.Title,
				Shard:      name,
			})
		}
		return entries, nil
	case model.ChangesShard:
		records, err := readFallbackShard[model.ChangeRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path:       record.Path,
				SearchText: model.SearchText(record.Path, record.Kind, record.ID, record.Title, record.Status, record.Text),
				Snippet:    strings.Join([]string{record.ID, record.Title, record.Status}, " "),
				Shard:      name,
			})
		}
		return entries, nil
	case model.SourceChunksShard:
		records, err := readFallbackShard[model.SourceChunkRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path: record.Path,
				SearchText: model.SearchText(
					record.Path,
					record.Language,
					fmt.Sprintf("%d", record.StartLine),
					fmt.Sprintf("%d", record.EndLine),
					record.Text,
				),
				Snippet:   record.Text,
				Shard:     name,
				Language:  record.Language,
				LineStart: record.StartLine,
				LineEnd:   record.EndLine,
			})
		}
		return entries, nil
	case model.SymbolsShard:
		records, err := readFallbackShard[model.SymbolRecord](q.mapDir, name)
		if err != nil {
			return nil, err
		}
		entries := make([]fallbackEntry, 0, len(records))
		for _, record := range records {
			entries = append(entries, fallbackEntry{
				Path: record.Path,
				SearchText: model.SearchText(
					record.Path,
					record.Language,
					record.Kind,
					record.Name,
					record.Receiver,
					record.Container,
					fmt.Sprintf("%d", record.StartLine),
					fmt.Sprintf("%d", record.EndLine),
				),
				Snippet:    symbolSnippet(record),
				Shard:      name,
				Language:   record.Language,
				LineStart:  record.StartLine,
				LineEnd:    record.EndLine,
				SymbolName: record.Name,
				SymbolKind: record.Kind,
			})
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("unsupported repo-map shard %q", name)
	}
}

func readFallbackShard[T any](mapDir, name string) ([]T, error) {
	file, err := os.Open(filepath.Join(mapDir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	defer file.Close()

	records, err := model.ReadJSONL[T](file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return records, nil
}

func selectedShards(shard string) ([]string, error) {
	if shard == "" {
		return allShardNames(), nil
	}
	normalized := normalizeShard(shard)
	for _, name := range allShardNames() {
		if normalized == name {
			return []string{name}, nil
		}
	}
	return nil, fmt.Errorf("unknown repo-map shard %q", shard)
}

func allShardNames() []string {
	return []string{
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

func normalizeShard(shard string) string {
	shard = strings.TrimSpace(strings.ToLower(shard))
	if shard == "" || strings.HasSuffix(shard, ".jsonl") {
		return shard
	}
	return shard + ".jsonl"
}

func queryTerms(query string) []string {
	return model.QueryTerms(query)
}

func matchScore(text string, terms []string) float64 {
	text = strings.ToLower(text)
	var score float64
	for _, term := range terms {
		if strings.Contains(text, term) {
			score++
		}
	}
	return score
}

func (q *FallbackQuerier) planEntries(plan model.SearchPlanV1) ([]fallbackEntry, error) {
	shards := plan.Shards
	if len(shards) == 0 {
		shards = allShardNames()
	}
	var entries []fallbackEntry
	for _, shard := range shards {
		loaded, err := q.loadShard(shard)
		if err != nil {
			return nil, err
		}
		for _, entry := range loaded {
			if !planAllowsEntry(plan, entry) {
				continue
			}
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func planAllowsEntry(plan model.SearchPlanV1, entry fallbackEntry) bool {
	if plan.IncludeTests != nil && !*plan.IncludeTests && isLikelyTestPath(entry.Path) {
		return false
	}
	if len(plan.Languages) > 0 && entry.Language != "" {
		found := false
		for _, language := range plan.Languages {
			if entry.Language == language {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func planTerms(plan model.SearchPlanV1) []string {
	return model.QueryTerms(model.SearchText(strings.Join(plan.Terms, " "), strings.Join(plan.Symbols, " "), plan.Question))
}

func textMatches(entries []fallbackEntry, terms []string, plan model.SearchPlanV1) []model.QueryResult {
	if len(terms) == 0 {
		return nil
	}
	results := make([]model.QueryResult, 0, len(entries))
	for _, entry := range entries {
		score := matchScore(entry.SearchText, terms)
		if score == 0 {
			continue
		}
		results = append(results, entryResult(entry, score))
	}
	sortFallbackRank(results)
	return topK(results, candidateLimitForPlan(plan.Limit))
}

func phraseMatches(entries []fallbackEntry, plan model.SearchPlanV1) []model.QueryResult {
	if len(plan.Phrases) == 0 {
		return nil
	}
	results := make([]model.QueryResult, 0, len(entries))
	for _, entry := range entries {
		searchText := strings.ToLower(entry.SearchText)
		var score float64
		for _, phrase := range plan.Phrases {
			if strings.Contains(searchText, strings.ToLower(phrase)) {
				score += 2
			}
		}
		if score > 0 {
			results = append(results, entryResult(entry, score))
		}
	}
	sortFallbackRank(results)
	return topK(results, candidateLimitForPlan(plan.Limit))
}

func pathMatches(entries []fallbackEntry, plan model.SearchPlanV1) []model.QueryResult {
	if len(plan.Paths) == 0 && len(plan.Globs) == 0 {
		return nil
	}
	matches := map[string]model.QueryResult{}
	for _, entry := range entries {
		score := pathScore(entry.Path, plan.Paths, plan.Globs)
		if score == 0 {
			continue
		}
		mergeResult(matches, entryResult(entry, score))
	}
	return sortedMapResults(matches, candidateLimitForPlan(plan.Limit))
}

func symbolMatches(entries []fallbackEntry, plan model.SearchPlanV1) []model.QueryResult {
	if len(plan.Symbols) == 0 {
		return nil
	}
	matches := map[string]model.QueryResult{}
	for _, entry := range entries {
		score := symbolScore(entry, plan.Symbols)
		if score == 0 {
			continue
		}
		mergeResult(matches, entryResult(entry, score))
	}
	return sortedMapResults(matches, candidateLimitForPlan(plan.Limit))
}

func (q *FallbackQuerier) relationshipMatches(ctx context.Context, entries []fallbackEntry, plan model.SearchPlanV1, direct []rankedList) ([]model.QueryResult, error) {
	seeds := fuseRankedLists(candidateLimitForPlan(plan.Limit), direct...)
	if len(seeds) == 0 {
		return nil, nil
	}
	seedPaths := map[string]struct{}{}
	for _, seed := range seeds {
		seedPaths[seed.Path] = struct{}{}
	}
	imports, err := readFallbackShard[model.ImportRecord](q.mapDir, model.ImportsShard)
	if err != nil {
		return nil, err
	}
	tests, err := readFallbackShard[model.TestRecord](q.mapDir, model.TestsShard)
	if err != nil {
		return nil, err
	}
	entryByPath := map[string]fallbackEntry{}
	for _, entry := range entries {
		if _, ok := entryByPath[entry.Path]; !ok {
			entryByPath[entry.Path] = entry
		}
	}
	related := map[string]model.QueryResult{}
	frontier := seedPaths
	for depth := 0; depth < plan.RelationshipDepth; depth++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		next := map[string]struct{}{}
		for _, record := range imports {
			if _, ok := frontier[record.Path]; ok {
				for path := range entryByPath {
					if importRefersToPath(record.ImportPath, path) {
						next[path] = struct{}{}
					}
				}
			}
			for seed := range frontier {
				if importRefersToPath(record.ImportPath, seed) {
					next[record.Path] = struct{}{}
				}
			}
		}
		for _, record := range tests {
			if _, ok := frontier[record.Path]; ok {
				next[record.TargetPath] = struct{}{}
			}
			if _, ok := frontier[record.TargetPath]; ok {
				next[record.Path] = struct{}{}
			}
		}
		for path := range next {
			if _, direct := seedPaths[path]; direct {
				continue
			}
			entry, ok := entryByPath[path]
			if !ok {
				continue
			}
			mergeResult(related, entryResult(entry, 1))
		}
		frontier = next
	}
	return sortedMapResults(related, candidateLimitForPlan(plan.Limit)), nil
}

func entryResult(entry fallbackEntry, score float64) model.QueryResult {
	return model.QueryResult{
		Path:    entry.Path,
		Score:   score,
		Snippet: compactSnippet(snippetSource{StartLine: entry.LineStart, EndLine: entry.LineEnd, Text: entry.Snippet}),
	}
}

func sortFallbackRank(results []model.QueryResult) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Path < results[j].Path
		}
		return results[i].Score > results[j].Score
	})
}

func sortedMapResults(matches map[string]model.QueryResult, limit int) []model.QueryResult {
	results := make([]model.QueryResult, 0, len(matches))
	for _, result := range matches {
		results = append(results, result)
	}
	sortFallbackRank(results)
	return topK(results, limit)
}

func mergeResult(results map[string]model.QueryResult, result model.QueryResult) {
	existing, ok := results[result.Path]
	if !ok || result.Score > existing.Score || (result.Score == existing.Score && result.Snippet < existing.Snippet) {
		results[result.Path] = result
	}
}

func pathScore(candidate string, paths, globs []string) float64 {
	var score float64
	for _, hint := range paths {
		switch {
		case candidate == hint:
			score += 4
		case strings.Contains(candidate, hint):
			score += 2
		case strings.Contains(path.Base(candidate), hint):
			score += 1.5
		}
	}
	for _, glob := range globs {
		if globMatches(glob, candidate) {
			score += 3
		}
	}
	return score
}

func symbolScore(entry fallbackEntry, symbols []string) float64 {
	text := strings.ToLower(model.SearchText(entry.SymbolName, entry.SymbolKind, entry.SearchText))
	var score float64
	for _, symbol := range symbols {
		symbol = strings.ToLower(strings.TrimSpace(symbol))
		if symbol == "" {
			continue
		}
		if strings.EqualFold(entry.SymbolName, symbol) {
			score += 5
			continue
		}
		if strings.Contains(text, symbol) {
			score += 2.5
			continue
		}
		for _, term := range model.QueryTerms(symbol) {
			if strings.Contains(text, term) {
				score++
			}
		}
	}
	return score
}

func globMatches(pattern, candidate string) bool {
	if hasGlobstarSegment(pattern) {
		return recursiveGlobMatches(pattern, candidate)
	}
	if ok, _ := path.Match(pattern, candidate); ok {
		return true
	}
	return false
}

func hasGlobstarSegment(pattern string) bool {
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "**" {
			return true
		}
	}
	return false
}

func recursiveGlobMatches(pattern, candidate string) bool {
	patternSegments := strings.Split(pattern, "/")
	candidateSegments := strings.Split(candidate, "/")
	memo := map[[2]int]bool{}
	visiting := map[[2]int]bool{}
	var match func(int, int) bool
	match = func(pi, ci int) bool {
		key := [2]int{pi, ci}
		if value, ok := memo[key]; ok {
			return value
		}
		if visiting[key] {
			return false
		}
		visiting[key] = true
		defer delete(visiting, key)

		var ok bool
		switch {
		case pi == len(patternSegments):
			ok = ci == len(candidateSegments)
		case patternSegments[pi] == "**":
			ok = match(pi+1, ci) || (ci < len(candidateSegments) && match(pi, ci+1))
		case ci < len(candidateSegments):
			segmentOK, err := path.Match(patternSegments[pi], candidateSegments[ci])
			ok = err == nil && segmentOK && match(pi+1, ci+1)
		}
		memo[key] = ok
		return ok
	}
	return match(0, 0)
}

func isLikelyTestPath(candidate string) bool {
	base := path.Base(candidate)
	return strings.Contains(base, "_test.") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.HasPrefix(base, "test_")
}

func importRefersToPath(importPath, candidate string) bool {
	importPath = strings.Trim(importPath, "/")
	candidate = strings.TrimSuffix(candidate, path.Ext(candidate))
	return candidate == importPath ||
		strings.HasSuffix(candidate, "/"+importPath) ||
		strings.Contains(candidate, importPath+"/")
}

func symbolSnippet(record model.SymbolRecord) string {
	parts := []string{record.Kind}
	if record.Receiver != "" {
		parts = append(parts, record.Receiver)
	}
	if record.Container != "" {
		parts = append(parts, record.Container)
	}
	parts = append(parts, record.Name)
	return strings.Join(parts, " ")
}
