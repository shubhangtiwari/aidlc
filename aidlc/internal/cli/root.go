package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/commands"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printRootHelp(stderr)
		return contract.ExitUsage
	}

	switch args[0] {
	case "init":
		return commands.RunInitCLI(ctx, args[1:], stdout, stderr)
	case "update":
		return commands.RunUpdateCLI(ctx, args[1:], stdout, stderr)
	case "upgrade":
		return commands.RunUpgradeCLI(ctx, args[1:], stdout, stderr)
	case "version":
		return commands.RunVersionCLI(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printRootHelp(stdout)
		return contract.ExitOK
	default:
		fmt.Fprintf(stderr, "aidlc: unknown command %q\n\n", args[0])
		printRootHelp(stderr)
		return contract.ExitUsage
	}
}

func printRootHelp(w io.Writer) {
	fmt.Fprintln(w, "aidlc initializes and updates AIDLC governance files.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  aidlc init <claude|codex|cursor|copilot|windsurf|all> [flags]")
	fmt.Fprintln(w, "  aidlc update [flags]")
	fmt.Fprintln(w, "  aidlc upgrade [flags]")
	fmt.Fprintln(w, "  aidlc version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Common flags:")
	fmt.Fprintln(w, "  --source github|local   Template source kind (default github)")
	fmt.Fprintln(w, "  --url URL               GitHub repository URL for github source")
	fmt.Fprintln(w, "  --ref REF               GitHub ref or local source label")
	fmt.Fprintln(w, "  --path PATH             Local source path when --source local")
	fmt.Fprintln(w, "  --dry-run               Print planned changes without writing")
}
