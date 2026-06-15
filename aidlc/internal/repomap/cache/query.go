package cache

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
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

	matchQuery := ftsMatchQuery(query)
	if matchQuery == "" {
		return nil, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT
			path,
			bm25(repo_map_fts) AS score,
			snippet(repo_map_fts, 4, '', '', '...', 12) AS snippet
		FROM repo_map_fts
		WHERE repo_map_fts MATCH ?
		ORDER BY score ASC, path ASC
		LIMIT ?`, matchQuery, candidateLimit(limit))
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
	fields := strings.Fields(query)
	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, `"`)
		if field == "" {
			continue
		}
		terms = append(terms, strconv.Quote(field))
	}
	return strings.Join(terms, " ")
}
