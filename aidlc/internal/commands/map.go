package commands

import (
	"bufio"
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
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
)

type MapOptions struct {
	Dir     string
	Check   bool
	Include []string
}

type MapDependencies struct {
	CacheBuilder  model.CacheBuilder
	Stdin         io.Reader
	IsInteractive func() bool
}

type MapResult struct {
	MapDir     string
	Include    []string
	Files      int
	Imports    int
	Tests      int
	Blueprints int
	Docs       int
	Changes    int
	CacheBuilt bool
	CacheError string
}

type mapDeclinedError struct{}

func (mapDeclinedError) Error() string {
	return "repo-map include confirmation declined; rerun with --include DIR[,DIR...] to choose folders explicitly"
}

func RunMapCLI(ctx context.Context, args []string, stdout, stderr io.Writer, deps MapDependencies) int {
	if isHelpArg(args) {
		printMapUsage(stdout)
		return contract.ExitOK
	}
	deps = mapDependenciesWithDefaults(deps)

	opts := MapOptions{Dir: "."}
	var includeFlag string
	fs := flag.NewFlagSet("aidlc map", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.Dir, "dir", opts.Dir, "repository directory to map")
	fs.StringVar(&includeFlag, "include", "", "comma-separated folders to include in the repo map")
	fs.BoolVar(&opts.Check, "check", false, "check whether docs/map/index.json is fresh")
	fs.Usage = func() { printMapUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return contract.ExitUsage
	}

	explicitInclude, explicitIncludeSet, err := parseIncludeFlag(includeFlag)
	if err != nil {
		fmt.Fprintf(stderr, "aidlc map: %v\n", err)
		return contract.ExitUsage
	}
	if opts.Check && explicitIncludeSet {
		fmt.Fprintln(stderr, "aidlc map: --include cannot be used with --check; --check uses the saved whitelist")
		return contract.ExitUsage
	}

	if opts.Check {
		include, ok, err := templatesync.ReadManifestMapInclude(opts.Dir)
		if err != nil {
			fmt.Fprintf(stderr, "aidlc map: %v\n", err)
			return contract.ExitUsage
		}
		if !ok {
			fmt.Fprintln(stderr, missingIncludeGuidance())
			return contract.ExitUsage
		}
		status, err := repomap.CheckStalenessWithOptions(opts.Dir, repomap.ScanOptions{Include: include})
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

	include, err := resolveMapInclude(opts.Dir, explicitInclude, explicitIncludeSet, stdout, stderr, deps)
	if err != nil {
		fmt.Fprintf(stderr, "aidlc map: %v\n", err)
		if _, ok := err.(mapDeclinedError); ok {
			return contract.ExitConflict
		}
		return contract.ExitUsage
	}
	opts.Include = include
	result, err := RunMap(ctx, opts, deps)
	if err != nil {
		fmt.Fprintf(stderr, "aidlc map: %v\n", err)
		return contract.ExitUsage
	}
	printMapResult(stdout, result)
	return contract.ExitOK
}

func mapDependenciesWithDefaults(deps MapDependencies) MapDependencies {
	if deps.Stdin == nil {
		deps.Stdin = os.Stdin
	}
	if deps.IsInteractive == nil {
		deps.IsInteractive = func() bool {
			file, ok := deps.Stdin.(*os.File)
			if !ok {
				return false
			}
			info, err := file.Stat()
			return err == nil && info.Mode()&os.ModeCharDevice != 0
		}
	}
	return deps
}

func RunMap(ctx context.Context, opts MapOptions, deps MapDependencies) (MapResult, error) {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if deps.CacheBuilder == nil {
		return MapResult{}, fmt.Errorf("repo-map cache builder is required")
	}

	include, err := repomap.NormalizeInclude(opts.Include)
	if err != nil {
		return MapResult{}, err
	}
	shards, err := repomap.ScanDirWithOptions(opts.Dir, repomap.ScanOptions{Include: include})
	if err != nil {
		return MapResult{}, err
	}
	mapDir := filepath.Join(opts.Dir, model.MapDir)
	if err := repomap.WriteShards(mapDir, *shards); err != nil {
		return MapResult{}, err
	}
	if err := repomap.WriteIndexWithOptions(mapDir, *shards, repomap.ScanOptions{Include: include}); err != nil {
		return MapResult{}, err
	}
	result := MapResult{
		MapDir:     filepath.ToSlash(model.MapDir),
		Include:    include,
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

func resolveMapInclude(root string, explicit []string, explicitSet bool, stdout, stderr io.Writer, deps MapDependencies) ([]string, error) {
	if explicitSet {
		if err := templatesync.WriteManifestMapInclude(root, explicit); err != nil {
			return nil, err
		}
		include, _, err := templatesync.ReadManifestMapInclude(root)
		return include, err
	}

	include, ok, err := templatesync.ReadManifestMapInclude(root)
	if err != nil {
		return nil, err
	}
	if ok {
		return include, nil
	}

	if deps.IsInteractive == nil || !deps.IsInteractive() {
		return nil, errors.New(missingIncludeGuidance())
	}
	candidates, err := repomap.DetectIncludeCandidates(root)
	if err != nil {
		return nil, err
	}
	printIncludeCandidates(stdout, candidates)
	confirmed, err := confirmIncludeCandidates(stderr, deps.Stdin)
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return nil, mapDeclinedError{}
	}
	if err := templatesync.WriteManifestMapInclude(root, candidates); err != nil {
		return nil, err
	}
	include, _, err = templatesync.ReadManifestMapInclude(root)
	return include, err
}

func parseIncludeFlag(raw string) ([]string, bool, error) {
	if raw == "" {
		return nil, false, nil
	}
	parts := strings.Split(raw, ",")
	include := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			return nil, true, fmt.Errorf("--include contains an empty path")
		}
		include = append(include, value)
	}
	normalized, err := repomap.NormalizeInclude(include)
	if err != nil {
		return nil, true, err
	}
	return normalized, true, nil
}

func printIncludeCandidates(w io.Writer, candidates []string) {
	fmt.Fprintln(w, "repo map include candidates:")
	if len(candidates) == 0 {
		fmt.Fprintln(w, "  (root files only)")
		return
	}
	for _, candidate := range candidates {
		fmt.Fprintf(w, "  - %s\n", candidate)
	}
}

func confirmIncludeCandidates(w io.Writer, r io.Reader) (bool, error) {
	if r == nil {
		return false, fmt.Errorf("interactive input is unavailable")
	}
	fmt.Fprint(w, "Use these folders for repo-map generation? [y/N] ")
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && len(line) == 0 {
		return false, fmt.Errorf("read include confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func missingIncludeGuidance() string {
	return "repo-map include whitelist is not configured; run aidlc map --include DIR[,DIR...] or run aidlc map interactively to confirm detected folders"
}

func printMapResult(w io.Writer, result MapResult) {
	fmt.Fprintln(w, "repo map: built")
	fmt.Fprintf(w, "map dir: %s\n", result.MapDir)
	if len(result.Include) > 0 {
		fmt.Fprintf(w, "include: %s\n", strings.Join(result.Include, ","))
	}
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
	if status.IncludeMismatch {
		fmt.Fprintln(w, "include: mismatch")
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
	fmt.Fprintln(w, "  --include DIR[,DIR...]")
	fmt.Fprintln(w, "              Folders to include and save in aidlc.lock.json")
	fmt.Fprintln(w, "  --check     Exit 0 when docs/map/index.json is fresh, 1 when stale")
}
