package cache

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

func TestBuilderBuildsSQLiteCacheFromJSONLShards(t *testing.T) {
	t.Parallel()

	mapDir := filepath.Join(t.TempDir(), filepath.FromSlash(model.MapDir))
	writeShard(t, mapDir, model.FilesShard, []model.FileRecord{
		{Path: "internal/auth/service.go", Language: "go", SizeBytes: 120, Lines: 7, ContentHash: "hash-auth"},
		{Path: "internal/billing/service.go", Language: "go", SizeBytes: 80, Lines: 5, ContentHash: "hash-billing"},
	})
	writeShard(t, mapDir, model.DocsShard, []model.DocRecord{
		{Path: "docs/spec/100-auth.md", Kind: "spec", Title: "Auth login", Text: "token validation and login flow"},
	})
	writeShard(t, mapDir, model.BlueprintsShard, []model.BlueprintRecord{
		{Path: "docs/blueprints/auth.md", Module: "auth", Section: "Contracts", Text: "login token validator"},
	})

	if err := NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(mapDir, model.SQLiteFilename)); err != nil {
		t.Fatalf("stat sqlite cache: %v", err)
	}

	db, err := sql.Open(sqliteDriver, filepath.Join(mapDir, model.SQLiteFilename))
	if err != nil {
		t.Fatalf("open sqlite cache: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM repo_map_fts`).Scan(&count); err != nil {
		t.Fatalf("count FTS rows: %v", err)
	}
	if count != 4 {
		t.Fatalf("FTS row count = %d, want 4", count)
	}
}

func TestBuilderReturnsProbeFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fts unavailable")
	builder := newBuilderWithProbe(func(context.Context, *sql.DB) error {
		return wantErr
	})

	err := builder.Build(context.Background(), filepath.Join(t.TempDir(), filepath.FromSlash(model.MapDir)))
	if !errors.Is(err, wantErr) {
		t.Fatalf("Build() error = %v, want %v", err, wantErr)
	}
}

func TestBuilderEmptyCorpusCreatesQueryableCache(t *testing.T) {
	t.Parallel()

	mapDir := filepath.Join(t.TempDir(), filepath.FromSlash(model.MapDir))
	if err := NewBuilder().Build(context.Background(), mapDir); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	results, err := NewQuerier(mapDir).Query(context.Background(), "anything", 10)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Query() len = %d, want 0", len(results))
	}
}

func writeShard[T model.SortableRecord](t testing.TB, mapDir, filename string, records []T) {
	t.Helper()

	if err := os.MkdirAll(mapDir, 0o755); err != nil {
		t.Fatalf("create map dir: %v", err)
	}
	var data bytes.Buffer
	if err := model.WriteJSONL(&data, records); err != nil {
		t.Fatalf("write JSONL %s: %v", filename, err)
	}
	if strings.Contains(data.String(), "\r") {
		t.Fatalf("test JSONL data contains CRLF")
	}
	if err := os.WriteFile(filepath.Join(mapDir, filename), data.Bytes(), 0o644); err != nil {
		t.Fatalf("write shard %s: %v", filename, err)
	}
}
