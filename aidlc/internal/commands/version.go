package commands

import (
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

func RunVersionCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		if isHelpArg(args) {
			fmt.Fprintln(stdout, "Usage: aidlc version")
			return contract.ExitOK
		}
		fmt.Fprintf(stderr, "aidlc version: unexpected argument %q\n", args[0])
		return contract.ExitUsage
	}
	fmt.Fprintf(stdout, "aidlc %s\n", CurrentVersion())
	return contract.ExitOK
}

func CurrentVersion() string {
	if Version != "" && Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		version := strings.TrimSpace(info.Main.Version)
		if version != "" && version != "(devel)" {
			return version
		}
	}
	if Version == "" {
		return "dev"
	}
	return Version
}
