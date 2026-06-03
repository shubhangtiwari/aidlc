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

Public governance guidance in `.ai/**` and the starter `docs/spec/README.md` defines scope-aware
spec ownership: medium and large specs are local to the resolved AIDLC scope root that owns the
affected paths. Repository-local in-flight specs such as `docs/spec/[0-9]*-*.md` are implementation
artifacts for this repository and must remain excluded from payload copying.

## Read-only Paths

The following repository-local paths are non-payload unless a future ADR and manifest update
explicitly narrows a starter exception:

- `docs/spec/*.md`, except the public starter `docs/spec/README.md`
- non-public `docs/adr/*.md`
- non-public `docs/blueprints/*.md`
- `docs/ARCHITECTURE.md`
- `docs/architecture/**`
- `aidlc/**`
- `.github/**`
- `release/**`, `dist/**`, `build/**`
- release and implementation files, including `.github/**`, `aidlc/**`, and `aidlc/scripts/**`

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
- Payload updates may refresh scope-resolution guidance in initialized roots, including
  `.ai/**` guidance and the public starter `docs/spec/README.md`.
- Payload updates must not move, delete, import, or overwrite local scoped specs. Numbered
  `docs/spec/[0-9]*-*.md` files remain local planning artifacts outside the public payload.

## Test Gates

- `make validate-governance`
- `make aidlc-test`
- `make test`

Validation must preserve the explicit `docs/spec/README.md` manifest include, the numbered
`docs/spec/[0-9]*-*.md` exclusions, and the prohibition on broad `docs/**` payload copying.
