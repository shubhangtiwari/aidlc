# ADR-1790867975: Token-Efficient Harness and Adaptive Local Retrieval

- **Status:** Accepted
- **Date:** 2026-10-01
- **Deciders:** Shubhang Tiwari

## Context

The current AIDLC harness preserves valuable properties: explicit ownership, portable governance,
scope-local specs, blueprint contracts, repo-map-first discovery, deterministic local CLI behavior,
and installable IDE projections. The honest harness audit also found that the current projection is
too heavy for everyday use. Codex and unified outputs can embed full persona and skill bodies into
root entrypoints; Codex and Cursor both write `AGENTS.md`; the default route delegates too much
low-risk work; current retrieval can return no rows for raw cache misses even when JSONL fallback
could help; source chunks omit file tails after a fixed early cap; and validation is mostly passive
guidance rather than executable checks.

Existing accepted ADRs constrain the redesign. ADR-1780346463 keeps the CLI as the portable native
sync and generation surface. ADR-1781478129 keeps SQLite as a derived cache over committed JSONL
map shards. ADR-1784130102 explicitly rejected search subprocesses as part of repo-map query
execution. Allowing bounded exact/path search therefore requires a new decision.

## Decision

Adopt a token-efficient harness design that keeps AIDLC's ownership and contract model while making
the common path smaller and less ceremonial.

Generated root entrypoints will become compact canonical launchers. They will include project
facts, the shared routing contract, and compact catalogs pointing to lazy persona and skill bodies.
Full persona and skill bodies remain in native IDE locations such as `.codex/agents/`,
`.codex/skills/`, `.cursor/agents/`, `.cursor/skills/`, and `.claude/**`; root `AGENTS.md` is a
deterministic shared file when both Codex and Cursor are generated. IDE-specific catalogs stay in
their native folders rather than competing in the root entrypoint.

AIDLC will also support consumer-owned installed skills through a local CLI surface:
`aidlc skill install LOCAL_DIR`, `aidlc skill list`, and `aidlc skill remove NAME`. Installed skills
live under `.ai/skills/installed/<name>/`, are excluded from upstream payload sync even during
`update --force`, and are projected into native IDE skill folders during init/update regeneration.
The installer copies a full local skill tree after validating `SKILL.md` metadata, rejecting unsafe
paths, symlinks, bundled-name collisions, and installed-name collisions. It does not execute skill
scripts or follow source symlinks. Removal deletes only AIDLC-owned unmodified IDE projections and
preserves/report diverged projections.

Governance will route by semantic risk and reversibility. Low-risk governed edits may be performed
by the primary agent after triage and an inline intent, without redundant human approval or
delegation. Medium, large, high-risk, or uncertain work still requires explicit approval and an
independent correctness review. Specs remain proportional: parallel work packages are optional and
Wave 0 is used only for shared contracts or fixtures that multiple implementers depend on.

Repo-map retrieval remains local and deterministic, with JSONL shards and SQLite FTS as the primary
structural index. Add an optional bounded exact/path discovery channel expressed through shared
DTOs in the repomap contracts layer and implemented by an infrastructure adapter that may call
`rg` when the query contains high-signal path, symbol, or literal clues and `rg` is available. This
channel is budgeted, slash-relative, time-bounded, executed through an argument vector rather than
a shell, and fused with existing map results. It must not become a required runtime dependency,
must not call network services, and must surface omissions or fallback diagnostics through opt-in
diagnostics without changing default TSV output. If `rg` is unavailable, query continues with the
structural map and JSONL fallback.

Workflow evidence will be represented by a compact versioned local task record under the local
scope, for example `docs/tasks/<task-id>.task.json`. Task records are opt-in for tiny fixes and
required only where validation gates need machine-readable evidence for approved specs, high-risk
scope boundaries, file ownership, completed checks, source freshness, review evidence, or open next
steps. Source freshness includes a dirty-tree content digest over the owned files, not HEAD alone.
The CLI may validate schema shape and evidence consistency against the current repository contents,
but it must not claim to prove human approval, self-authenticate a task record, prove model review,
prove host-enforced write isolation, actual LLM token usage, or end-to-end cost savings.

Benchmarks will use deterministic proxy metrics: unknown-location retrieval recall, repeated reads,
retry counts, input/output byte or word budgets, and optional host-reported token or cost values.
The CLI must label such values honestly and must not imply a tokenizer-accurate measurement unless
the host supplies one.

## Consequences

- Root generated guidance becomes smaller and more stable while retaining the lazy detailed bodies
  needed by each IDE.
- Existing consumer repositories can be updated without losing supported `.codex/**`,
  `.cursor/**`, `.claude/**`, Copilot, or Windsurf outputs.
- Consumer-owned installed skills survive payload updates and can be explicitly reinstalled when
  their local source changes.
- The governance path becomes more usable for safe, reversible work while preserving approval and
  independent review for risky contract, state, topology, integration, or public workflow changes.
- `rg` improves exact and unknown-location discovery where available, but the CLI remains useful
  without it and does not add a new binary distribution dependency.
- Validation becomes more executable, but remains honest about what local tooling can and cannot
  prove.
- Existing cache and JSONL contracts remain backward compatible; any new schemas must be versioned.

## Alternatives Considered

| Alternative | Why rejected |
| --- | --- |
| Keep full bodies in root generated entrypoints | Preserves simplicity in the generator but keeps paying large prompt overhead on every session and worsens Codex/Cursor root-file collisions. |
| Remove persona and skill body projections entirely | Saves tokens but weakens supported IDE behavior and portable distribution. Lazy native body files preserve capability without loading everything into root guidance. |
| Require delegation and approval for all governed edits | Maximizes ceremony and consistency but blocks reversible low-risk work that the audit identified as unnecessarily expensive. |
| Replace the repo map with `rg` | Loses structural records, source freshness, committed navigation evidence, and deterministic JSONL fallback. The new exact-search channel is an augmentation, not a replacement. |
| Add model or embedding retrieval | Rejected by ADR-1784130102 and still out of scope because it adds privacy, cost, runtime, and distribution concerns. |
| Make task records mandatory for every edit | Creates the same ceremony problem the redesign is meant to reduce. The record should be required where it supports validation, and optional for tiny fixes. |
