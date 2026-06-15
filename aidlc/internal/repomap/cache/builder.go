package cache

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"

	_ "modernc.org/sqlite"
)

const sqliteDriver = "sqlite"

type Builder struct {
	probe fts5Probe
}

var _ model.CacheBuilder = (*Builder)(nil)

func NewBuilder() *Builder {
	return &Builder{probe: probeFTS5}
}

func newBuilderWithProbe(probe fts5Probe) *Builder {
	return &Builder{probe: probe}
}

func (b *Builder) Build(ctx context.Context, mapDir string) error {
	if b.probe == nil {
		b.probe = probeFTS5
	}
	if err := os.MkdirAll(mapDir, 0o755); err != nil {
		return fmt.Errorf("create map directory: %w", err)
	}

	dbPath := filepath.Join(mapDir, model.SQLiteFilename)
	db, err := sql.Open(sqliteDriver, dbPath)
	if err != nil {
		return fmt.Errorf("open repo-map cache: %w", err)
	}
	defer db.Close()

	if err := b.probe(ctx, db); err != nil {
		return err
	}
	if err := resetSchema(ctx, db); err != nil {
		return err
	}

	entries, err := loadEntries(mapDir)
	if err != nil {
		return err
	}
	return insertEntries(ctx, db, entries)
}

type indexEntry struct {
	Path  string
	Shard string
	Kind  string
	Title string
	Body  string
}

func resetSchema(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`DROP TABLE IF EXISTS repo_map_docs`,
		`DROP TABLE IF EXISTS repo_map_fts`,
		`CREATE TABLE repo_map_docs (
			path TEXT NOT NULL,
			shard TEXT NOT NULL,
			kind TEXT NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL
		)`,
		`CREATE VIRTUAL TABLE repo_map_fts USING fts5(
			path UNINDEXED,
			shard UNINDEXED,
			kind UNINDEXED,
			title,
			body
		)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize repo-map cache schema: %w", err)
		}
	}
	return nil
}

func insertEntries(ctx context.Context, db *sql.DB, entries []indexEntry) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin repo-map cache insert: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	docStmt, err := tx.PrepareContext(ctx, `INSERT INTO repo_map_docs(path, shard, kind, title, body) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare repo-map docs insert: %w", err)
	}
	defer docStmt.Close()

	ftsStmt, err := tx.PrepareContext(ctx, `INSERT INTO repo_map_fts(path, shard, kind, title, body) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare repo-map FTS insert: %w", err)
	}
	defer ftsStmt.Close()

	for _, entry := range entries {
		if _, err := docStmt.ExecContext(ctx, entry.Path, entry.Shard, entry.Kind, entry.Title, entry.Body); err != nil {
			return fmt.Errorf("insert repo-map doc %s: %w", entry.Path, err)
		}
		if _, err := ftsStmt.ExecContext(ctx, entry.Path, entry.Shard, entry.Kind, entry.Title, entry.Body); err != nil {
			return fmt.Errorf("insert repo-map FTS doc %s: %w", entry.Path, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit repo-map cache insert: %w", err)
	}
	return nil
}

func loadEntries(mapDir string) ([]indexEntry, error) {
	var entries []indexEntry

	files, err := readShard[model.FileRecord](mapDir, model.FilesShard)
	if err != nil {
		return nil, err
	}
	for _, record := range files {
		entries = append(entries, indexEntry{
			Path:  record.Path,
			Shard: model.FilesShard,
			Kind:  "file",
			Title: record.Path,
			Body:  strings.Join([]string{record.Path, record.Language, record.ContentHash}, " "),
		})
	}

	imports, err := readShard[model.ImportRecord](mapDir, model.ImportsShard)
	if err != nil {
		return nil, err
	}
	for _, record := range imports {
		entries = append(entries, indexEntry{
			Path:  record.Path,
			Shard: model.ImportsShard,
			Kind:  "import",
			Title: record.ImportPath,
			Body:  strings.Join([]string{record.Path, record.Language, record.ImportPath}, " "),
		})
	}

	tests, err := readShard[model.TestRecord](mapDir, model.TestsShard)
	if err != nil {
		return nil, err
	}
	for _, record := range tests {
		entries = append(entries, indexEntry{
			Path:  record.Path,
			Shard: model.TestsShard,
			Kind:  "test",
			Title: record.TargetPath,
			Body:  strings.Join([]string{record.Path, record.Language, record.TargetPath}, " "),
		})
	}

	blueprints, err := readShard[model.BlueprintRecord](mapDir, model.BlueprintsShard)
	if err != nil {
		return nil, err
	}
	for _, record := range blueprints {
		entries = append(entries, indexEntry{
			Path:  record.Path,
			Shard: model.BlueprintsShard,
			Kind:  "blueprint",
			Title: strings.Join([]string{record.Module, record.Section}, " "),
			Body:  strings.Join([]string{record.Path, record.Module, record.Section, record.Text}, " "),
		})
	}

	docs, err := readShard[model.DocRecord](mapDir, model.DocsShard)
	if err != nil {
		return nil, err
	}
	for _, record := range docs {
		entries = append(entries, indexEntry{
			Path:  record.Path,
			Shard: model.DocsShard,
			Kind:  record.Kind,
			Title: record.Title,
			Body:  strings.Join([]string{record.Path, record.Kind, record.Title, record.Text}, " "),
		})
	}

	changes, err := readShard[model.ChangeRecord](mapDir, model.ChangesShard)
	if err != nil {
		return nil, err
	}
	for _, record := range changes {
		entries = append(entries, indexEntry{
			Path:  record.Path,
			Shard: model.ChangesShard,
			Kind:  record.Kind,
			Title: strings.Join([]string{record.ID, record.Title, record.Status}, " "),
			Body:  strings.Join([]string{record.Path, record.Kind, record.ID, record.Title, record.Status, record.Text}, " "),
		})
	}

	return entries, nil
}

func readShard[T any](mapDir, filename string) ([]T, error) {
	file, err := os.Open(filepath.Join(mapDir, filename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", filename, err)
	}
	defer file.Close()

	records, err := model.ReadJSONL[T](file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filename, err)
	}
	return records, nil
}
