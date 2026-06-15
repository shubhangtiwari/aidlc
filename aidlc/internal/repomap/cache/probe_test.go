package cache

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestProbeFTS5Success(t *testing.T) {
	t.Parallel()

	db, err := sql.Open(sqliteDriver, ":memory:")
	if err != nil {
		t.Fatalf("open sqlite memory db: %v", err)
	}
	defer db.Close()

	if err := probeFTS5(context.Background(), db); err != nil {
		t.Fatalf("probeFTS5() error = %v", err)
	}

	var name string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE name = 'repo_map_fts5_probe'`).Scan(&name)
	if err == nil {
		t.Fatalf("probeFTS5() left probe table %q behind", name)
	}
	if err != sql.ErrNoRows {
		t.Fatalf("probe table lookup error = %v, want sql.ErrNoRows", err)
	}
}

func TestProbeFTS5FailureSimulation(t *testing.T) {
	t.Parallel()

	db, err := sql.Open(sqliteDriver, ":memory:")
	if err != nil {
		t.Fatalf("open sqlite memory db: %v", err)
	}
	defer db.Close()

	err = probeFTS5WithSQL(context.Background(), db, `CREATE VIRTUAL TABLE repo_map_fts5_probe USING no_such_fts_module(content)`)
	if err == nil {
		t.Fatalf("probeFTS5WithSQL() error = nil")
	}
	if !strings.Contains(err.Error(), "probe FTS5 support") {
		t.Fatalf("probeFTS5WithSQL() error = %q, want probe context", err)
	}
}
