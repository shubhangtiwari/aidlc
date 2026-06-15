package cache

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type Querier struct {
	dbPath string
}

var _ model.Querier = (*Querier)(nil)

func NewQuerier(mapDir string) *Querier {
	return &Querier{dbPath: filepath.Join(mapDir, model.SQLiteFilename)}
}

func (q *Querier) Query(ctx context.Context, query string, limit int) ([]model.QueryResult, error) {
	if limit <= 0 || strings.TrimSpace(query) == "" {
		return nil, nil
	}

	db, err := sql.Open(sqliteDriver, q.dbPath)
	if err != nil {
		return nil, fmt.Errorf("open repo-map cache: %w", err)
	}
	defer db.Close()

	queryTerms := model.QueryTerms(query)
	matchQuery := ftsMatchQueryFromTerms(queryTerms)
	if matchQuery == "" {
		return nil, nil
	}
	sourceBoost := sourceChunkRankBoost(queryTerms)

	rows, err := db.QueryContext(ctx, `
		SELECT
			path,
			bm25(repo_map_fts) AS score,
			snippet(repo_map_fts, 4, '', '', '...', 12) AS snippet
		FROM repo_map_fts
		WHERE repo_map_fts MATCH ?
		ORDER BY
			bm25(repo_map_fts)
				* CASE WHEN shard = ? THEN ? ELSE 1.0 END
				* CASE WHEN path GLOB '*_test.*' THEN 0.5 ELSE 1.0 END ASC,
			score ASC,
			path ASC
		LIMIT ?`, matchQuery, model.SourceChunksShard, sourceBoost, candidateLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("query repo-map cache: %w", err)
	}
	defer rows.Close()

	results := make([]model.QueryResult, 0, limit)
	seen := make(map[string]struct{}, limit)
	for rows.Next() {
		var result model.QueryResult
		if err := rows.Scan(&result.Path, &result.Score, &result.Snippet); err != nil {
			return nil, fmt.Errorf("scan repo-map result: %w", err)
		}
		if _, ok := seen[result.Path]; ok {
			continue
		}
		seen[result.Path] = struct{}{}
		results = append(results, result)
		if len(results) == limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo-map results: %w", err)
	}
	return results, nil
}

func candidateLimit(limit int) int {
	if limit < 10 {
		return 100
	}
	return limit * 10
}

func ftsMatchQuery(query string) string {
	return ftsMatchQueryFromTerms(model.QueryTerms(query))
}

func ftsMatchQueryFromTerms(queryTerms []string) string {
	terms := make([]string, 0, len(queryTerms))
	for _, term := range queryTerms {
		terms = append(terms, fmt.Sprintf("%q", term))
	}
	return strings.Join(terms, " ")
}

func sourceChunkRankBoost(queryTerms []string) float64 {
	termSet := make(map[string]struct{}, len(queryTerms))
	for _, term := range queryTerms {
		termSet[term] = struct{}{}
	}
	if hasTerm(termSet, "extract") || hasTerm(termSet, "extraction") {
		return 2.0
	}
	if hasTerm(termSet, "source") && (hasTerm(termSet, "chunk") || hasTerm(termSet, "chunks")) {
		return 2.0
	}
	return 1.0
}

func hasTerm(terms map[string]struct{}, term string) bool {
	_, ok := terms[term]
	return ok
}
