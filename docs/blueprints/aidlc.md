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
| `aidlc/internal/sync` | Application | Manifest-aware planning and copy decisions. |
| `aidlc/internal/source` | Infrastructure | GitHub archive and local-source access. |
| `aidlc/internal/install` | Infrastructure | Installer, release download, checksum validation, archive extraction, and binary replacement. |
| `aidlc/internal/contract` | Contracts | Shared command, IDE, manifest, and option types. |
| `aidlc/internal/payload` | Contracts | Public payload path normalization and exclusion policy. |
| `aidlc/internal/testutil` | Test Support | Filesystem helpers for tests. |

## Cross-package Contracts

- Commands: `init`, `update`, `upgrade`, and `version`.
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
  file modes, and command metadata such as source kind and local source path. `workspace.ides`
  stores concrete IDE identifiers only, de-duplicated in canonical supported IDE order; `all`
  expands to every concrete IDE before persistence. Tracked payload paths are destination paths:
  the AIDLC repository root `LICENSE` payload is tracked in target repositories as
  `licenses/aidlc.md`, leaving any consumer root `LICENSE` outside AIDLC ownership.
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
- Repository `make init <ide>` and `make update` targets are thin wrappers around the native CLI.
  They do not define a separate Bash compatibility contract.
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
`.github/copilot-instructions.md`, and `.windsurfrules`. The consumer repository root `LICENSE` is
not owned by AIDLC after license relocation; init and update must not create, overwrite, delete, or
track it as the active AIDLC license payload. The root lock owns workspace IDE selections, tracked
payload checksums, generation metadata, and update source metadata. Legacy `.aidlc/manifest.json`
may still be read for compatibility but is no longer the authoritative write target. During
conflicted init, the partial root lock records only accepted upstream payload files plus
generation/workspace metadata; conflicted payload paths are excluded from tracked clean file entries.
Forced init and forced update may replace divergent public payload destination files and then record
those overwritten paths as clean tracked files in `aidlc.lock.json`. Removed-upstream files,
private paths, unknown local files, and local-only files remain outside forced deletion or overwrite
behavior.
`aidlc upgrade` owns only the installed `aidlc` executable at the resolved install destination and
temporary staging files in that destination directory during replacement. It does not modify target
repository payload state, `aidlc.lock.json`, generated IDE files, or legacy `.aidlc/manifest.json`.

## Integration Boundaries

- GitHub archive access is used to fetch template payload snapshots for normal init/update flows.
- Local and GitHub/archive source providers may read payload bytes and modes from a source path that
  differs from the emitted target path for explicit manifest mappings. Normal init/update flows
  still use native filesystem, archive, checksum, and rendering logic and avoid shelling out.
- GitHub release artifacts and checksum files are consumed by shell and PowerShell installers.
- `aidlc upgrade` uses native GitHub release metadata and release asset downloads for CLI binary
  replacement, reusing `checksums.txt` verification and the existing release artifact naming
  scheme: `aidlc_<os>_<arch>.tar.gz` for Darwin/Linux, `aidlc_windows_<arch>.zip` for Windows, and
  `checksums.txt`.
- `.github/workflows/aidlc-release.yml` and `aidlc/scripts/build-release-assets.sh` own
  cross-platform static binary packaging, checksums, and GitHub release asset upload.
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

Coverage must include partial init conflicts that still write safe payload files, generated IDE
files, and an honest root lock; root `aidlc.lock.json` workspace IDE persistence; legacy manifest
fallback and migration timing; update regeneration of only selected IDE surfaces; unchanged update
conflict behavior; formatted CLI result output; Makefile wrapper invocation of the native CLI;
normalized tracked payload paths; license relocation to `licenses/aidlc.md`; preservation of a
consumer root `LICENSE`; mapped target lock entries; historical `LICENSE` update behavior where the
old tracked root path is reported as removed upstream without deletion; release asset selection for
supported platforms; checksum verification before binary replacement; dry-run behavior without
archive downloads or destination writes; already-latest no-op behavior; upgrade command help and
root CLI routing; packaged-binary `aidlc upgrade --help` smoke coverage; and rendered governance
guidance parity where hardcoded spec-gate text exists. Cursor guidance tests must cover native Go
generation for scope-aware spec ownership invariants. Forced init/update coverage must prove
overwrite output rows, exit `0` on successful forced overwrites, lock tracking of overwritten public
payload paths, regenerated requested or persisted IDE files, private path exclusion, dry-run force
read-only behavior, and unchanged non-forced conflict behavior.
