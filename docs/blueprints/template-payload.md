# Template Payload Blueprint

## Package Purpose

The template payload is the set of repository files that may be copied into a consumer project by
`aidlc init`, `aidlc update`, or compatibility tooling. Payload membership is intentionally smaller
than this repository because this repository also contains AIDLC product implementation,
architecture, in-flight specifications, CI, and release machinery.

## Package Boundary

Payload membership is declared by `.ai/template-manifest.yaml`. The source repository may contain
many governance and implementation files that are not public payload, and those files must not be
copied to initialized or updated consumer repositories.

## Public Payload Contract

The public payload is a strict allowlist:

- `.ai/**` files intentionally listed in `.ai/template-manifest.yaml`.
- Starter docs or templates intentionally listed in `.ai/template-manifest.yaml`.
- License files intentionally listed in `.ai/template-manifest.yaml`.

Broad directory copying is forbidden. In particular, `docs/**` is never public by directory; each
public starter document must be listed explicitly. Manifest includes are normalized relative paths;
absolute paths, parent traversal, Windows drive paths, and broad globs are rejected by the CLI
payload policy.

## Read-only Paths

The following repository-local paths are non-payload unless a future ADR and manifest update
explicitly narrows a starter exception:

- `docs/spec/*.md`
- non-public `docs/adr/*.md`
- non-public `docs/blueprints/*.md`
- `docs/ARCHITECTURE.md`
- `docs/architecture/**`
- `aidlc/**`
- `.github/**`
- `release/**`, `dist/**`, `build/**`
- release and implementation files, including `aidlc/.goreleaser.yaml` and `aidlc/scripts/**`

## Update Semantics

- Init is additive: create missing files, skip identical files, and report divergent existing files
  as conflicts.
- Update is checksum-aware: compare prior manifest checksums, local checksums, and upstream
  checksums before writing.
- Unknown local project files are not deleted.
- Files removed upstream are reported through planning state and are not blindly deleted from target
  repositories.
- `.aidlc/manifest.json` records tracked public payload files, upstream source/ref/commit,
  checksums, modes, generation metadata, and command metadata. Update decisions use that manifest
  instead of scanning broad repository directories.

## Test Gates

- `make validate-governance`
- `make aidlc-test`
- `make test`
