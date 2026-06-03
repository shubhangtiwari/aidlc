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
- Native IDE generation, manifest-aware payload sync, installers, and release checks.

It does not own root template source files except by reading the public template manifest as input.

## Layer Map

| Path | Layer | Notes |
| --- | --- | --- |
| `aidlc/cmd/aidlc` | Interface | Executable entrypoint. |
| `aidlc/internal/cli` | Interface | Root command wiring and process-facing concerns. |
| `aidlc/internal/commands` | Application | Init, update, version orchestration. |
| `aidlc/internal/generator` | Application | IDE file generation. |
| `aidlc/internal/sync` | Application | Manifest-aware planning and copy decisions. |
| `aidlc/internal/source` | Infrastructure | GitHub archive and local-source access. |
| `aidlc/internal/install` | Infrastructure | Installer and checksum validation. |
| `aidlc/internal/contract` | Contracts | Shared command, IDE, manifest, and option types. |
| `aidlc/internal/payload` | Contracts | Public payload path normalization and exclusion policy. |
| `aidlc/internal/testutil` | Test Support | Filesystem helpers for tests. |

## Cross-package Contracts

- Commands: `init`, `update`, and `version`.
- `aidlc init <claude|codex|cursor|copilot|windsurf|all> [--source github|local] [--url URL]
  [--ref REF] [--path PATH] [--dry-run]` copies the public template payload and then generates the
  requested IDE files, recording concrete workspace IDEs in `aidlc.lock.json`.
- `aidlc update [--source github|local] [--url URL] [--ref REF] [--path PATH] [--dry-run]` reads
  `aidlc.lock.json`, or legacy `.aidlc/manifest.json` when the root lock is absent, fetches the
  configured or overridden source, applies clean manifest-aware updates, and regenerates the
  persisted workspace IDE surfaces.
- `aidlc version` prints the CLI version string.
- Supported IDEs: `claude`, `codex`, `cursor`, `copilot`, `windsurf`, and aggregate `all`.
- Target lock: root `aidlc.lock.json` records schema version, upstream source/ref/commit,
  authoritative `workspace.ides`, generated IDE metadata, tracked payload paths, file checksums,
  file modes, and command metadata such as source kind and local source path. `workspace.ides`
  stores concrete IDE identifiers only, de-duplicated in canonical supported IDE order; `all`
  expands to every concrete IDE before persistence.
- Legacy fallback: when `aidlc.lock.json` is absent, `.aidlc/manifest.json` may supply
  `workspace.ides`; if that field is absent, `generated.ide` is used, with `generated.ide: all`
  expanded to every concrete IDE. Root `aidlc.lock.json` takes precedence when both files exist.
- Public template manifest: `.ai/template-manifest.yaml` allowlists public paths and documents
  blocked private paths.
- Payload policy: only normalized relative paths are valid; absolute paths, parent traversal,
  empty paths, and Windows drive paths are rejected.
- Native IDE generation emits generated governance guidance from the portable `.ai/` contract.
  Cursor rule rendering is part of that generated guidance contract: spec-gate text must follow the
  portable scope-resolution rule for nested initialized AIDLC roots, including scope-local
  `docs/spec/<epoch>-<slug>.md` ownership and parent-scope refusal for files owned by nested scopes.
- Repository `make init <ide>` and `make update` targets are thin wrappers around the native CLI.
  They do not define a separate Bash compatibility contract.
- Exit behavior: successful no-op exits `0`, conflicts exit `1`, and invalid usage exits `2`.

## Owned State

Source repository state owned by `aidlc/` is limited to the isolated Go module, tests, testdata,
release configuration, installers, and release verification scripts. Target repositories may
receive `aidlc.lock.json`, public template payload files, and generated IDE files such as
`AGENTS.md`, `CLAUDE.md`, `.codex/**`, `.cursor/**`, `.claude/**`,
`.github/copilot-instructions.md`, and `.windsurfrules`. The root lock owns workspace IDE
selections, tracked payload checksums, generation metadata, and update source metadata. Legacy
`.aidlc/manifest.json` may still be read for compatibility but is no longer the authoritative write
target.

## Integration Boundaries

- GitHub archive access is used to fetch template payload snapshots for normal init/update flows.
- GitHub release artifacts and checksum files are consumed by shell and PowerShell installers.
- `.github/workflows/aidlc-release.yml` and `aidlc/scripts/build-release-assets.sh` own
  cross-platform static binary packaging, checksums, and GitHub release asset upload.
- Local-source mode is allowed for tests and development fixtures.
- Normal init/update flows must not call Bash, Make, rsync, or git.
- Root Makefile init/update targets may invoke the native CLI for repository developer workflows,
  but they must not call retired `.ai/scripts/ai_init.sh` or `.ai/scripts/ai_update.sh` paths.
- `aidlc update` regenerates IDE files selected by `aidlc.lock.json` after a conflict-free
  mutating update. When the root lock is absent, update can fall back to legacy
  `.aidlc/manifest.json` and writes `aidlc.lock.json` only after a clean non-dry-run mutation.

## Test Gates

- `make aidlc-test`
- `make aidlc-release-check`
- `make test`
- `make validate-governance`

Coverage must include root `aidlc.lock.json` workspace IDE persistence, legacy manifest fallback and
migration timing, update regeneration of only selected IDE surfaces, Makefile wrapper invocation of
the native CLI, normalized tracked payload paths, and rendered governance guidance parity where
hardcoded spec-gate text exists. Cursor guidance tests must cover native Go generation for
scope-aware spec ownership invariants.
