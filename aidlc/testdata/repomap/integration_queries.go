package repomaptestdata

import "github.com/shubhangtiwari/aidlc/aidlc/internal/repomap/model"

type IntegrationQuery struct {
	Text     string
	Plan     model.SearchPlanV1
	Expected []string
}

var IntegrationQueries = []IntegrationQuery{
	{
		Text:     "Add Auth",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Terms: []string{"add", "auth"}, Phrases: []string{"Add Auth"}, Paths: []string{"docs/spec/1000000000-add-auth.md"}, Shards: []string{model.ChangesShard}, Limit: 10},
		Expected: []string{"docs/spec/1000000000-add-auth.md"},
	},
	{
		Text:     "token validation greeting flow",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Terms: []string{"token", "validation", "greeting", "flow"}, Phrases: []string{"token validation", "greeting flow"}, Paths: []string{"docs/spec/1000000000-add-auth.md"}, Shards: []string{model.ChangesShard}, Limit: 10},
		Expected: []string{"docs/spec/1000000000-add-auth.md"},
	},
	{
		Text:     "approved specs scanner extraction",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Terms: []string{"approved", "specs", "scanner", "extraction"}, Paths: []string{"docs/spec/1000000000-add-auth.md"}, Shards: []string{model.ChangesShard}, Limit: 10},
		Expected: []string{"docs/spec/1000000000-add-auth.md"},
	},
	{
		Text:     "Use SQLite local query cache",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Terms: []string{"sqlite", "local", "query", "cache"}, Phrases: []string{"Use SQLite local query cache"}, Paths: []string{"docs/adr/1000000001-use-sqlite.md"}, Shards: []string{model.ChangesShard}, Limit: 10},
		Expected: []string{"docs/adr/1000000001-use-sqlite.md"},
	},
	{
		Text:     "committed JSONL shards",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Terms: []string{"committed", "jsonl", "shards"}, Phrases: []string{"committed JSONL shards"}, Paths: []string{"docs/adr/1000000001-use-sqlite.md"}, Shards: []string{model.ChangesShard}, Limit: 10},
		Expected: []string{"docs/adr/1000000001-use-sqlite.md"},
	},
	{
		Text:     "core module formats greetings",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Terms: []string{"core", "module", "formats", "greetings"}, Paths: []string{"docs/blueprints/core.md"}, Shards: []string{model.BlueprintsShard}, Limit: 10},
		Expected: []string{"docs/blueprints/core.md"},
	},
	{
		Text:     "Greet accepts name display string",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Terms: []string{"greet", "accepts", "name", "display", "string"}, Symbols: []string{"Greet"}, Paths: []string{"docs/blueprints/core.md"}, Shards: []string{model.BlueprintsShard, model.SymbolsShard}, Limit: 10},
		Expected: []string{"docs/blueprints/core.md"},
	},
	{
		Text:     "integration boundaries standard library",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Terms: []string{"integration", "boundaries", "standard", "library"}, Paths: []string{"docs/blueprints/core.md"}, Shards: []string{model.BlueprintsShard}, Limit: 10},
		Expected: []string{"docs/blueprints/core.md"},
	},
	{
		Text:     "internal auth",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Terms: []string{"internal", "auth"}, Paths: []string{"internal/auth/auth.go", "internal/auth/auth_test.go"}, Globs: []string{"internal/auth/*.go"}, Shards: []string{model.FilesShard, model.SourceChunksShard, model.SymbolsShard, model.TestsShard}, Limit: 10},
		Expected: []string{"internal/auth/auth.go", "internal/auth/auth_test.go"},
	},
	{
		Text:     "where does Authorize NormalizePrincipal Greet?",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Question: "where does Authorize NormalizePrincipal Greet?", Terms: []string{"authorize", "normalizeprincipal", "greet"}, Symbols: []string{"Authorize", "NormalizePrincipal", "Greet"}, Paths: []string{"internal/auth/auth.go"}, Shards: []string{model.SourceChunksShard, model.SymbolsShard, model.ImportsShard}, Limit: 10},
		Expected: []string{"internal/auth/auth.go"},
	},
	{
		Text:     "internal core",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Terms: []string{"internal", "core"}, Paths: []string{"internal/core/core.go", "internal/core/core_test.go"}, Globs: []string{"internal/core/*.go"}, Shards: []string{model.FilesShard, model.SourceChunksShard, model.SymbolsShard, model.TestsShard}, Limit: 10},
		Expected: []string{"internal/core/core.go", "internal/core/core_test.go"},
	},
	{
		Text:     "how does Greet NormalizeGreetingName?",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Question: "how does Greet NormalizeGreetingName?", Terms: []string{"greet", "normalizegreetingname"}, Symbols: []string{"Greet", "NormalizeGreetingName"}, Paths: []string{"internal/core/core.go"}, Shards: []string{model.SourceChunksShard, model.SymbolsShard}, Limit: 10},
		Expected: []string{"internal/core/core.go"},
	},
	{
		Text:     "pkg util",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Terms: []string{"pkg", "util"}, Paths: []string{"pkg/util/util.go", "pkg/util/util_test.go"}, Globs: []string{"pkg/util/*.go"}, Shards: []string{model.FilesShard, model.SourceChunksShard, model.SymbolsShard, model.TestsShard}, Limit: 10},
		Expected: []string{"pkg/util/util.go", "pkg/util/util_test.go"},
	},
	{
		Text:     "what does StableKey do with parts?",
		Plan:     model.SearchPlanV1{Version: model.SearchPlanVersion, Question: "what does StableKey do with parts?", Terms: []string{"stablekey", "parts"}, Symbols: []string{"StableKey"}, Paths: []string{"pkg/util/util.go"}, Shards: []string{model.SourceChunksShard, model.SymbolsShard}, Limit: 10},
		Expected: []string{"pkg/util/util.go"},
	},
}
