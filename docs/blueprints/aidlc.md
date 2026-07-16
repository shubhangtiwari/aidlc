# aidlc Blueprint

## Package Purpose

`aidlc/` is an isolated Go module that produces the installable AIDLC CLI. It initializes and
updates AIDLC governance payloads in target repositories, generates IDE-native files, and provides
release/install surfaces without requiring Unix-only tooling for normal CLI flows.

Domain: `software`

## Package Boundary

`aidlc/` owns:

- CLI command contracts and option DTOs.
- Supported IDE identifiers.
- Target repository lock schema for root `aidlc.lock.json`; `.aidlc/manifest.json` is a legacy read
  fallback.
- Public template manifest schema for `.ai/template-manifest.yaml`.
- Payload path policy.
- Native IDE generation, manifest-aware payload sync, installers, release checks, and native CLI
  binary upgrades.

It does not own root template source files except by reading the public template manifest as input.

## Layer Map

| Path | Layer | Notes |
| --- | --- | --- |
| `aidlc/cmd/aidlc` | Interface | Executable entrypoint. |
| `aidlc/internal/cli` | Interface | Root command wiring and process-facing concerns. |
| `aidlc/internal/commands` | Application | Init, update, upgrade, and version orchestration. |
| `aidlc/internal/generator` | Application | IDE file generation. |
| `aidlc/internal/repomap` | Application | Repo-map scanning, query orchestration, staleness checks, and JSONL fallback query. |
| `aidlc/internal/repomap/model` | Contracts | Repo-map record schemas, index metadata, cache/query interfaces, JSONL helpers, and content hashing. |
| `aidlc/internal/repomap/cache` | Infrastructure | SQLite FTS5 cache builder, querier, and FTS5 capability probe. |
| `aidlc/internal/sync` | Application | Manifest-aware planning and copy decisions. |
| `aidlc/internal/source` | Infrastructure | GitHub archive and local-source access. |
| `aidlc/internal/install` | Infrastructure | Installer, release download, checksum validation, archive extraction, and binary replacement. |
| `aidlc/internal/contract` | Contracts | Shared command, IDE, manifest, and option types. |
| `aidlc/internal/payload` | Contracts | Public payload path normalization and exclusion policy. |
| `aidlc/internal/testutil` | Test Support | Filesystem helpers for tests. |

## Cross-package Contracts

- Commands: `doctor`, `init`, `map`, `query`, `update`, `upgrade`, and `version`.
- Native persona rendering consumes `.ai/models.defaults.toml` by IDE and persona. Codex renders
  `model` directly and translates source `reasoning` to `model_reasoning_effort`; Claude Code
  renders `model` and source `effort` as agent-frontmatter `effort`; Cursor renders `model` only
  and has no effort field. Empty or absent source values omit their corresponding generated field.
- `aidlc map [--dir DIR] [--include DIR[,DIR...]] [--check]` builds the repository navigation
  index for `DIR` (default `.`). A normal run scans the target repository using the saved
  `workspace.map.include` whitelist from `aidlc.lock.json`, writes deterministic JSONL shards and
  `docs/map/index.json`, rebuilds the derived `docs/map/repo-map.sqlite` cache, prints a stable
  plain-text summary, and exits `0`. `--include` normalizes, saves, and uses an explicit
  comma-separated folder whitelist. On the first interactive run without a saved whitelist, the
  command detects candidate root folders, asks the user to confirm them, saves the confirmed list,
  and then builds the map. On later runs it reuses the saved whitelist without prompting. A
  non-interactive first run without `--include` exits `2` with deterministic guidance.
- `aidlc map --check` is read-only. It rejects `--include`, requires a saved
  `workspace.map.include` whitelist, does not prompt, does not write `aidlc.lock.json`, and compares
  the current files under the saved whitelist to `docs/map/index.json`. It prints
  `repo map: fresh` or a deterministic stale report, exits `0` when fresh, exits `1` when stale,
  and exits `2` for invalid usage or unreadable map state. A mismatch between the saved whitelist
  and the include list recorded in `docs/map/index.json` is stale output.
- `aidlc query [--dir DIR] [--limit N] [--shard NAME] <search terms>` queries the repo map for
  `DIR` (default `.`) and prints ranked tab-separated rows as `<path>\t<score>\t<snippet>`.
  `--limit` defaults to `10`; negative limits and empty search terms exit `2`. Raw text compiles to
  the public `SearchPlanV1` contract internally while preserving simple CLI behavior. Without
  `--shard`, query uses `docs/map/repo-map.sqlite` when present and falls back to JSONL linear scan
  when the cache is absent. Query text is normalized for lexical matching so question-shaped
  searches drop connector noise while preserving code-shaped terms; the public output format and
  exit behavior do not change. `--shard` forces JSONL fallback for the selected shard. Successful
  empty result sets exit `0` with no rows.
- `aidlc query --plan-json JSON` and `aidlc query --plan-file PATH` execute public
  `SearchPlanV1` input with version, question, terms, phrases, symbols, paths, globs, languages,
  shards, include-tests hint, relationship depth, and limit. Plan parsing rejects malformed JSON,
  multiple JSON values, unsupported versions, absolute paths, parent traversal, malformed globs,
  unknown shards, invalid relationship depth, and invalid raw-text/plan flag combinations with
  deterministic exit `2` errors. Recursive `**` globs match zero or more complete path segments.
  Structured plan output uses the same tab-separated result rows as raw text. When the SQLite cache
  is present, plan execution is hybrid: SQLite FTS handles text channels while deterministic JSONL
  channels handle paths, recursive globs, symbols, source chunks, imports, and test links. It is not
  fallback-only unless the cache is absent or unusable.
- Map/query dependency boundary: command orchestration accepts `repomap/model` interfaces for cache
  building and querying. The CLI root wires the concrete SQLite implementation into those
  interfaces as a narrow composition-root exception; application command code must not import
  `aidlc/internal/repomap/cache` directly. CLI root wiring may construct and pass concrete cache
  dependencies only. It must not contain repo-map business logic, persistence logic,
  scanning/query behavior, or direct infrastructure operations beyond dependency assembly.
- `aidlc init <claude|codex|cursor|copilot|windsurf|all> [--source github|local] [--url URL]
  [--ref REF] [--path PATH] [--dry-run] [--force]` copies the public template payload and then
  generates the requested IDE files, recording concrete workspace IDEs in `aidlc.lock.json`. Init
  conflicts exit `1` but do not block safe create/skip payload decisions, requested IDE generation,
  or root lock writing. Divergent local files remain untouched unless `--force` is set.
- `aidlc update [--source github|local] [--url URL] [--ref REF] [--path PATH] [--dry-run]
  [--force]` reads `aidlc.lock.json`, or legacy `.aidlc/manifest.json` when the root lock is absent,
  fetches the configured or overridden source, applies clean manifest-aware updates, and regenerates
  the persisted workspace IDE surfaces.
- `aidlc upgrade [--repo owner/repo] [--version latest|TAG] [--install-dir DIR] [--dry-run]`
  upgrades the installed CLI binary from GitHub release assets. The default repository is
  `shubhangtiwari/aidlc`, the default version selector is `latest`, and the default destination is
  the directory of the running executable. Explicit selectors accept `vX.Y.Z` and `aidlc/vX.Y.Z`;
  release tags normalize to the `aidlc version` format `vX.Y.Z` in command output. Latest-release
  requests that resolve to the current version exit `0` without writing, while explicit version
  requests reinstall the selected release. Dry-run exits `0` after resolving release metadata,
  selected asset, and destination without downloading archives, extracting binaries, or writing.
  Usage, unsupported platform, release lookup, download, checksum, extraction, and install errors
  exit `2` with deterministic `aidlc upgrade:` stderr. Human output is deterministic plain text
  with current version, target version, release tag, selected asset, destination, and status
  `installed`, `skipped`, or `dry-run`.
- `aidlc version` prints the CLI version string.
- Supported IDEs: `claude`, `codex`, `cursor`, `copilot`, `windsurf`, and aggregate `all`.
- Target lock: root `aidlc.lock.json` records schema version, upstream source/ref/commit,
  authoritative `workspace.ides`, generated IDE metadata, tracked payload paths, file checksums,
  file modes, repo-map whitelist state under `workspace.map.include`, and command metadata such as
  source kind and local source path. `workspace.ides` stores concrete IDE identifiers only,
  de-duplicated in canonical supported IDE order; `all` expands to every concrete IDE before
  persistence. `workspace.map.include` stores normalized slash-relative folder paths used by
  `aidlc map` and `aidlc map --check`; explicit include writes preserve existing upstream,
  generated, files, workspace IDE, metadata, and unknown lock fields. Tracked payload paths are
  destination paths: the AIDLC repository root `LICENSE` payload is tracked in target repositories
  as `licenses/aidlc.md`, leaving any consumer root `LICENSE` outside AIDLC ownership.
- Legacy fallback: when `aidlc.lock.json` is absent, `.aidlc/manifest.json` may supply
  `workspace.ides`; if that field is absent, `generated.ide` is used, with `generated.ide: all`
  expanded to every concrete IDE. Root `aidlc.lock.json` takes precedence when both files exist.
- Public template manifest: `.ai/template-manifest.yaml` allowlists public paths and documents
  blocked private paths. Manifest entries may be same-path includes or explicit single-file
  source-to-target includes; the AIDLC license reads from source `LICENSE` and installs to target
  `licenses/aidlc.md`.
- Payload policy: only normalized relative paths are valid; absolute paths, parent traversal,
  empty paths, and Windows drive paths are rejected.
- Native IDE generation emits generated governance guidance from the portable `.ai/` contract.
  Cursor rule rendering is part of that generated guidance contract: spec-gate text must follow the
  portable scope-resolution rule for nested initialized AIDLC roots, including scope-local
  `docs/spec/<epoch>-<slug>.md` ownership and parent-scope refusal for files owned by nested scopes.
- `aidlc doctor [--dir DIR]` reports deterministic local diagnostics without mutating user or
  repository state. It prints the current version, running executable path, selected repository
  directory, whether `aidlc` is discoverable through the current process `PATH`, executable status
  for common install candidates including the Windows installer default
  `%LOCALAPPDATA%\Programs\aidlc\bin`, and `.ai/Makefile.inc` plus root Makefile include status. It
  exits `0` when no action is needed, `1` when PATH or Make helper findings are present, and `2`
  for invalid usage. Findings include sanitized-shell and CI guidance, including `AIDLC_BIN` and
  installer PATH next steps.
- Repository `make init <ide>` and `make update` targets are thin wrappers around the native CLI.
  Public `.ai/Makefile.inc` repo-map helpers resolve `aidlc` through explicit `AIDLC_BIN`, then
  `command -v aidlc`, then supported common install locations including
  `$LOCALAPPDATA/Programs/aidlc/bin/aidlc.exe`, `$HOME/.local/bin`, `$HOME/bin`,
  `/opt/homebrew/bin`, and `/usr/local/bin` on Unix-like systems, with Windows executable variants
  where supported by the shell. `make ai-map`, `make ai-map-check`, `make ai-query`, and
  `make ai-doctor` share that resolver. Resolver failure exits `2` before invoking a helper command
  and prints guidance for `AIDLC_BIN`, `make ai-doctor`, Unix `AIDLC_INSTALL_DIR`, and Windows user
  PATH behavior. These targets do not define a separate Bash compatibility contract.
- Exit behavior: successful no-op exits `0`, conflicts exit `1`, and invalid usage exits `2`.
  Successful forced init/update overwrites exit `0`; `--force` does not downgrade usage, source,
  fetch, manifest, generation, lock, write, or upgrade errors.
- Human CLI result output is deterministic plain text with sections headed exactly `◆ plan`,
  `✓ written`, and `✦ generated`. Plan rows use `<decision-state> <path> <reason>`, written rows
  use `write <path> <comment>`, and generated rows use `generate <path> <comment>`. Forced
  conflict bypasses print as `overwrite <path> <reason>` rows instead of `conflict` rows.

## Owned State

Source repository state owned by `aidlc/` is limited to the isolated Go module, tests, testdata,
release configuration, installers, and release verification scripts. Target repositories may
receive `aidlc.lock.json`, public template payload files including `licenses/aidlc.md`, and
generated IDE files such as `AGENTS.md`, `CLAUDE.md`, `.codex/**`, `.cursor/**`, `.claude/**`,
`.github/copilot-instructions.md`, and `.windsurfrules`. Target repositories may also receive
repo-specific generated map artifacts from `aidlc map` under `docs/map/`: committed canonical JSONL
shards including `docs/map/source_chunks.jsonl`, `docs/map/symbols.jsonl`, and `docs/map/index.json`,
plus the ignored derived cache `docs/map/repo-map.sqlite`. The saved map whitelist in
`aidlc.lock.json` is authoritative for subsequent map builds and read-only freshness checks. The
consumer repository root `LICENSE` is not
owned by AIDLC after license relocation; init and update must not create, overwrite, delete, or
track it as the active AIDLC license payload. The root lock owns workspace IDE selections, map
include selections, tracked payload checksums, generation metadata, and update source metadata.
Legacy `.aidlc/manifest.json` may still be read for compatibility but is no longer the authoritative
write target. During conflicted init, the partial root lock records only accepted upstream payload
files plus generation/workspace metadata; conflicted payload paths are excluded from tracked clean
file entries. Forced init and forced update may replace divergent public payload destination files
and then record those overwritten paths as clean tracked files in `aidlc.lock.json`.
Removed-upstream files, private paths, unknown local files, and local-only files remain outside
forced deletion or overwrite behavior.
`aidlc upgrade` owns only the installed `aidlc` executable at the resolved install destination and
temporary staging files in that destination directory during replacement. It does not modify target
repository payload state, `aidlc.lock.json`, generated IDE files, or legacy `.aidlc/manifest.json`.
`aidlc doctor` owns no repository, installer, PATH, shell configuration, network, or payload state.
The Unix shell installer owns only the installed `aidlc` executable at the selected install
directory. `AIDLC_INSTALL_DIR` remains the highest-priority override; otherwise the installer
chooses a writable `/usr/local/bin` destination when available and falls back to
`$HOME/.local/bin`. It checks PATH membership and prints guidance, but it does not edit shell
dotfiles or invoke `sudo`. The Windows PowerShell installer owns the installed `aidlc.exe` under the
selected install directory, defaulting to the user-local app bin directory
`$LOCALAPPDATA\Programs\aidlc\bin`, and may update only the current user's PATH. It does not update
machine PATH or target repository payload state.

## Integration Boundaries

- GitHub archive access is used to fetch template payload snapshots for normal init/update flows.
- Local and GitHub/archive source providers may read payload bytes and modes from a source path that
  differs from the emitted target path for explicit manifest mappings. Normal init/update flows
  still use native filesystem, archive, checksum, and rendering logic and avoid shelling out.
- GitHub release artifacts and checksum files are consumed by shell and PowerShell installers.
- The Unix shell installer downloads release artifacts, verifies them against `checksums.txt`, and
  writes the `aidlc` executable to `AIDLC_INSTALL_DIR` when set, otherwise to writable
  `/usr/local/bin` or the user-local fallback `$HOME/.local/bin`. It does not elevate privileges
  itself and does not edit shell dotfiles. After installation it checks whether the selected
  directory is on PATH and prints deterministic verification or next-step guidance, including
  `AIDLC_INSTALL_DIR`, shell PATH configuration, and `AIDLC_BIN=<installed path> make <target>`.
- The Windows PowerShell installer downloads release artifacts, verifies them against
  `checksums.txt`, and writes `aidlc.exe` to `AIDLC_INSTALL_DIR` when set, otherwise to
  `$LOCALAPPDATA\Programs\aidlc\bin`. It attempts a user-scoped PATH update, reports whether the
  path was already present, updated, or not changed, and prints deterministic new-terminal/IDE
  restart, manual user PATH, and `AIDLC_BIN` guidance.
- `aidlc upgrade` uses native GitHub release metadata and release asset downloads for CLI binary
  replacement, reusing `checksums.txt` verification and the existing release artifact naming
  scheme: `aidlc_<os>_<arch>.tar.gz` for Darwin/Linux, `aidlc_windows_<arch>.zip` for Windows, and
  `checksums.txt`.
- `.github/workflows/aidlc-release.yml` and `aidlc/scripts/build-release-assets.sh` own
  cross-platform static binary packaging, checksums, and GitHub release asset upload.
- `modernc.org/sqlite` is embedded only through `aidlc/internal/repomap/cache` to provide the local
  FTS5 query cache. The dependency must remain pure Go and compatible with CGO-disabled release
  builds. Repo-map query execution must not introduce embeddings, vector tables, model runtimes,
  network services, parser dependencies, language servers, search subprocesses, or external search
  engine runtimes.
- The interface-layer `aidlc/internal/cli` package may bind `aidlc/internal/repomap/cache`
  implementations into `repomap/model` interfaces as the CLI composition root. This exception does
  not allow application packages to import cache or SQLite packages directly.
- Local-source mode is allowed for tests and development fixtures.
- Normal init/update flows must not call Bash, Make, rsync, or git.
- Root Makefile init/update targets may invoke the native CLI for repository developer workflows,
  but they must not call retired `.ai/scripts/ai_init.sh` or `.ai/scripts/ai_update.sh` paths.
- `aidlc init` applies non-conflicting payload decisions, generates requested IDE files, and writes
  an honest partial root lock even when other payload paths conflict.
- `aidlc update` regenerates IDE files selected by `aidlc.lock.json` after a conflict-free
  mutating update. When the root lock is absent, update can fall back to legacy
  `.aidlc/manifest.json` and writes `aidlc.lock.json` only after a clean non-dry-run mutation.
- Forced update regenerates only the persisted workspace IDE surfaces and writes or migrates the
  root lock only after a non-dry-run forced mutation. `--dry-run --force` is read-only: it reports
  overwrite decisions without writing payload files, generated IDE files, `aidlc.lock.json`, or a
  legacy manifest migration.

## Test Gates

- `make aidlc-test`
- `make aidlc-release-check`
- `make test`
- `make validate-governance`

Persona rendering coverage must assert the exact architect, implementer, and reviewer model and
effort mappings for Codex and Claude Code, model-only `composer-2.5` output for all Cursor roles,
and omission of generated fields for empty or absent optional source values.

Coverage must include partial init conflicts that still write safe payload files, generated IDE
files, and an honest root lock; root `aidlc.lock.json` workspace IDE persistence; legacy manifest
fallback and migration timing; update regeneration of only selected IDE surfaces; unchanged update
conflict behavior; formatted CLI result output; Makefile wrapper invocation of the native CLI;
normalized tracked payload paths; license relocation to `licenses/aidlc.md`; preservation of a
consumer root `LICENSE`; mapped target lock entries; historical `LICENSE` update behavior where the
old tracked root path is reported as removed upstream without deletion; release asset selection for
supported platforms; release verification of the Unix shell installer default destination and
`AIDLC_INSTALL_DIR` override contract without network access; Unix installer fallback destination
and PATH guidance; Windows installer user-local default destination, user PATH update behavior, and
restart/manual guidance; Make helper `AIDLC_BIN`, PATH, common-location, and missing-binary failure
resolution; checksum verification before binary replacement; dry-run behavior without
archive downloads or destination writes; already-latest no-op behavior; upgrade command help and
root CLI routing; `aidlc doctor` help, routing, deterministic output, Make helper diagnostics, and
exit semantics; Windows installer default common-location discovery for sanitized PATH shells;
packaged-binary `aidlc upgrade --help` and `aidlc doctor --help` smoke coverage; and rendered governance
guidance parity where hardcoded spec-gate text exists. Cursor guidance tests must cover native Go
generation for scope-aware spec ownership invariants. Forced init/update coverage must prove
overwrite output rows, exit `0` on successful forced overwrites, lock tracking of overwritten public
payload paths, regenerated requested or persisted IDE files, private path exclusion, dry-run force
read-only behavior, unchanged non-forced conflict behavior, map/query root CLI help and routing,
query plan CLI coverage for `--plan-json`, `--plan-file`, malformed plan validation, invalid flag
combinations, and unchanged tab-separated output rows, repo-map build output with `docs/map/` JSONL
shards including `source_chunks.jsonl` and `symbols.jsonl`, `index.json`, and derived SQLite cache,
saved map whitelist persistence and reuse, first-run map include confirmation, non-interactive
first-run guidance, read-only `aidlc map --check` behavior, include mismatch stale output,
staleness exit codes, structured plan recall@10 of at least 0.85 across at least 14 labeled fixture
queries, raw text recall@10 of at least 0.70 across the same representative natural-language and
question-shaped code queries, compact `--limit 10` output at least 30 percent smaller than the
source-heavy baseline while preserving expected paths, JSONL fallback superset behavior for the
same queries, map artifact regeneration coverage, and `make aidlc-release-check` coverage proving
the SQLite dependency does not break CGO-disabled cross-compilation and no model, vector, parser,
network-service, or search-engine runtime dependency is introduced.
