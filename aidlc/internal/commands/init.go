package commands

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/generator"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/source"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
)

const (
	DefaultGitHubURL = "https://github.com/shubhangtiwari/aidlc"
	DefaultRef       = "main"
)

var Version = "dev"

func RunInitCLI(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if isHelpArg(args) {
		printInitUsage(stdout)
		return contract.ExitOK
	}

	flagArgs, positionals := splitInitCLIArgs(args)
	fs := flag.NewFlagSet("aidlc init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := contract.InitOptions{TargetDir: ".", Source: defaultSourceOptions()}
	fs.StringVar(&opts.Source.Kind, "source", opts.Source.Kind, "template source kind: github or local")
	fs.StringVar(&opts.Source.URL, "url", opts.Source.URL, "GitHub repository URL")
	fs.StringVar(&opts.Source.Ref, "ref", opts.Source.Ref, "GitHub ref or local source label")
	fs.StringVar(&opts.Source.Path, "path", "", "local source path when --source local")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "print planned changes without writing")
	fs.BoolVar(&opts.Force, "force", false, "overwrite divergent payload files")

	fs.Usage = func() { printInitUsage(stderr) }
	if err := fs.Parse(flagArgs); err != nil {
		return contract.ExitUsage
	}
	if len(positionals) != 1 || fs.NArg() != 0 {
		fs.Usage()
		return contract.ExitUsage
	}
	ide, err := contract.ParseIDE(positionals[0])
	if err != nil {
		fmt.Fprintf(stderr, "aidlc init: %v\n", err)
		return contract.ExitUsage
	}
	opts.IDE = ide

	result, err := RunInit(ctx, opts)
	printCommandResult(stdout, result)
	if err != nil {
		fmt.Fprintf(stderr, "aidlc init: %v\n", err)
		return contract.ExitUsage
	}
	if hasConflict(result.Plan) {
		return contract.ExitConflict
	}
	return contract.ExitOK
}

func splitInitCLIArgs(args []string) ([]string, []string) {
	flagArgs := []string{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flagArgs = append(flagArgs, arg)
			if initFlagNeedsValue(arg) && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		positionals = append(positionals, arg)
	}
	return flagArgs, positionals
}

func initFlagNeedsValue(arg string) bool {
	name := strings.TrimLeft(arg, "-")
	if strings.Contains(name, "=") {
		return false
	}
	switch name {
	case "source", "url", "ref", "path":
		return true
	default:
		return false
	}
}

func RunInit(ctx context.Context, opts contract.InitOptions) (CommandResult, error) {
	if opts.TargetDir == "" {
		opts.TargetDir = "."
	}
	previous, err := templatesync.ReadManifest(opts.TargetDir)
	if err != nil {
		return CommandResult{}, err
	}
	provider, normalizedSource, err := providerFor(opts.Source)
	if err != nil {
		return CommandResult{}, err
	}
	opts.Source = normalizedSource

	snapshot, err := provider.Snapshot(ctx)
	if err != nil {
		return CommandResult{}, err
	}
	plan, err := templatesync.BuildPlan(templatesync.PlanRequest{
		Mode:      templatesync.ModeInit,
		TargetDir: opts.TargetDir,
		Source:    snapshot,
		Force:     opts.Force,
	})
	if err != nil {
		return CommandResult{}, err
	}
	result := CommandResult{Mode: templatesync.ModeInit, DryRun: opts.DryRun, Plan: plan}
	if opts.DryRun {
		return result, nil
	}

	applied, err := templatesync.ApplyPlan(opts.TargetDir, plan)
	if err != nil {
		return result, err
	}
	result.Written = append(result.Written, applied.Written...)

	generated, err := generator.Generate(generator.Options{TargetDir: opts.TargetDir, IDE: opts.IDE})
	if err != nil {
		return result, err
	}
	result.Generated = append(result.Generated, generated.Written...)

	manifest := templatesync.ManifestFromAcceptedPlan(plan, generationRecord(opts.IDE), commandMetadata(contract.CommandInit, opts.Source))
	selection, err := initWorkspaceIDEs(previous, opts.IDE)
	if err != nil {
		return result, err
	}
	manifest.Workspace.IDEs = selection
	if err := templatesync.WriteManifest(opts.TargetDir, manifest); err != nil {
		return result, err
	}
	result.Written = append(result.Written, contract.TargetManifestPath)
	return result, nil
}

type CommandResult struct {
	Mode      templatesync.Mode
	DryRun    bool
	Plan      templatesync.Plan
	Written   []string
	Generated []string
}

func defaultSourceOptions() contract.SourceOptions {
	return contract.SourceOptions{Kind: "github", URL: DefaultGitHubURL, Ref: DefaultRef}
}

func providerFor(opts contract.SourceOptions) (source.Provider, contract.SourceOptions, error) {
	if opts.Kind == "" {
		opts.Kind = "github"
	}
	switch opts.Kind {
	case "local":
		if opts.Path == "" {
			return nil, opts, fmt.Errorf("--path is required when --source local")
		}
		if opts.Ref == "" {
			opts.Ref = "local"
		}
		return source.Local{Root: opts.Path, Source: "local", Ref: opts.Ref, Commit: opts.Ref}, opts, nil
	case "github":
		if opts.URL == "" {
			opts.URL = DefaultGitHubURL
		}
		if opts.Ref == "" {
			opts.Ref = DefaultRef
		}
		owner, repo, err := parseGitHubRepo(opts.URL)
		if err != nil {
			return nil, opts, err
		}
		return source.GitHub{Owner: owner, Repo: repo, Ref: opts.Ref}, opts, nil
	default:
		return nil, opts, fmt.Errorf("unsupported source %q", opts.Kind)
	}
}

func parseGitHubRepo(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", fmt.Errorf("github URL is required")
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://github.com/" + strings.TrimPrefix(trimmed, "github.com/")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("parse github URL: %w", err)
	}
	if parsed.Host != "github.com" {
		return "", "", fmt.Errorf("--url must point to github.com")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("--url must include GitHub owner and repository")
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

func generationRecord(ide contract.IDE) contract.GenerationRecord {
	return contract.GenerationRecord{
		IDE:       ide,
		Version:   CurrentVersion(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func initWorkspaceIDEs(previous *contract.TargetManifest, requested contract.IDE) ([]contract.IDE, error) {
	selection := []contract.IDE{requested}
	if previous != nil {
		selection = append(previous.Workspace.IDEs, requested)
	}
	return contract.NormalizeIDESelection(selection)
}

func printCommandResult(w io.Writer, result CommandResult) {
	if result.Mode == "" {
		return
	}
	fmt.Fprintln(w, "◆ plan")
	for _, decision := range result.Plan.Decisions {
		fmt.Fprintf(w, "%s %s %s\n", decision.State, decision.Path, decision.Reason)
	}
	if len(result.Written) > 0 {
		fmt.Fprintln(w, "✓ written")
		for _, name := range result.Written {
			fmt.Fprintf(w, "write %s %s\n", name, writtenComment(name))
		}
	}
	if len(result.Generated) > 0 {
		fmt.Fprintln(w, "✦ generated")
		for _, name := range result.Generated {
			fmt.Fprintf(w, "generate %s ide\n", name)
		}
	}
}

func writtenComment(name string) string {
	if name == contract.TargetManifestPath {
		return "lock"
	}
	return "payload"
}

func hasConflict(plan templatesync.Plan) bool {
	for _, decision := range plan.Decisions {
		if decision.State == templatesync.StateConflict {
			return true
		}
	}
	return false
}

func hasCleanWrites(plan templatesync.Plan) bool {
	for _, decision := range plan.Decisions {
		if decision.IsWritable() {
			return true
		}
	}
	return false
}

func sourceOptionsFromManifest(manifest *contract.TargetManifest) contract.SourceOptions {
	opts := defaultSourceOptions()
	if manifest == nil {
		return opts
	}
	if manifest.Upstream.Source == "local" {
		opts.Kind = "local"
		opts.URL = ""
		opts.Ref = manifest.Upstream.Ref
		opts.Path = manifest.Metadata["source_path"]
		return opts
	}
	if manifest.Upstream.Source != "" {
		opts.Kind = "github"
		opts.URL = manifest.Upstream.Source
	}
	if manifest.Upstream.Ref != "" {
		opts.Ref = manifest.Upstream.Ref
	}
	return opts
}

func commandMetadata(command contract.CommandName, source contract.SourceOptions) map[string]string {
	metadata := map[string]string{
		"command":     string(command),
		"source_kind": source.Kind,
	}
	if source.URL != "" {
		metadata["source_url"] = source.URL
	}
	if source.Path != "" {
		metadata["source_path"] = source.Path
	}
	return metadata
}

func printInitUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc init <claude|codex|cursor|copilot|windsurf|all> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --source github|local   Template source kind (default github)")
	fmt.Fprintln(w, "  --url URL               GitHub repository URL")
	fmt.Fprintln(w, "  --ref REF               GitHub ref or local source label")
	fmt.Fprintln(w, "  --path PATH             Local source path when --source local")
	fmt.Fprintln(w, "  --dry-run               Print planned changes without writing")
	fmt.Fprintln(w, "  --force                 Overwrite divergent payload files")
}

func isHelpArg(args []string) bool {
	return len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help")
}

func ensureDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}
