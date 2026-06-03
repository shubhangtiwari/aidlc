# `.ai/` — Project-Agnostic AI Guidance

This directory holds only project-agnostic guidance for AI assistants: how they should code, what
skills they have, and which personas they can adopt. It carries no project-specific facts.

Project-specific facts live in canonical project files generated or curated after fork:

- Architecture style, layer rules, execution model → `docs/ARCHITECTURE.md` and
  `docs/architecture/<domain>.md` (created by the `init-architecture` skill).
- Module-specific contracts, owned state, and read-only paths → module blueprints under
  `docs/blueprints/`.
- Test gates and standard commands → `Makefile`.
- Persona model defaults per IDE → `.ai/models.defaults.toml` (projected by `aidlc init <ide>` or
  this repository's `make init <ide>` wrapper).

The native `aidlc init <ide>` generator reads `.ai/` and optionally enriches output from a project
manifest when one is detected at the repo root. Run `aidlc init <ide>` without a manifest to project
governance only; re-run after adding a language/runtime manifest to refresh toolchain facts. In this
repository, `make init <ide>` remains a thin wrapper around the native CLI.
**Do not edit generated IDE files by hand.**

## Files in this directory

- `models.defaults.toml` — model and reasoning defaults for cursor, codex, claude.
- `personas/` — operating modes for AI agents.
  - `architect.md` — plans, work packages, DAG; never edits without instruction.
  - `implementer.md` — WP-scoped edits within layer rules.
  - `reviewer.md` — validates layer purity, spec, and WP `done_when`.
- `skills/` — task playbooks (projected per IDE by `make init <ide>`).
  - `classify-change.md` — mandatory tier triage in the main session before governed implementation.
  - `init-architecture.md` — analyze repo and write `docs/ARCHITECTURE.md`.
  - `orchestrate-spec.md` — parallel implementer waves from spec work packages.
- `templates/` — spec drafting template; approval brief template (chat-only, for architect).
- `references/architectures/` — reference profiles for the `init-architecture` skill.
- `scripts/` — maintenance scripts (`finalize_spec.sh`).

## Model defaults

Edit `.ai/models.defaults.toml` to change which model each IDE uses for each persona, then run
`make init <ide>` to regenerate agent files.

## Responsibility Boundaries

- **Personas define authority:** who plans, who changes files, who reviews, what each role must
  refuse, when work escalates, and which quality bar applies before handoff.
- **Skills define procedure:** reusable task playbooks in `.ai/skills/`. Skills do not define
  authority — personas and Hard Rules do.
- **Docs define truth:** architecture, specs, ADRs, blueprints, and manifests hold project facts.

<!-- INIT:BEGIN -->
<!-- Everything below this marker is copied verbatim into per-IDE entrypoints by aidlc init. -->

## Main agent delegation

Portable governance for the main session. Persona bodies live in `.ai/personas/`. Skills live in
`.ai/skills/`. Architecture lives in `docs/ARCHITECTURE.md` and `docs/architecture/` once
`init-architecture` has run; before then there is no governed source tree.

### Clarify before acting

Before classifying or delegating, restate the request in your own terms and surface any ambiguity.
If the goal, scope, target files, success criteria, or tier is genuinely unclear — and the gap
would change which path you take below — ask the user a focused question first. Prefer one or two
sharp questions over a list; do not ask about details you can determine by reading the repo. Skip
clarification only when the request is unambiguous or the user has already answered. This step
runs before the spec gate and before any tool call that changes state.

### Spec gate

The main agent must **not** edit governed paths directly when tier is medium, large, or uncertain,
or when the user asks to implement, fix, add, refactor, or ship without an approved spec. User goal
verbs do **not** waive the spec gate.

### Scope resolution

Governance is resolved from the **invocation root**: the AIDLC root where the current agent session
or prompt started. An **AIDLC scope root** is the invocation root or a directory below it that
contains both `.ai/README.md` and `docs/spec/README.md`. Generated IDE files such as `AGENTS.md`,
`.codex/**`, `.cursor/**`, or `CLAUDE.md` are not scope markers.

For each affected path, resolve the owning scope by walking from the path's directory upward to the
invocation root and selecting the nearest AIDLC scope root. If no nested scope is found, fall back to
the invocation root. Paths outside the invocation root are outside this governed workflow unless the
user explicitly changes the invocation scope or starts a separate governed session there.

Medium, large, or uncertain requests that map affected files to multiple AIDLC scope roots require
one draft spec per resolved scope. Each scoped spec lives at
`docs/spec/<epoch>-<slug>.md` relative to that scope and may include only files owned by that scope.
A parent scoped spec must not claim files below a nested initialized AIDLC scope. One approval brief
may summarize multiple scoped draft specs for the same user request, but implementation and review
treat each approved scoped spec as its own governing artifact.

### Before any state-changing tool call

1. **Choose the path:**
   - **Configuration / tooling / Q&A** (`Makefile` when not a contract, `.ai/`, IDE
     config, dependencies, exploration) → main agent may proceed (no triage).
   - **Governed code or contracts** (project source tree, `tests/`, `docs/blueprints/`,
     `docs/adr/`, `docs/spec/`, `docs/ARCHITECTURE.md`, `docs/architecture/`) → **triage first**
     (below), then the persona chain. The main session **must not** patch governed source or tests —
     only `implementer` (or a delegated WP implementer) applies those edits.
2. **Triage (governed only):** main session applies skill **`classify-change`** (read the playbook;
   post a **Triage record** in chat) before inline intent, spec drafting, or `implementer`. Do not
   delegate triage to `architect`. Route from `next`: `inline-intent` | `draft-spec` | `ask-user`.
   When `next` is `draft-spec`, **delegate `architect`** automatically for planning. See Hard Rule 7.
3. **Classify by semantic risk,** not raw file count alone. File count, line count, and directory
   spread are signals to inspect for coordination cost, but the tier is driven by contract impact,
   public behavior, owned state, topology, integrations, and rollback risk.
4. **When in doubt,** assign tier **uncertain** or **medium**, not small.

### Persona chain

| Step | Persona | When |
| --- | --- | --- |
| 1 | `architect` | **Planning only** — after main-session triage with `next: draft-spec` (medium / large / uncertain) |
| 2 | `implementer` | After spec `status: approved`, or after trivial/small intent confirmed with user |
| 3 | `reviewer` | **Medium / large / uncertain (after approved spec) only** — finished diff vs governing spec; **not** used for trivial / small |

**Medium / large / uncertain:** main session runs `classify-change`; when `next: draft-spec`,
**delegate `architect`** for planning. The architect writes the scope-local spec file(s) to disk and
posts an **approval brief** in chat (see `.ai/templates/approval-brief.md`). Return the spec path(s)
and brief summary to the user, then **stop** until they explicitly approve. Do not call
`implementer` in the same turn. Do not paste the full spec into chat — the brief is the human gate;
the scoped spec file is the machine gate. After all implementer work (including `orchestrate-spec`
waves), the main session **must** delegate `reviewer` before reporting implementation complete or
opening a PR — see **Review** and Hard Rule 6.

**Trivial / small:** After triage (`next: inline-intent`). No spec file. Main agent states intent
inline (short summary: what, which files, expected outcome) informed by the Triage record. User
confirms. Delegate **`implementer`** — do not apply governed edits from the main session. **Do not**
delegate `reviewer` unless the user explicitly asks for a review. Small work can span a few files
when the change is mechanical, uses a bounded helper, or adjusts one localized flow without changing
module contracts, owned state, integration boundaries, or topology.

**Blueprint sanity (all tiers):** Every `implementer` run ends with a blueprint check. If the
change touches anything blueprints document (contracts, owned state, read-only paths, integrations,
topology, layer map, test gates), update `docs/blueprints/<module>.md` in the same branch. If not,
leave blueprints unchanged — do not add noise. Medium/large work still lists deltas in the spec;
trivial/small rely on this check instead of a spec file.

### Workflow

```text
Governed (all tiers):
  main session + skill classify-change → Triage record (chat)
       ↓
  next: ask-user → main asks → classify-change again
  next: inline-intent → main inline intent → user confirms → implementer → done
  next: draft-spec → delegate architect → scope-local spec file(s) + approval brief (chat)
       ↓ user approves
       main → implementer (per WP / orchestrate-spec) → reviewer → merge
```

**Human gate:** approval brief in chat for medium/large (~250–500 words); short inline intent for
trivial/small. **Machine gate:** spec file on disk when tier requires it.

### Escalation

- **Minor discovery** → spec `Implementation notes` with date; continue.
- **Material change** → stop; architect amends spec.

### Work packages and parallel execution

For medium/large specs with `work_packages` in frontmatter:

1. Read waves from the spec's **Parallel execution plan**.
2. Execute **one wave at a time**. All work packages in wave *N* must finish before wave *N+1*.
3. Spawn **one implementer per work package** in the current wave (cap concurrency at 3–6; queue
   the rest in the same wave).
4. Each implementer receives: spec path, WP id, allowed `files`, `gates`, `done_when`, and
   `domain`.
5. **One writer per path** per active wave — refuse overlapping file ownership.
6. **Wave 0** freezes shared contracts: models/DTOs, integration interfaces, shared pure utils
   (2+ consumers), shared test fixtures. Not feature-complete services.
7. **WP-INT** (final wave): wire-up, integration tests, blueprint sync, cross-cutting barrels —
   `depends_on: [*]`.
8. Apply skill `orchestrate-spec` for the step-by-step playbook.

### Human vs machine artifacts

| Audience | Artifact | Location |
| --- | --- | --- |
| Human (approval) | Approval brief | Chat only — architect synthesis |
| Agents (execution) | Spec | `<scope-root>/docs/spec/<epoch>-<slug>.md` on disk |

The main agent may restate the brief for clarity but must not regenerate a full plan or duplicate
the spec body in chat.

### Review

| Tier | Reviewer |
| --- | --- |
| **Trivial / small** | **Skip** — implementer + blueprint sanity is sufficient. Invoke `reviewer` only when the user explicitly asks (e.g. “review this diff”). |
| **Medium / large / uncertain** (approved spec) | **Required** — delegate `reviewer` on the branch diff vs the governing spec after implementer finishes. |

For medium and large work, the main session must not tell the user implementation is complete,
open a PR, or end the governed workflow until `reviewer` has run (same session or next turn is
fine; skipping is not). Input to `reviewer`: diff + spec path + `status: approved` frontmatter.

Trivial/small: verify blueprint sanity during implementer; no mandatory `reviewer` pass.

## Hard Rules

1. Layer purity follows `docs/architecture/` for the spec's `domain` once initialized.
2. Execute through `Makefile` targets only.
3. Cross-cutting decisions require an ADR in `docs/adr/` before implementation.
4. Medium and large changes require an approved spec before code is written.
5. The main agent must enforce the spec gate before any state-changing action.
6. For **medium and large** governed changes (approved spec on disk), the main agent must delegate
   `reviewer` after implementer completes and before reporting implementation complete or merge.
   **Trivial and small** changes must **not** use `reviewer` unless the user explicitly requests it.
7. For **governed** implementation requests, the main session must apply skill **`classify-change`**
   (triage playbook + Triage record in chat) before inline intent, spec drafting, or `implementer`.
   Triage stays in the main session — **do not** delegate it to `architect`. When triage yields
   **medium, large, or uncertain** (`next: draft-spec`), the main session **must** delegate
   **`architect`** for the spec and approval brief before `implementer`.

## IDE hints (optional)

These are invocation hints only. Governance truth is this file and `.ai/personas/`.

| IDE | Delegate persona |
| --- | --- |
| Cursor | Task tool with `subagent_type` matching persona name |
| Codex | Spawn named custom agent from `.codex/agents/` |
| Claude Code | Invoke subagent from `.claude/agents/` |
