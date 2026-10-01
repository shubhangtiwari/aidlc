package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/testutil"
)

func TestSkillCLIInstallListAndRemove(t *testing.T) {
	target := t.TempDir()
	source := createSkillCLISource(t, "local-tool")

	var stdout, stderr bytes.Buffer
	code := RunSkillCLI(context.Background(), []string{"install", "--dir", target, source}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("install code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "installed local-tool\n") {
		t.Fatalf("install output = %q", stdout.String())
	}
	assertExists(t, target, ".ai/skills/installed/local-tool/SKILL.md")

	stdout.Reset()
	stderr.Reset()
	code = RunSkillCLI(context.Background(), []string{"list", "--dir", target}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("list code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "local-tool\tCLI local tool skill\n") {
		t.Fatalf("list output = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = RunSkillCLI(context.Background(), []string{"list", "--dir", target, "--format", "json"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("json list code = %d, stderr = %q", code, stderr.String())
	}
	var listed contract.SkillListResult
	if err := json.Unmarshal(stdout.Bytes(), &listed); err != nil {
		t.Fatalf("parse json list: %v\n%s", err, stdout.String())
	}
	if len(listed.Skills) != 1 || listed.Skills[0].Name != "local-tool" {
		t.Fatalf("json list = %#v", listed)
	}

	stdout.Reset()
	stderr.Reset()
	code = RunSkillCLI(context.Background(), []string{"remove", "--dir", target, "local-tool"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("remove code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "removed local-tool\n") {
		t.Fatalf("remove output = %q", stdout.String())
	}
	assertMissing(t, target, ".ai/skills/installed/local-tool/SKILL.md")
}

func TestSkillCLIUsageErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := RunSkillCLI(context.Background(), nil, &stdout, &stderr); code != contract.ExitUsage {
		t.Fatalf("empty code = %d", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := RunSkillCLI(context.Background(), []string{"list", "--format", "xml"}, &stdout, &stderr); code != contract.ExitUsage {
		t.Fatalf("bad format code = %d", code)
	}
	if !strings.Contains(stderr.String(), "unsupported format") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func createSkillCLISource(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	testutil.WriteFile(t, root, "SKILL.md", "---\nname: "+name+"\ndescription: CLI local tool skill\n---\n\nUse it.\n")
	return root
}
