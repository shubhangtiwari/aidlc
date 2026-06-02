package commands

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/generator"
	templatesync "github.com/shubhangtiwari/aidlc/aidlc/internal/sync"
)

func RunUpdateCLI(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if isHelpArg(args) {
		printUpdateUsage(stdout)
		return contract.ExitOK
	}

	previous, err := templatesync.ReadManifest(".")
	if err != nil {
		fmt.Fprintf(stderr, "aidlc update: %v\n", err)
		return contract.ExitUsage
	}

	opts := contract.UpdateOptions{TargetDir: ".", Source: sourceOptionsFromManifest(previous)}
	fs := flag.NewFlagSet("aidlc update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.Source.Kind, "source", opts.Source.Kind, "template source kind: github or local")
	fs.StringVar(&opts.Source.URL, "url", opts.Source.URL, "GitHub repository URL")
	fs.StringVar(&opts.Source.Ref, "ref", opts.Source.Ref, "GitHub ref or local source label")
	fs.StringVar(&opts.Source.Path, "path", opts.Source.Path, "local source path when --source local")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "print planned changes without writing")
	fs.Usage = func() { printUpdateUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return contract.ExitUsage
	}

	result, err := RunUpdate(ctx, opts)
	printCommandResult(stdout, result)
	if err != nil {
		fmt.Fprintf(stderr, "aidlc update: %v\n", err)
		if hasConflict(result.Plan) {
			return contract.ExitConflict
		}
		return contract.ExitUsage
	}
	if hasConflict(result.Plan) {
		return contract.ExitConflict
	}
	return contract.ExitOK
}

func RunUpdate(ctx context.Context, opts contract.UpdateOptions) (CommandResult, error) {
	if opts.TargetDir == "" {
		opts.TargetDir = "."
	}
	previous, err := templatesync.ReadManifest(opts.TargetDir)
	if err != nil {
		return CommandResult{}, err
	}
	if opts.Source.Kind == "" {
		opts.Source = sourceOptionsFromManifest(previous)
	}
	if opts.Source.Kind == "local" {
		if err := ensureDir(opts.Source.Path); err != nil {
			return CommandResult{}, fmt.Errorf("local source path: %w", err)
		}
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
		Mode:             templatesync.ModeUpdate,
		TargetDir:        opts.TargetDir,
		Source:           snapshot,
		PreviousManifest: previous,
	})
	if err != nil {
		return CommandResult{}, err
	}
	result := CommandResult{Mode: templatesync.ModeUpdate, DryRun: opts.DryRun, Plan: plan}
	if opts.DryRun {
		return result, nil
	}
	if hasConflict(plan) {
		return result, nil
	}
	if hasCleanWrites(plan) {
		applied, err := templatesync.ApplyPlan(opts.TargetDir, plan)
		if err != nil {
			return result, err
		}
		result.Written = append(result.Written, applied.Written...)
	}

	ides := updateWorkspaceIDEs(previous)
	if len(ides) > 0 {
		generatedFiles, err := generator.Generate(generator.Options{TargetDir: opts.TargetDir, IDEs: ides})
		if err != nil {
			return result, err
		}
		result.Generated = append(result.Generated, generatedFiles.Written...)
	}

	generated := contract.GenerationRecord{IDE: contract.IDEAll, Version: Version}
	if previous != nil {
		generated = previous.Generated
	}
	manifest := templatesync.ManifestFromPlan(plan, generated, commandMetadata(contract.CommandUpdate, opts.Source))
	manifest.Workspace.IDEs = ides
	if err := templatesync.WriteManifest(opts.TargetDir, manifest); err != nil {
		return result, err
	}
	result.Written = append(result.Written, contract.TargetManifestPath)
	return result, nil
}

func updateWorkspaceIDEs(previous *contract.TargetManifest) []contract.IDE {
	if previous == nil {
		return nil
	}
	return append([]contract.IDE(nil), previous.Workspace.IDEs...)
}

func printUpdateUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc update [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --source github|local   Template source kind (default from aidlc.lock.json, legacy manifest, or github)")
	fmt.Fprintln(w, "  --url URL               GitHub repository URL")
	fmt.Fprintln(w, "  --ref REF               GitHub ref or local source label")
	fmt.Fprintln(w, "  --path PATH             Local source path when --source local")
	fmt.Fprintln(w, "  --dry-run               Print planned changes without writing")
}
