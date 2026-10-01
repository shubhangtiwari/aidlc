# Repomap Blueprint

## Package Purpose

`aidlc/internal/repomap` builds and queries a compact repository navigation index for AI agents.
`aidlc map` writes deterministic JSONL map shards and an index under `docs/map/`; `aidlc query`
returns ranked file paths so agents can target source reads before editing.

Domain: `software`

## Package Boundary

Repomap owns:

- Tier 1 repository scanning with language-agnostic file, import, test-link, doc, spec, ADR, and
  blueprint extraction.
- Stable record schemas, deterministic JSONL helpers, content hashing, cache and query interfaces,
  and query result DTOs.
- Public `SearchPlanV1` execution, symbol/declaration extraction, relationship expansion, rank
  fusion, compact snippets, fallback JSONL scanning, and staleness checks for generated map
  artifacts.
- The derived SQLite FTS5 cache implementation bound by CLI composition-root wiring.
- Optional exact/path search request and response contracts used to augment map results through a
  CLI-wired local adapter.

Repomap does not own agent runtime behavior, hosted indexes, embeddings, vector stores, model
runtimes, network search, AST/LSP/parser integrations, concrete search subprocess adapters, or
template payload delivery files under `.ai/`.

## Layer Map

| Path | Layer | Notes |
| --- | --- | --- |
| `aidlc/internal/repomap/model` | Contracts | Record schemas, `SearchPlanV1`, `IndexMeta`, `CacheBuilder`, `Querier`, `QueryResult`, JSONL helpers, and SHA-256 content hash helper. |
| `aidlc/internal/repomap` | Application | Scanner, symbol extraction, import extraction, test linking, documentation extraction, staleness checks, query engine, rank fusion, compact snippets, and JSONL fallback querier. |
| `aidlc/internal/repomap/cache` | Infrastructure | SQLite FTS5 cache builder, querier, and FTS5 probe. Imports only `model` and SQLite/standard library dependencies. |
| `aidlc/testdata/repomap` | Test Support | Fixture repository and labeled acceptance queries. |

## Cross-package Contracts

- `model.FileRecord`, `ImportRecord`, `TestRecord`, `BlueprintRecord`, `DocRecord`, and
  `ChangeRecord` are deterministic JSONL record contracts. `model.SourceChunkRecord` is the
  deterministic code-content shard contract with path, language, bounded line range, and text
  fields. `model.SymbolRecord` is the deterministic declaration shard contract with path, language,
  kind, name, receiver, container, and bounded line range fields. Shards use LF line endings, stable
  sort keys, and a trailing newline.
- `model.IndexMeta` declares schema version, `docs/map`, `index.json`, `repo-map.sqlite`, and all
  shard filenames, including `source_chunks.jsonl` and `symbols.jsonl`. `IndexMeta.Include`
  records the normalized folder whitelist used to generate the map so `aidlc map --check` can
  report mismatches between saved lock state and generated output.
- `model.SearchPlanV1` is the public structured query contract for `aidlc query --plan-json` and
  `aidlc query --plan-file`. It contains version, question, terms, phrases, symbols, paths, globs,
  languages, shards, include-tests hint, relationship depth, optional `exact_search`, and limit.
  Validation rejects absolute paths, parent traversal, malformed globs, unknown shards, unsupported
  versions, invalid exact-search path hints, and out-of-bounds relationship depth; limits default to
  `10` and clamp at `50`. Recursive `**` globs match zero or more complete path segments, so
  `aidlc/internal/repomap/**/*.go` matches both direct files under `aidlc/internal/repomap/` and
  deeper files such as `aidlc/internal/repomap/cache/query.go`, but not unrelated prefixes.
- `model.ExactSearchPlan`, `ExactSearchRequest`, `ExactSearchResponse`, and `ExactSearcher` are the
  cross-package exact/path search contracts. Exact-search literals are fixed strings, path hints are
  slash-relative, result counts, bytes, and timeouts are clamped, and diagnostics report skipped
  paths, unavailable adapters, timeouts, and truncation without failing normal map retrieval.
- `model.CacheBuilder` builds a derived cache from a map directory without exposing the concrete
  SQLite implementation to application command code.
- `model.Querier` returns ranked `model.QueryResult` values with path, score, and snippet fields.
- `repomap.NewQueryEngine` accepts a `model.Querier`; shard filtering is available when the
  injected querier also supports `QueryShard`, and structured plan execution is available when it
  supports `QueryPlan`.
- `repomap.NewFallbackQuerier` implements JSONL linear-scan query and structured-plan semantics and
  returns a correct path superset when the SQLite cache is unavailable, shard filtering is
  requested, or raw cache retrieval returns no useful rows while fallback has matches. Fallback
  results are ranked by score with deterministic path tie-breaks.
- Cache-backed structured-plan execution is hybrid rather than fallback-only: SQLite FTS handles
  text channels while deterministic JSONL channels handle path hints, recursive globs, symbols,
  source chunks, imports, and test links, and the bounded channel lists are fused deterministically.
- `model.QueryTerms` is the shared lexical normalization contract for SQLite and JSONL fallback
  query paths. It lowercases query text, removes question/connective stop words and punctuation
  noise, preserves code-shaped terms, and returns deterministic unique terms in input order.
- Fused query results, including exact-search augmentation, are de-duplicated by path, sorted by
  descending score and ascending path for ties, and formatted as unchanged tab-separated
  `<path>\t<score>\t<snippet>` rows by default. JSON output may include diagnostics. Snippets are
  one-line, whitespace-normalized, line-aware where available, and bounded for navigation rather
  than full source review.
- `ScanOptions.Include` is the scanner and staleness contract for repo-map folder whitelists.
  Include entries are normalized slash-relative directory paths. Map generation descends only into
  whitelisted folders while still scanning regular files at the repository root.

## Owned State

Target repositories may contain generated repo-specific map state under `docs/map/`:

- `docs/map/files.jsonl`
- `docs/map/imports.jsonl`
- `docs/map/tests.jsonl`
- `docs/map/blueprints.jsonl`
- `docs/map/docs.jsonl`
- `docs/map/changes.jsonl`
- `docs/map/source_chunks.jsonl`
- `docs/map/symbols.jsonl`
- `docs/map/index.json`
- `docs/map/repo-map.sqlite`

The JSONL shards and `index.json` are canonical generated artifacts intended to be committed in the
target repository. `source_chunks.jsonl` is canonical generated map state for bounded code-search
text, and `symbols.jsonl` is canonical generated map state for conservative declaration lookup.
`repo-map.sqlite` is a local derived cache, including source chunk and symbol FTS rows loaded from
JSONL, and is ignored by `docs/map/.gitignore`. The `.ai/` tree remains static delivery and
guidance; it does not persist repo-specific map state. The root `aidlc.lock.json` owns the saved
`workspace.map.include` whitelist; `docs/map/index.json` owns the include list used for the
generated artifacts. A difference between those lists makes the map stale.

## Integration Boundaries

- Filesystem walking reads target repository files and skips `docs/map/` generated artifacts.
  Whitelisted scans prune descent to confirmed include roots and their descendants, plus any
  ancestors needed to reach those roots. Root-level regular files are still indexed even when a
  folder whitelist is present.
- Source chunking and symbol extraction are local, bounded, deterministic, and lexical. They extract
  reviewable navigation evidence only; they do not use vectors, embeddings, hosted indexes, language
  servers, parser dependencies, model runtimes, or remote services.
- Optional local exact/path discovery is permitted only through the CLI-wired
  `aidlc/internal/search` adapter implementing `model.ExactSearcher`. The adapter invokes `rg`
  with `exec.CommandContext`, never through a shell; uses fixed-string arguments, validated
  slash-relative path hints, bounded result and byte limits, and short timeouts; and treats missing
  `rg` as a diagnostic fallback rather than a hard dependency.
- Structured plan query execution stays local and deterministic. With `repo-map.sqlite`, it combines
  SQLite FTS text retrieval with JSONL path, glob, symbol, source chunk, import, and test-link
  channels; without a usable cache, JSONL fallback still provides deterministic results.
- Candidate detection considers only direct child directories of the map root and excludes generated,
  dependency, virtualenv, cache, VCS, and IDE-agent output directories such as `.git`, `.claude`,
  `.codex`, `.cursor`, `.venv`, `node_modules`, `vendor`, `build`, `dist`, and `target`.
- `modernc.org/sqlite` is isolated to `aidlc/internal/repomap/cache` for pure-Go SQLite/FTS5 cache
  creation and querying.
- Concrete SQLite cache construction is allowed in `aidlc/internal/cli` only as composition-root
  dependency assembly into `model.CacheBuilder` and `model.Querier`; application packages must use
  those interfaces and must not import `aidlc/internal/repomap/cache`.
- The FTS5 probe validates local SQLite capabilities at cache-build time.
- No network service, remote index, shell, Make, git invocation, model runtime, vector table,
  embedding provider, hosted search, or required external search runtime is part of repomap
  generation or query execution.

## Test Gates

- `make aidlc-test` covers model determinism, `SearchPlanV1` parsing/defaulting/validation, symbol
  extraction, scanner extraction, include normalization, whitelisted descent with root-file
  indexing, candidate detection exclusions, staleness checks including include mismatch stale
  output, query engine behavior, rank fusion, relationship expansion, compact snippets, JSONL
  fallback, SQLite cache build/query behavior, map/query command parsing, malformed plan errors,
  root CLI routing, and the integration acceptance metrics.
- The integration acceptance gate runs `aidlc map` on the fixture repository, queries through the
  SQLite FTS path, covers representative natural-language/question-shaped code and documentation
  queries, asserts structured-plan mean recall@10 is at least 0.85 across at least 14 labeled
  queries, asserts raw-text mean recall@10 remains at least 0.70 across the same fixture set, and
  asserts compact `--limit 10` result text is at least 30 percent smaller than a source-heavy
  result baseline while preserving expected paths.
- A separate fallback subtest removes the derived SQLite cache and asserts JSONL fallback results
  include every expected path for the same labeled queries.
- Source chunk and symbol coverage proves code files produce bounded deterministic source and
  declaration records and that question-shaped queries can retrieve source paths through both
  public CLI and fallback paths.
- Exact-search coverage proves fixed-string/path search is fused with map results when available,
  unavailable-safe when `rg` is missing, bounded by result/byte/timeout limits, and honest about
  omissions through diagnostics.
- Unknown-location benchmark coverage records deterministic proxy metrics for at least ten
  fixture queries: mean recall@10, precision@10, critical misses, output bytes/words, repeated
  reads, retries, source-heavy baseline bytes, compact reduction, and optional host-reported usage
  fields.
- `make aidlc-release-check` verifies the pure-Go SQLite dependency remains compatible with
  CGO-disabled cross-platform release builds and that no model, vector, parser, network-service,
  hosted-search, or required search-engine runtime dependency is introduced.
