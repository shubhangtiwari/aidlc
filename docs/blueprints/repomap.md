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
- Query orchestration, fallback JSONL scanning, and staleness checks for generated map artifacts.
- The derived SQLite FTS5 cache implementation bound by CLI composition-root wiring.

Repomap does not own agent runtime behavior, hosted indexes, symbol-level AST extraction, or
template payload delivery files under `.ai/`.

## Layer Map

| Path | Layer | Notes |
| --- | --- | --- |
| `aidlc/internal/repomap/model` | Contracts | Record schemas, `IndexMeta`, `CacheBuilder`, `Querier`, `QueryResult`, JSONL helpers, and SHA-256 content hash helper. |
| `aidlc/internal/repomap` | Application | Scanner, import extraction, test linking, documentation extraction, staleness checks, query engine, and JSONL fallback querier. |
| `aidlc/internal/repomap/cache` | Infrastructure | SQLite FTS5 cache builder, querier, and FTS5 probe. Imports only `model` and SQLite/standard library dependencies. |
| `aidlc/testdata/repomap` | Test Support | Fixture repository and labeled acceptance queries. |

## Cross-package Contracts

- `model.FileRecord`, `ImportRecord`, `TestRecord`, `BlueprintRecord`, `DocRecord`, and
  `ChangeRecord` are deterministic JSONL record contracts. Shards use LF line endings, stable sort
  keys, and a trailing newline.
- `model.IndexMeta` declares schema version, `docs/map`, `index.json`, `repo-map.sqlite`, and all
  shard filenames. `IndexMeta.Include` records the normalized folder whitelist used to generate the
  map so `aidlc map --check` can report mismatches between saved lock state and generated output.
- `model.CacheBuilder` builds a derived cache from a map directory without exposing the concrete
  SQLite implementation to application command code.
- `model.Querier` returns ranked `model.QueryResult` values with path, score, and snippet fields.
- `repomap.NewQueryEngine` accepts a `model.Querier`; shard filtering is available when the
  injected querier also supports `QueryShard`.
- `repomap.NewFallbackQuerier` implements JSONL linear-scan query semantics and returns a correct
  path superset when the SQLite cache is unavailable or shard filtering is requested.
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
- `docs/map/index.json`
- `docs/map/repo-map.sqlite`

The JSONL shards and `index.json` are canonical generated artifacts intended to be committed in the
target repository. `repo-map.sqlite` is a local derived cache and is ignored by `docs/map/.gitignore`.
The `.ai/` tree remains static delivery and guidance; it does not persist repo-specific map state.
The root `aidlc.lock.json` owns the saved `workspace.map.include` whitelist; `docs/map/index.json`
owns the include list used for the generated artifacts. A difference between those lists makes the
map stale.

## Integration Boundaries

- Filesystem walking reads target repository files and skips `docs/map/` generated artifacts.
  Whitelisted scans prune descent to confirmed include roots and their descendants, plus any
  ancestors needed to reach those roots. Root-level regular files are still indexed even when a
  folder whitelist is present.
- Candidate detection considers only direct child directories of the map root and excludes generated,
  dependency, virtualenv, cache, VCS, and IDE-agent output directories such as `.git`, `.claude`,
  `.codex`, `.cursor`, `.venv`, `node_modules`, `vendor`, `build`, `dist`, and `target`.
- `modernc.org/sqlite` is isolated to `aidlc/internal/repomap/cache` for pure-Go SQLite/FTS5 cache
  creation and querying.
- Concrete SQLite cache construction is allowed in `aidlc/internal/cli` only as composition-root
  dependency assembly into `model.CacheBuilder` and `model.Querier`; application packages must use
  those interfaces and must not import `aidlc/internal/repomap/cache`.
- The FTS5 probe validates local SQLite capabilities at cache-build time.
- No network service, remote index, shell, Make, or git invocation is part of repomap generation or
  query execution.

## Test Gates

- `make aidlc-test` covers model determinism, scanner extraction, include normalization, whitelisted
  descent with root-file indexing, candidate detection exclusions, staleness checks including include
  mismatch stale output, query engine behavior, JSONL fallback, SQLite cache build/query behavior,
  map/query command parsing, root CLI routing, and the integration acceptance metric.
- The integration acceptance gate runs `aidlc map` on the fixture repository, queries through the
  SQLite FTS path, and asserts mean recall@10 is at least 0.7 across at least 10 labeled queries.
- A separate fallback subtest removes the derived SQLite cache and asserts JSONL fallback results
  include every expected path for the same labeled queries.
- `make aidlc-release-check` verifies the pure-Go SQLite dependency remains compatible with
  CGO-disabled cross-platform release builds.
