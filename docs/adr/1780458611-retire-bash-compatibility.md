# ADR-1780458611: Retire Bash Init/Update Compatibility

## Status

Accepted

## Context

AIDLC now has a native Go CLI that initializes and updates governance payloads across macOS, Linux,
and Windows without requiring Bash, Make, rsync, git, or a language runtime in target repositories.
The repository still carries Bash init/update scripts, public payload manifest entries, Makefile
invocations, and compatibility tests from the CLI rollout period.

Keeping Bash init/update compatibility as a public contract would require ongoing parity work for an
obsolete path even though the native CLI owns manifest-aware sync, lock persistence, IDE
regeneration, and cross-platform behavior.

## Decision

Retire Bash init/update compatibility. The supported user-facing init/update behavior is the native
`aidlc init` and `aidlc update` CLI.

Keep this repository's root `make init <ide>` and `make update` targets as thin wrappers around the
native CLI. They remain repository execution entrypoints, but they no longer run
`.ai/scripts/ai_init.sh` or `.ai/scripts/ai_update.sh` and no longer define a separate shell-script
compatibility contract.

Remove `.ai/scripts/ai_init.sh` and `.ai/scripts/ai_update.sh` from the public payload manifest.
Retain `.ai/scripts/finalize_spec.sh` because post-merge spec cleanup is a separate governance
maintenance flow.

## Consequences

- Normal init/update behavior has one supported implementation: the native `aidlc` CLI.
- Public payload consumers no longer receive Bash init/update scripts.
- `make init <ide>` and `make update` remain available in this repository as native CLI wrappers.
- Tests and validation should assert the native payload contract instead of Bash parity.
- Shell-specific compatibility work is no longer required for init/update changes.
