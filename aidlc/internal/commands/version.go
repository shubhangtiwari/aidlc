package commands

import (
	"fmt"
	"io"

	"github.com/aidlc/ai-dlc-template/aidlc/internal/contract"
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
	fmt.Fprintf(stdout, "aidlc %s\n", Version)
	return contract.ExitOK
}
