# Repo-Map Query Plans

`aidlc query` accepts either raw search text or a structured `SearchPlanV1` JSON document. Raw text
remains the simple human path. Structured plans are intended for LLM callers that can identify
terms, symbols, paths, and useful shards before asking AIDLC to run deterministic local retrieval.

## CLI

```sh
aidlc query "where is Authorize implemented?"
aidlc query --plan-json '{"version":1,"symbols":["Authorize"],"paths":["internal/auth/auth.go"],"limit":10}'
aidlc query --plan-file .tmp/query-plan.json
```

The default output remains tab-separated rows:

```text
<path>	<score>	<snippet>
```

Supplying raw search terms together with `--plan-json` or `--plan-file` is invalid. `--shard` is
valid for raw text only; structured callers should set `shards` in the JSON plan.

## SearchPlanV1

```json
{
  "version": 1,
  "question": "where does Authorize call greeting normalization?",
  "terms": ["authorize", "greeting", "normalization"],
  "phrases": ["greeting normalization"],
  "symbols": ["Authorize", "NormalizePrincipal", "Greet"],
  "paths": ["internal/auth/auth.go"],
  "globs": ["internal/auth/**/*.go"],
  "languages": ["go"],
  "shards": ["source_chunks.jsonl", "symbols.jsonl", "imports.jsonl"],
  "include_tests": true,
  "relationship_depth": 1,
  "limit": 10
}
```

Fields:

- `version`: required. The current version is `1`.
- `question`: optional natural-language question used as retrieval text and snippet context.
- `terms`: lexical terms chosen by the caller. AIDLC normalizes terms deterministically.
- `phrases`: exact or near-exact phrases to search as higher-signal text.
- `symbols`: declarations or identifiers such as `Authorize`, `NormalizePrincipal`, or
  `QueryEngine.Query`.
- `paths`: slash-relative exact path hints.
- `globs`: slash-relative file globs such as `aidlc/internal/repomap/**/*.go`.
- `languages`: optional language labels from the map.
- `shards`: optional shard filenames. Known shards are `files.jsonl`, `imports.jsonl`,
  `tests.jsonl`, `blueprints.jsonl`, `docs.jsonl`, `changes.jsonl`, `source_chunks.jsonl`, and
  `symbols.jsonl`.
- `include_tests`: optional test inclusion hint.
- `relationship_depth`: optional import/test relationship expansion depth. Default is `1`; maximum
  is `2`.
- `limit`: optional result limit. Default is `10`; maximum is `50`.

Paths and globs must be slash-relative. Absolute paths, parent traversal, Windows drive paths,
backslashes, empty path segments, and unknown shards are rejected with deterministic usage errors.
Malformed JSON and multiple JSON values are also rejected. In globs, `**` matches zero or more
complete path segments: `aidlc/internal/repomap/**/*.go` matches both
`aidlc/internal/repomap/fallback.go` and `aidlc/internal/repomap/cache/query.go`, but not
`aidlc/internal/commands/query.go`.

## Planning Guidance

Prefer a small plan with several independent hints over a broad text dump:

- Use `symbols` for declarations, methods, DTOs, and exported helpers.
- Use `paths` or `globs` when the caller has a credible location hint.
- Use `shards` to keep documentation, source, symbol, import, or test searches focused.
- Keep `terms` concrete and code-shaped; omit filler words that do not narrow retrieval.
- Leave `limit` at `10` for normal navigation.

For example, to find the public query-plan CLI contract:

```json
{
  "version": 1,
  "question": "how does aidlc query accept structured plans?",
  "terms": ["query", "plan-json", "plan-file", "searchplanv1"],
  "symbols": ["SearchPlanV1", "RunQueryCLI"],
  "paths": ["aidlc/internal/commands/query.go"],
  "shards": ["source_chunks.jsonl", "symbols.jsonl"],
  "limit": 10
}
```

## Retrieval Boundary

AIDLC executes plans locally against generated JSONL shards and the derived SQLite FTS cache. The
JSONL shards and `docs/map/index.json` are the canonical committed map state; `repo-map.sqlite` is
a derived ignored cache. When the cache is present, structured plans use hybrid execution: SQLite
FTS handles text channels and deterministic JSONL channels handle paths, recursive globs, symbols,
source chunks, imports, and test links before rank fusion. When the cache is absent or unusable,
the JSONL fallback still provides a deterministic path superset. Retrieval does not use embeddings,
vector tables, model runtimes, network services, parser dependencies, language servers, or search
subprocesses.

Query output is navigation evidence. Agents must read the real source, tests, specs, ADRs, and
blueprints before making code or architecture claims.
