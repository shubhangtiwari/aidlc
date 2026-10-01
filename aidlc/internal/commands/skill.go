package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/skills"
)

func RunSkillCLI(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	_ = ctx
	if len(args) == 0 || isHelpArg(args) {
		printSkillUsage(stdout)
		if len(args) == 0 {
			return contract.ExitUsage
		}
		return contract.ExitOK
	}
	switch args[0] {
	case "install":
		return runSkillInstallCLI(args[1:], stdout, stderr)
	case "list":
		return runSkillListCLI(args[1:], stdout, stderr)
	case "remove":
		return runSkillRemoveCLI(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "aidlc skill: unknown subcommand %q\n", args[0])
		printSkillUsage(stderr)
		return contract.ExitUsage
	}
}

func runSkillInstallCLI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("aidlc skill install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	targetDir := "."
	fs.StringVar(&targetDir, "dir", targetDir, "target repository directory")
	fs.Usage = func() { printSkillInstallUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return contract.ExitUsage
	}
	result, err := skills.Install(skills.InstallOptions{TargetDir: targetDir, SourceDir: fs.Arg(0)})
	if err != nil {
		fmt.Fprintf(stderr, "aidlc skill install: %v\n", err)
		return contract.ExitUsage
	}
	fmt.Fprintf(stdout, "installed %s\n", result.Skill.Name)
	return contract.ExitOK
}

func runSkillListCLI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("aidlc skill list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	targetDir := "."
	format := "text"
	fs.StringVar(&targetDir, "dir", targetDir, "target repository directory")
	fs.StringVar(&format, "format", format, "output format: text or json")
	fs.Usage = func() { printSkillListUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return contract.ExitUsage
	}
	result, err := skills.List(skills.ListOptions{TargetDir: targetDir})
	if err != nil {
		fmt.Fprintf(stderr, "aidlc skill list: %v\n", err)
		return contract.ExitUsage
	}
	switch format {
	case "text":
		for _, skill := range result.Skills {
			fmt.Fprintf(stdout, "%s\t%s\n", skill.Name, skill.Description)
		}
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(stderr, "aidlc skill list: %v\n", err)
			return contract.ExitUsage
		}
	default:
		fmt.Fprintf(stderr, "aidlc skill list: unsupported format %q\n", format)
		return contract.ExitUsage
	}
	return contract.ExitOK
}

func runSkillRemoveCLI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("aidlc skill remove", flag.ContinueOnError)
	fs.SetOutput(stderr)
	targetDir := "."
	fs.StringVar(&targetDir, "dir", targetDir, "target repository directory")
	fs.Usage = func() { printSkillRemoveUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		return contract.ExitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return contract.ExitUsage
	}
	result, err := skills.Remove(skills.RemoveOptions{TargetDir: targetDir, Name: fs.Arg(0)})
	if err != nil {
		fmt.Fprintf(stderr, "aidlc skill remove: %v\n", err)
		return contract.ExitUsage
	}
	fmt.Fprintf(stdout, "removed %s\n", result.Name)
	for _, projection := range result.PreservedProjections {
		fmt.Fprintf(stdout, "preserved %s %s diverged\n", projection.IDE, projection.Path)
	}
	return contract.ExitOK
}

func printSkillUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  aidlc skill install [--dir DIR] LOCAL_DIR")
	fmt.Fprintln(w, "  aidlc skill list [--dir DIR] [--format text|json]")
	fmt.Fprintln(w, "  aidlc skill remove [--dir DIR] NAME")
}

func printSkillInstallUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc skill install [--dir DIR] LOCAL_DIR")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --dir DIR   Target repository directory")
}

func printSkillListUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc skill list [--dir DIR] [--format text|json]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --dir DIR             Target repository directory")
	fmt.Fprintln(w, "  --format text|json    Output format")
}

func printSkillRemoveUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: aidlc skill remove [--dir DIR] NAME")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --dir DIR   Target repository directory")
}
