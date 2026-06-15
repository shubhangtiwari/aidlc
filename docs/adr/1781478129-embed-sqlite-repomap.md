# ADR-1781478129: Embed Pure-Go SQLite for Repo-Map FTS

- **Status:** Accepted
- **Date:** 2026-06-15
- **Deciders:** Shubhang Tiwari

## Context

The agent repo-map feature requires a local query engine that supports ranked full-text search
(FTS5 with bm25) over JSONL-derived records so that AI agents can quickly locate relevant files
before reading source. The engine must compile into the aidlc static binary with CGO_ENABLED=0
across all six release targets (darwin/linux/windows x amd64/arm64) and add no runtime dependencies
to target repositories.

## Decision

Embed **modernc.org/sqlite** (v1.52.0, SQLite 3.53.2) as the FTS5 query/ranking engine. The
derived `docs/map/repo-map.sqlite` database is a local cache rebuilt from committed JSONL shards;
it is gitignored and never committed.

Key properties:

- **Pure-Go / CGO-free.** modernc.org/sqlite is a machine-translated C-to-Go port. It compiles
  with `CGO_ENABLED=0` and cross-compiles to all six release targets without a C toolchain.
- **FTS5 + bm25() enabled by default.** No build tags or compile-time flags required.
- **Binary size trade-off.** Adds approximately 4--7 MB to the static binary. Acceptable for a
  developer CLI that already ships platform-specific archives.
- **JSONL canonical, SQLite derived.** Committed JSONL shards are the source of truth. The SQLite
  database is a derived, gitignored cache that any developer can regenerate locally. This avoids
  binary merge conflicts and keeps diffs reviewable.
- **FTS5 capability probe.** At cache-build time, the builder probes for FTS5 support. If the probe
  fails (defensive; not expected with modernc builds), query falls back to linear JSONL scan. The
  probe is a `CREATE VIRTUAL TABLE ... USING fts5(...)` attempt inside a transaction that rolls back
  on failure.

## Consequences

- `go.mod` gains `modernc.org/sqlite` and its transitive dependencies (pure Go; no C headers).
  `go.sum` grows accordingly. The `libc` version pin is auto-managed by Go MVS.
- `CGO_ENABLED=0` cross-compile must remain green. `scripts/build-release-assets.sh` and
  `make aidlc-release-check` are unchanged because the dependency is pure Go.
- The aidlc binary grows by 4--7 MB. All six release archives grow proportionally.
- Only the `internal/repomap/cache` package may import `modernc.org/sqlite`. Application packages
  depend on `CacheBuilder`/`Querier` interfaces defined in the contracts layer, not on the concrete
  SQLite implementation. The interface-layer `internal/cli` package may import
  `internal/repomap/cache` only as the composition root that constructs concrete cache
  dependencies and passes them behind those interfaces; it must not contain repo-map business
  logic, persistence logic, scanning/query behavior, or direct SQLite operations.
- Target repositories gain a gitignored `docs/map/repo-map.sqlite` file and a self-contained
  `docs/map/.gitignore` managing that derived cache.

## Alternatives Considered

| Alternative | Why rejected |
| --- | --- |
| **bleve (blevesearch/bleve)** | Full search engine with heavier binary footprint (~10+ MB), complex index format, and overkill for a per-repo navigation index. SQLite FTS5 is simpler and sufficient. |
| **In-memory linear scan only** | Adequate for small repos but degrades on large monorepos. FTS5 bm25 ranking is essential for returning the most relevant K files. Linear scan is retained as the fallback path only. |
| **mattn/go-sqlite3 (CGO)** | Requires a C compiler on every build host and CI runner. Breaks the CGO_ENABLED=0 cross-compile contract that all six release targets depend on. |
| **Target-runtime Python (sqlite3 stdlib)** | Requires Python installed in target repos. Violates the "no runtime dependency" principle of the CLI distribution model. |
| **gotreesitter for symbol-level extraction** | Pre-1.0, single maintainer. Deferred to a future Tier 2 that is explicitly out of scope for this ADR and the governing spec. |
