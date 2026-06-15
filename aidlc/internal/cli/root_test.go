package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
	"github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"
	repomaptestdata "github.com/shubhangtiwari/aidlc/aidlc/testdata/repomap"
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

	rawMetrics := queryMetricTotals{}
	structuredMetrics := queryMetricTotals{}
	var compactOutputBytes int
	var sourceHeavyOutputBytes int
	sourceHeavyText := sourceHeavyTextByPath(t, filepath.Join(root, filepath.FromSlash(model.MapDir)))
	for _, query := range repomaptestdata.IntegrationQueries {
		stdout.Reset()
		stderr.Reset()
		code := Run(context.Background(), []string{"query", "--dir", root, "--limit", "10", query.Text}, &stdout, &stderr)
		if code != contract.ExitOK {
			t.Fatalf("aidlc query %q code = %d, stderr = %q", query.Text, code, stderr.String())
		}
		results := queryResultPaths(stdout.String())
		recall := recallAtK(results, query.Expected)
		precision := precisionAtK(results, query.Expected)
		t.Logf("raw query %q recall@10=%.2f precision@10=%.2f results=%v", query.Text, recall, precision, results)
		rawMetrics.add(recall, precision)
		if missing := missingExpected(results, query.Expected); len(missing) > 0 {
			t.Fatalf("raw query %q missing expected paths %v from results %v", query.Text, missing, results)
		}

		compactOutputBytes += len(stdout.String())
		sourceHeavyOutputBytes += sourceHeavyResultBytes(results, sourceHeavyText)

		plan, err := json.Marshal(query.Plan)
		if err != nil {
			t.Fatalf("marshal plan for %q: %v", query.Text, err)
		}
		stdout.Reset()
		stderr.Reset()
		code = Run(context.Background(), []string{"query", "--dir", root, "--plan-json", string(plan)}, &stdout, &stderr)
		if code != contract.ExitOK {
			t.Fatalf("aidlc query plan %q code = %d, stderr = %q", query.Text, code, stderr.String())
		}
		planResults := queryResultPaths(stdout.String())
		planRecall := recallAtK(planResults, query.Expected)
		planPrecision := precisionAtK(planResults, query.Expected)
		t.Logf("plan query %q recall@10=%.2f precision@10=%.2f results=%v", query.Text, planRecall, planPrecision, planResults)
		structuredMetrics.add(planRecall, planPrecision)
	}
	if len(repomaptestdata.IntegrationQueries) < 14 {
		t.Fatalf("fixture query count = %d, want at least 14", len(repomaptestdata.IntegrationQueries))
	}
	t.Logf("repo-map raw mean recall@10=%.2f mean precision@10=%.2f over %d queries", rawMetrics.meanRecall(), rawMetrics.meanPrecision(), rawMetrics.count)
	if rawMetrics.meanRecall() < 0.70 {
		t.Fatalf("raw mean recall@10 = %.2f, want >= 0.70", rawMetrics.meanRecall())
	}
	t.Logf("repo-map structured plan mean recall@10=%.2f mean precision@10=%.2f over %d queries", structuredMetrics.meanRecall(), structuredMetrics.meanPrecision(), structuredMetrics.count)
	if structuredMetrics.meanRecall() < 0.85 {
		t.Fatalf("structured plan mean recall@10 = %.2f, want >= 0.85", structuredMetrics.meanRecall())
	}
	t.Logf("repo-map compact output bytes=%d source-heavy baseline bytes=%d reduction=%.1f%%",
		compactOutputBytes,
		sourceHeavyOutputBytes,
		100*(1-float64(compactOutputBytes)/float64(sourceHeavyOutputBytes)),
	)
	if compactOutputBytes == 0 || sourceHeavyOutputBytes == 0 {
		t.Fatalf("compact bytes = %d, source-heavy baseline bytes = %d", compactOutputBytes, sourceHeavyOutputBytes)
	}
	if float64(compactOutputBytes) > float64(sourceHeavyOutputBytes)*0.70 {
		t.Fatalf("compact output bytes = %d, want at most 70%% of source-heavy baseline %d", compactOutputBytes, sourceHeavyOutputBytes)
	}

	if err := os.Remove(filepath.Join(root, filepath.FromSlash(model.MapDir), model.SQLiteFilename)); err != nil {
		t.Fatalf("remove sqlite cache: %v", err)
	}
	for _, query := range repomaptestdata.IntegrationQueries {
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

type queryMetricTotals struct {
	recallSum    float64
	precisionSum float64
	count        int
}

func (m *queryMetricTotals) add(recall, precision float64) {
	m.recallSum += recall
	m.precisionSum += precision
	m.count++
}

func (m queryMetricTotals) meanRecall() float64 {
	if m.count == 0 {
		return 0
	}
	return m.recallSum / float64(m.count)
}

func (m queryMetricTotals) meanPrecision() float64 {
	if m.count == 0 {
		return 0
	}
	return m.precisionSum / float64(m.count)
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

func sourceHeavyTextByPath(t testing.TB, mapDir string) map[string]string {
	t.Helper()

	texts := map[string]string{}
	for _, shard := range []string{
		model.SourceChunksShard,
		model.BlueprintsShard,
		model.DocsShard,
		model.ChangesShard,
	} {
		file, err := os.Open(filepath.Join(mapDir, shard))
		if err != nil {
			t.Fatalf("open %s: %v", shard, err)
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var record struct {
				Path string `json:"path"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
				_ = file.Close()
				t.Fatalf("decode %s: %v", shard, err)
			}
			if record.Path != "" && record.Text != "" {
				texts[record.Path] += " " + record.Text
			}
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			t.Fatalf("scan %s: %v", shard, err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close %s: %v", shard, err)
		}
	}
	return texts
}

func sourceHeavyResultBytes(paths []string, textByPath map[string]string) int {
	var b strings.Builder
	for _, path := range paths {
		text := strings.TrimSpace(textByPath[path])
		if text == "" {
			text = path
		}
		b.WriteString(path)
		b.WriteString("\t1.000000\t")
		b.WriteString(text)
		b.WriteString("\n")
	}
	return b.Len()
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
