---
name: classify-change
description: Triage governed implementation into direct-execute, direct-intent, bounded-investigation, draft-spec, or ask-user.
---

# Skill: Classify Change (triage)

Portable playbook for **tier triage** on governed work. Authority for routing after triage lives in
`.ai/README.md`. This skill grants no extra write permission beyond the selected route.

## When to run

- **Main session** is about to touch **governed paths** (`src/`, `tests/`, `docs/blueprints/`,
  `docs/spec/`, `docs/adr/`, `docs/ARCHITECTURE.md`, `docs/architecture/`) or needs a spec —
  **before** edits, spec drafting, or implementer delegation.
- **Skip** for configuration-only work (`.ai/`, IDE config, `Makefile` when not a contract),
  exploration, or Q&A with no implementation.

## Who runs it

The **main session** follows this playbook directly (read `docs/spec/README.md` and relevant
blueprints; post the Triage record in chat). **Do not** delegate triage to `architect`.

Triage is **read-only** with respect to governed trees: no spec file, no ADR, no blueprint edits, no
`src/` or `tests/` changes.

When triage yields **`next: draft-spec`**, the main session **must** delegate **`architect`** for
planning (spec + approval brief) — do not draft the spec in the main session.

## Prerequisites (main session)

1. Restate the user's problem statement in one sentence.
2. Read `docs/spec/README.md` (tier definitions).
3. Skim relevant `docs/blueprints/` modules if the request names an area (router, API, graph,
   integrations).
4. If architecture is initialized, skim `docs/ARCHITECTURE.md` and the domain profile under
   `docs/architecture/` when layer boundaries matter.
5. Resolve the AIDLC owning scope for named or likely affected paths: start at each path's directory,
   walk upward to the invocation root, and select the nearest directory containing both
   `.ai/README.md` and `docs/spec/README.md`. If none is found below the invocation root, use the
   invocation root.

## Classification rules

Apply `docs/spec/README.md` literally. Tier by semantic and coordination risk. File count, line
count, and directory spread are evidence to inspect; they are not automatic gates by themselves.
In particular:

| Tier | Typical signals |
| --- | --- |
| **Trivial** | Typo, rename, comment, or obvious one-line fix; reversible; no behavior, contract, state, integration, or topology change |
| **Small** | Low-risk localized fix, single test, dependency bump, bounded helper use, or mechanical multi-file edit; reversible enough to inspect; no public behavior/schema change, module contract change, owned-state change, integration boundary change, or topology change |
| **Medium** | Externally significant public behavior change, schema/contract change, target-state semantics change, workflow or graph topology change, integration boundary change, or coordination across modules that needs a plan |
| **Large** | Cross-cutting architectural change, many coordinated modules, new external integration, durable state migration, ADR-level decision, or high rollback/user-impact risk |
| **Uncertain** | Scope or tier unclear after reading docs; would change path if wrong |

Multi-file work can remain **small** when each touched file is part of the same bounded, low-risk
change and the blueprint-owned concerns above do not move. A one-file change can still be
**medium** or **large** when it changes public behavior, schemas, contracts, target state,
integrations, or workflow topology. When **any** medium trigger might apply, prefer **medium** or
**uncertain** over small. Do not down-tier to avoid a spec.

## Method

1. List concrete triggers you checked (files/areas, file/line count signals, blueprint concerns,
   public behavior blast radius, state, graph/topology, integrations, and coordination cost).
2. Assign **one** tier: `trivial` | `small` | `medium` | `large` | `uncertain`.
3. Set **`next`**:
   - `direct-execute` — trivial, reversible, and obvious → primary agent states intent and edits.
   - `direct-intent` — small low-risk → primary agent posts concise intent and proceeds unless new
     evidence raises risk; delegate `implementer` when local policy or the host requires it.
   - `bounded-investigation` — uncertainty can be resolved by a short read-only pass; re-run triage
     after the bounded read.
   - `draft-spec` — medium, large, high-risk, or still-uncertain after investigation → delegate
     `architect` for spec + approval brief.
   - `ask-user` — one or two focused questions whose answers would change tier or `next`.
4. Write a **rationale** (2–4 sentences), plain language.
5. Include the resolved owning scope(s) in `suggested_scope` when path evidence is available. If a
   medium/large/uncertain request spans multiple scopes, note that architect must draft one spec per
   scope.
6. Post the **Triage record** (format below) in chat, then **route immediately** per `next` (do not
   skip architect delegation when `next` is `draft-spec`).

## Triage record (required output)

Use exact field names:

```markdown
## Triage record

- **tier:** trivial | small | medium | large | uncertain
- **next:** direct-execute | direct-intent | bounded-investigation | draft-spec | ask-user
- **rationale:** …
- **triggers_checked:** …
- **suggested_scope:** files or modules (optional, best effort)
```

## After triage (main session)

| `next` | Main session action |
| --- | --- |
| `direct-execute` | State intent and edit directly. Run the relevant Make gate and blueprint sanity. No reviewer unless the user asks. |
| `direct-intent` | Post short inline intent, proceed within stated bounds, and use `implementer` if required by the active host/persona workflow. No reviewer unless the user asks. |
| `bounded-investigation` | Perform the stated read-only investigation, then re-run this skill with evidence. |
| `draft-spec` | **Delegate `architect`** with the Triage record and problem statement. Architect writes spec + approval brief. **Do not** call `implementer` until spec is approved. After explicit user approval, the main session may perform only the `status: draft` to `status: approved` frontmatter flip before delegating `implementer`. |
| `ask-user` | Ask the questions; re-run this skill after answers. |

Do not override `tier` or `next` without user consent in chat.

## Out of scope (this skill)

- Writing or updating `<scope-root>/docs/spec/*.md` in the main session, except the status-only
  `draft` to `approved` frontmatter flip after explicit user approval. Architect owns planning
  content and amendments.
- Running `implementer` or `reviewer` before triage and routing complete when those steps are
  required by the route.
- Tiering for non-governed paths (main session may proceed without this skill).

## Outcome

- A **Triage record** exists in chat before any governed implementation step.
- Trivial/small → direct route with Make gate and blueprint sanity.
- Medium/large/high-risk/still-uncertain → **architect** (automatic) → spec → main approval status
  flip → implementer → reviewer.
