package commands

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type QueryDependencies struct {
	NewCacheQuerier func(mapDir string) model.Querier
}

func RunQueryCLI(ctx context.Context, args []string, stdout, stderr io.Writer, deps QueryDependencies) int {
	if isHelpArg(args) {
		printQueryUsage(stdout)
		return contract.ExitOK
	}

	var dir string
	var shard string
	var limit int
	fs := flag.NewFlagSet("aidlc query", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&dir, "dir", ".", "repository root to query")
	fs.IntVar(&limit, "limit", 10, "maximum number of results")
	fs.StringVar(&shard, "shard", "", "restrict query to one JSONL shard")
	fs.Usage = func() { printQueryUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	query := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(query) == "" || limit < 0 {
		fs.Usage()
		return contract.ExitUsage
	}

	text, err := RunQuery(ctx, QueryOptions{
		TargetDir: dir,
		Query:     query,
		Limit:     limit,
		Shard:     shard,
	}, deps)
	if err != nil {
		fmt.Fprintf(stderr, "aidlc query: %v\n", err)
		return contract.ExitUsage
	}
	fmt.Fprint(stdout, text)
	return contract.ExitOK
}

type QueryOptions struct {
	TargetDir string
	Query     string
	Limit     int
	Shard     string
}

func RunQuery(ctx context.Context, opts QueryOptions, deps QueryDependencies) (string, error) {
	if opts.TargetDir == "" {
		opts.TargetDir = "."
	}
	if opts.Limit < 0 {
		return "", fmt.Errorf("--limit must be non-negative")
	}
	mapDir := filepath.Join(opts.TargetDir, filepath.FromSlash(model.MapDir))
	querier := queryQuerier(mapDir, opts.Shard, deps)
	return repomap.NewQueryEngine(querier).QueryText(ctx, opts.Query, opts.Limit, opts.Shard)
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
		}
	}
	return fallback
}

type cacheFallbackQuerier struct {
	primary  model.Querier
	fallback model.Querier
}

func (q cacheFallbackQuerier) Query(ctx context.Context, query string, limit int) ([]model.QueryResult, error) {
	results, err := q.primary.Query(ctx, query, limit)
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return results, err
	}
	return q.fallback.Query(ctx, query, limit)
}

func printQueryUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc query [flags] <search terms>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --dir DIR        Repository root (default .)")
	fmt.Fprintln(w, "  --limit N        Maximum number of results (default 10)")
	fmt.Fprintln(w, "  --shard NAME     Restrict query to one JSONL shard")
}
