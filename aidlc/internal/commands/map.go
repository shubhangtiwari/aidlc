package commands

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

type MapOptions struct {
	Dir   string
	Check bool
}

type MapDependencies struct {
	CacheBuilder model.CacheBuilder
}

type MapResult struct {
	MapDir     string
	Files      int
	Imports    int
	Tests      int
	Blueprints int
	Docs       int
	Changes    int
	CacheBuilt bool
	CacheError string
}

func RunMapCLI(ctx context.Context, args []string, stdout, stderr io.Writer, deps MapDependencies) int {
	if isHelpArg(args) {
		printMapUsage(stdout)
		return contract.ExitOK
	}

	opts := MapOptions{Dir: "."}
	fs := flag.NewFlagSet("aidlc map", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.Dir, "dir", opts.Dir, "repository directory to map")
	fs.BoolVar(&opts.Check, "check", false, "check whether docs/map/index.json is fresh")
	fs.Usage = func() { printMapUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return contract.ExitUsage
	}

	if opts.Check {
		status, err := repomap.CheckStaleness(opts.Dir)
		if err != nil {
			fmt.Fprintf(stderr, "aidlc map: %v\n", err)
			return contract.ExitUsage
		}
		printStaleness(stdout, status)
		if status.Fresh {
			return contract.ExitOK
		}
		return contract.ExitConflict
	}

	result, err := RunMap(ctx, opts, deps)
	if err != nil {
		fmt.Fprintf(stderr, "aidlc map: %v\n", err)
		return contract.ExitUsage
	}
	printMapResult(stdout, result)
	return contract.ExitOK
}

func RunMap(ctx context.Context, opts MapOptions, deps MapDependencies) (MapResult, error) {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if deps.CacheBuilder == nil {
		return MapResult{}, fmt.Errorf("repo-map cache builder is required")
	}

	shards, err := repomap.ScanDir(opts.Dir)
	if err != nil {
		return MapResult{}, err
	}
	mapDir := filepath.Join(opts.Dir, model.MapDir)
	if err := repomap.WriteShards(mapDir, *shards); err != nil {
		return MapResult{}, err
	}
	if err := repomap.WriteIndex(mapDir, *shards); err != nil {
		return MapResult{}, err
	}
	result := MapResult{
		MapDir:     filepath.ToSlash(model.MapDir),
		Files:      len(shards.Files),
		Imports:    len(shards.Imports),
		Tests:      len(shards.Tests),
		Blueprints: len(shards.Blueprints),
		Docs:       len(shards.Docs),
		Changes:    len(shards.Changes),
		CacheBuilt: true,
	}
	if err := deps.CacheBuilder.Build(ctx, mapDir); err != nil {
		result.CacheBuilt = false
		result.CacheError = err.Error()
	}
	return result, nil
}

func printMapResult(w io.Writer, result MapResult) {
	fmt.Fprintln(w, "repo map: built")
	fmt.Fprintf(w, "map dir: %s\n", result.MapDir)
	fmt.Fprintf(w, "files: %d\n", result.Files)
	fmt.Fprintf(w, "imports: %d\n", result.Imports)
	fmt.Fprintf(w, "tests: %d\n", result.Tests)
	fmt.Fprintf(w, "blueprints: %d\n", result.Blueprints)
	fmt.Fprintf(w, "docs: %d\n", result.Docs)
	fmt.Fprintf(w, "changes: %d\n", result.Changes)
	if result.CacheBuilt {
		fmt.Fprintf(w, "cache: %s\n", filepath.ToSlash(filepath.Join(model.MapDir, model.SQLiteFilename)))
		return
	}
	fmt.Fprintf(w, "cache: unavailable: %s\n", result.CacheError)
}

func printStaleness(w io.Writer, status repomap.StalenessStatus) {
	if status.Fresh {
		fmt.Fprintln(w, "repo map: fresh")
		return
	}
	fmt.Fprintln(w, "repo map: stale")
	if status.MissingIndex {
		fmt.Fprintf(w, "missing: %s\n", filepath.ToSlash(filepath.Join(model.MapDir, model.IndexFilename)))
	}
	for _, path := range status.Changed {
		fmt.Fprintf(w, "changed: %s\n", path)
	}
	for _, path := range status.Missing {
		fmt.Fprintf(w, "missing: %s\n", path)
	}
	for _, path := range status.Added {
		fmt.Fprintf(w, "added: %s\n", path)
	}
}

func printMapUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc map [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --dir DIR   Repository directory to map (default .)")
	fmt.Fprintln(w, "  --check     Exit 0 when docs/map/index.json is fresh, 1 when stale")
}
