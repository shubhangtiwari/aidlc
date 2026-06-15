package repomap

import (
	"sort"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

const directRelationshipWeight = 0.35

type rankedList struct {
	weight  float64
	results []model.QueryResult
}

type fusedCandidate struct {
	path    string
	score   float64
	snippet string
}

func fuseRankedLists(limit int, lists ...rankedList) []model.QueryResult {
	if limit <= 0 {
		return nil
	}
	candidates := map[string]fusedCandidate{}
	for _, list := range lists {
		if list.weight <= 0 {
			continue
		}
		seenInList := map[string]struct{}{}
		for rank, result := range list.results {
			path := strings.TrimSpace(result.Path)
			if path == "" {
				continue
			}
			if _, ok := seenInList[path]; ok {
				continue
			}
			seenInList[path] = struct{}{}
			candidate := candidates[path]
			candidate.path = path
			candidate.score += list.weight / float64(rank+1)
			if candidate.snippet == "" || (result.Snippet != "" && len(result.Snippet) < len(candidate.snippet)) {
				candidate.snippet = result.Snippet
			}
			if result.Score > 0 {
				candidate.score += list.weight * result.Score * 0.001
			}
			candidates[path] = candidate
		}
	}

	results := make([]model.QueryResult, 0, len(candidates))
	for _, candidate := range candidates {
		results = append(results, model.QueryResult{
			Path:    candidate.path,
			Score:   candidate.score,
			Snippet: CompactSnippet(candidate.snippet),
		})
	}
	sortQueryResults(results)
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func sortQueryResults(results []model.QueryResult) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Path < results[j].Path
		}
		return results[i].Score > results[j].Score
	})
}

func candidateLimitForPlan(limit int) int {
	if limit <= 0 {
		return 0
	}
	if limit < 10 {
		return 50
	}
	return limit * 5
}
