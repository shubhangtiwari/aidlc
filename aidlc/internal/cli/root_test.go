package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
)

func TestRootHelpListsCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--help"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("root help code = %d", code)
	}
	for _, want := range []string{
		"aidlc doctor [flags]",
		"aidlc map [flags]",
		"aidlc query [flags] <search terms>",
		"aidlc upgrade [flags]",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("root help missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRootRoutesMapAndQueryHelp(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "doctor", args: []string{"doctor", "--help"}, want: "Usage: aidlc doctor [flags]"},
		{name: "map", args: []string{"map", "--help"}, want: "Usage: aidlc map [flags]"},
		{name: "query", args: []string{"query", "--help"}, want: "Usage: aidlc query [flags] <search terms>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), tc.args, &stdout, &stderr)
			if code != contract.ExitOK {
				t.Fatalf("code = %d, stderr = %q", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.want) {
				t.Fatalf("stdout missing %q:\n%s", tc.want, stdout.String())
			}
		})
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
	if !strings.Contains(stderr.String(), "aidlc doctor [flags]") {
		t.Fatalf("stderr root help missing doctor:\n%s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRepoMapIntegrationRecallAndFallbackSuperset(t *testing.T) {
	root := copyRepoMapFixture(t)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"map", "--dir", root, "--include", "docs,internal,pkg"}, &stdout, &stderr)
	if code != contract.ExitOK {
		t.Fatalf("aidlc map code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(model.MapDir), model.SQLiteFilename)); err != nil {
		t.Fatalf("expected sqlite cache: %v", err)
	}

	var recallSum float64
	var precisionSum float64
	for _, query := range integrationQueries {
		stdout.Reset()
		stderr.Reset()
		code := Run(context.Background(), []string{"query", "--dir", root, "--limit", "10", query.Text}, &stdout, &stderr)
		if code != contract.ExitOK {
			t.Fatalf("aidlc query %q code = %d, stderr = %q", query.Text, code, stderr.String())
		}
		results := queryResultPaths(stdout.String())
		recall := recallAtK(results, query.Expected)
		precision := precisionAtK(results, query.Expected)
		t.Logf("fts query %q recall@10=%.2f precision@10=%.2f results=%v", query.Text, recall, precision, results)
		recallSum += recall
		precisionSum += precision
	}
	meanRecall := recallSum / float64(len(integrationQueries))
	meanPrecision := precisionSum / float64(len(integrationQueries))
	t.Logf("repo-map FTS mean recall@10=%.2f mean precision@10=%.2f over %d queries", meanRecall, meanPrecision, len(integrationQueries))
	if meanRecall < 0.7 {
		t.Fatalf("mean recall@10 = %.2f, want >= 0.70", meanRecall)
	}

	if err := os.Remove(filepath.Join(root, filepath.FromSlash(model.MapDir), model.SQLiteFilename)); err != nil {
		t.Fatalf("remove sqlite cache: %v", err)
	}
	for _, query := range integrationQueries {
		stdout.Reset()
		stderr.Reset()
		code := Run(context.Background(), []string{"query", "--dir", root, "--limit", "100", query.Text}, &stdout, &stderr)
		if code != contract.ExitOK {
			t.Fatalf("fallback query %q code = %d, stderr = %q", query.Text, code, stderr.String())
		}
		results := queryResultPaths(stdout.String())
		if missing := missingExpected(results, query.Expected); len(missing) > 0 {
			t.Fatalf("fallback query %q missing expected paths %v from results %v", query.Text, missing, results)
		}
	}
}

type labeledQuery struct {
	Text     string
	Expected []string
}

var integrationQueries = []labeledQuery{
	{Text: "Add Auth", Expected: []string{"docs/spec/1000000000-add-auth.md"}},
	{Text: "token validation greeting flow", Expected: []string{"docs/spec/1000000000-add-auth.md"}},
	{Text: "approved specs scanner extraction", Expected: []string{"docs/spec/1000000000-add-auth.md"}},
	{Text: "Use SQLite local query cache", Expected: []string{"docs/adr/1000000001-use-sqlite.md"}},
	{Text: "committed JSONL shards", Expected: []string{"docs/adr/1000000001-use-sqlite.md"}},
	{Text: "core module formats greetings", Expected: []string{"docs/blueprints/core.md"}},
	{Text: "Greet accepts name display string", Expected: []string{"docs/blueprints/core.md"}},
	{Text: "integration boundaries standard library", Expected: []string{"docs/blueprints/core.md"}},
	{Text: "internal auth", Expected: []string{"internal/auth/auth.go", "internal/auth/auth_test.go"}},
	{Text: "where does Authorize NormalizePrincipal Greet?", Expected: []string{"internal/auth/auth.go"}},
	{Text: "internal core", Expected: []string{"internal/core/core.go", "internal/core/core_test.go"}},
	{Text: "how does Greet NormalizeGreetingName?", Expected: []string{"internal/core/core.go"}},
	{Text: "pkg util", Expected: []string{"pkg/util/util.go", "pkg/util/util_test.go"}},
	{Text: "what does StableKey do with parts?", Expected: []string{"pkg/util/util.go"}},
}

func copyRepoMapFixture(t testing.TB) string {
	t.Helper()

	sourceRoot := filepath.Join("..", "..", "testdata", "repomap", "fixture-repo")
	targetRoot := t.TempDir()
	err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(targetRoot, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return targetRoot
}

func queryResultPaths(output string) []string {
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		paths = append(paths, fields[0])
	}
	return paths
}

func recallAtK(results []string, expected []string) float64 {
	if len(expected) == 0 {
		return 1
	}
	found := len(expected) - len(missingExpected(results, expected))
	return float64(found) / float64(len(expected))
}

func precisionAtK(results []string, expected []string) float64 {
	if len(results) == 0 {
		return 0
	}
	expectedSet := map[string]struct{}{}
	for _, path := range expected {
		expectedSet[path] = struct{}{}
	}
	var found int
	for _, path := range results {
		if _, ok := expectedSet[path]; ok {
			found++
		}
	}
	return float64(found) / float64(len(results))
}

func missingExpected(results []string, expected []string) []string {
	resultSet := map[string]struct{}{}
	for _, path := range results {
		resultSet[path] = struct{}{}
	}
	var missing []string
	for _, path := range expected {
		if _, ok := resultSet[path]; !ok {
			missing = append(missing, path)
		}
	}
	return missing
}
