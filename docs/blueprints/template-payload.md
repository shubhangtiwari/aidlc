# Template Payload Blueprint

## Package Purpose

The template payload is the set of repository files that may be copied into a consumer project by
`aidlc init` or `aidlc update`. Payload membership is intentionally smaller than this repository
because this repository also contains AIDLC product implementation, architecture, in-flight
specifications, CI, and release machinery.

## Package Boundary

Payload membership is declared by `.ai/template-manifest.yaml`. The source repository may contain
many governance and implementation files that are not public payload, and those files must not be
copied to initialized or updated consumer repositories.

## Public Payload Contract

The public payload is a strict allowlist:

- `.ai/**` files intentionally listed in `.ai/template-manifest.yaml`.
- Reference architecture profiles under `.ai/references/architectures/**`, intentionally listed
  file-by-file in `.ai/template-manifest.yaml`, because `init-architecture` depends on them in
  initialized repositories.
- Starter docs or templates intentionally listed in `.ai/template-manifest.yaml`.
- License files intentionally listed in `.ai/template-manifest.yaml`.

Manifest includes may be same-path entries, where the source repository path and target repository
path match, or explicit single-file source-to-target entries. The public AIDLC license uses the
mapped form: it reads the repository root `LICENSE` and installs that content to target
`licenses/aidlc.md`. The manifest must not broaden this into a `licenses/**` or root-license
directory copy.

`.ai/scripts/ai_init.sh` and `.ai/scripts/ai_update.sh` are not public payload entries. Native
`aidlc init` and `aidlc update` own supported init/update behavior. `.ai/scripts/finalize_spec.sh`
remains public because spec finalization is a separate governance maintenance flow.

Broad directory copying is forbidden. In particular, `docs/**` is never public by directory; each
public starter document must be listed explicitly. Manifest source and target paths are normalized
relative paths; absolute paths, parent traversal, Windows drive paths, private target paths,
duplicate target paths, and broad globs are rejected by the CLI payload policy.

Public governance guidance in `.ai/**` and the starter `docs/spec/README.md` defines scope-aware
spec ownership and tier classification by semantic risk, contract impact, target state, topology,
integration changes, and coordination cost. File count and line count are evidence to inspect, not
automatic tier gates: multi-file low-risk mechanical edits may remain small, while one-file changes
that alter public behavior, schemas, owned state, contracts, integrations, or workflow topology can
be medium or large. Repository-local in-flight specs such as `docs/spec/[0-9]*-*.md` are
implementation artifacts for this repository and must remain excluded from payload copying.

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
- retired init/update compatibility scripts, including `.ai/scripts/ai_init.sh` and
  `.ai/scripts/ai_update.sh`

## Update Semantics

- Init is additive: create missing files, skip identical files, and report divergent existing files
  as conflicts. When some payload paths conflict, init still applies non-conflicting payload
  decisions without overwriting divergent local files.
- Update is checksum-aware: compare prior manifest checksums, local checksums, and upstream
  checksums before writing.
- Conflicts are non-destructive by default. Explicit force converts conflicting public payload
  destination paths into overwrite decisions, but it does not delete removed-upstream files and does
  not make private paths, unknown local files, or local-only files writable.
- Unknown local project files are not deleted.
- Payload planning, checksum comparison, write decisions, filesystem writes, and target manifest
  tracking use destination paths. For mapped entries, source bytes and modes come from the source
  repository path, while target conflict detection and lock entries use the mapped destination path.
- Historical tracked paths that are no longer present in the destination path set are reported
  through planning state as removed upstream and are not deleted from target repositories.
- `.aidlc/manifest.json` records tracked public payload files, upstream source/ref/commit,
  checksums, modes, generation metadata, and command metadata. Update decisions use that manifest
  instead of scanning broad repository directories.
- Payload updates may refresh scope-resolution and tier-classification guidance in initialized
  roots, including `.ai/**` guidance and the public starter `docs/spec/README.md`.
- Payload updates must not move, delete, import, or overwrite local scoped specs. Numbered
  `docs/spec/[0-9]*-*.md` files remain local planning artifacts outside the public payload.

## Test Gates

- `make validate-governance`
- `make aidlc-test`
- `make test`

Validation must preserve the explicit `docs/spec/README.md` manifest include, the numbered
`docs/spec/[0-9]*-*.md` exclusions, and the prohibition on broad `docs/**` payload copying.
Coverage must include mapped manifest entries, path normalization for both source and target paths,
private path rejection for target paths, duplicate target rejection, destination-path lock tracking,
and the prohibition on broad license directory copying.
