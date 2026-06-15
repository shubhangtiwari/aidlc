package cache

import (
	"context"
	"database/sql"
	"fmt"
)

const fts5ProbeSQL = `CREATE VIRTUAL TABLE repo_map_fts5_probe USING fts5(content)`

type fts5Probe func(context.Context, *sql.DB) error

func probeFTS5(ctx context.Context, db *sql.DB) error {
	return probeFTS5WithSQL(ctx, db, fts5ProbeSQL)
}

func probeFTS5WithSQL(ctx context.Context, db *sql.DB, statement string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin FTS5 probe transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("probe FTS5 support: %w", err)
	}
	return nil
}
