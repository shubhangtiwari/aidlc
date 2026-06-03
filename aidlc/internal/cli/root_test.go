package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

func TestRootHelpListsUpgrade(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--help"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("root help code = %d", code)
	}
	if !strings.Contains(stdout.String(), "aidlc upgrade [flags]") {
		t.Fatalf("root help missing upgrade:\n%s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRootRoutesUpgradeHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"upgrade", "--help"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("upgrade help code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage: aidlc upgrade [flags]") {
		t.Fatalf("upgrade help missing:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "--repo owner/repo") {
		t.Fatalf("upgrade help missing repo flag:\n%s", stdout.String())
	}
}

func TestRootUnknownCommandStillExitsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"missing"}, &stdout, &stderr)
	if code != contract.ExitUsage {
		t.Fatalf("unknown command code = %d, want usage", code)
	}
	if !strings.Contains(stderr.String(), "aidlc: unknown command \"missing\"") {
		t.Fatalf("stderr missing unknown command:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "aidlc upgrade [flags]") {
		t.Fatalf("stderr root help missing upgrade:\n%s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
