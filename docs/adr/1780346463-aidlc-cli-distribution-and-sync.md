# ADR-1780346463: AIDLC CLI Distribution and Sync

## Status

Accepted

## Context

AIDLC is currently consumed by cloning or copying this template repository and running Bash/Make
entrypoints. That creates friction for non-Unix environments and risks copying repository-local
implementation artifacts into consumer projects. The new CLI must preserve existing Make/Bash
compatibility while providing a native installable surface.

## Decision

Build `aidlc` as a Go static-binary CLI in an isolated module rooted at `aidlc/`.

The root repository will not contain a Go manifest. All Go tests, builds, and release checks run
from the `aidlc/` directory through root Make targets.

Template payload membership is controlled by `.ai/template-manifest.yaml`, a strict public
allowlist. The CLI must only copy paths explicitly listed there. It must not treat broad directories
such as `docs/`, `.github/`, or `aidlc/` as payload.

Target repositories receive a manifest-aware sync model. `aidlc init` is additive and
non-destructive. `aidlc update` compares the prior target manifest, local checksums, and upstream
checksums before writing. Divergent local files are reported as conflicts instead of overwritten,
and unknown local project files are not deleted.

## Consequences

- Cross-platform installation can distribute macOS, Linux, and Windows binaries without requiring
  Bash, Make, rsync, git, or a language runtime for normal users.
- Existing `make init <ide>` and `make update` remain available during and after the CLI rollout.
- Release tooling must stay under `aidlc/` or repository automation paths and must not become part
  of public template payload.
- Consumer repositories only receive explicitly public governance payload, not in-flight specs,
  repository architecture docs, CLI source, CI, or release implementation files.
