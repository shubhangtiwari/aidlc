package repomaptestdata

type IntegrationQuery struct {
	Text     string
	Expected []string
}

var IntegrationQueries = []IntegrationQuery{
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
