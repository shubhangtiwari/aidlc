# `.ai/` — Portable AI Guidance

This directory contains project-agnostic operating guidance for AI assistants. Project facts live in
`docs/ARCHITECTURE.md`, `docs/architecture/`, `docs/blueprints/`, specs, ADRs, and the root
`Makefile`. Do not edit generated IDE files by hand; regenerate them with `aidlc init <ide>` or the
repository wrapper.

## Directory Contract

- `models.defaults.toml` — optional model and effort defaults. Empty or absent fields mean inherit
  the host default.
- `personas/` — role authority: architect plans, implementer edits, reviewer checks finished
  high-risk work.
- `skills/` — reusable playbooks. Bundled skills are payload-owned; `.ai/skills/installed/**` is
  local consumer-owned state and must be preserved by init/update, including force updates.
- `templates/` — spec, task-record, and approval-brief templates.
- `repo-map-protocol.md` — navigation rules for map, exact, and conventional discovery.
- `references/` and `scripts/` — public reference material and maintenance helpers listed in the
  template manifest.

<!-- INIT:BEGIN -->
<!-- Everything below this marker is copied verbatim into per-IDE entrypoints by aidlc init. -->

## Main Agent Contract

Use the user's request as authorization for low-risk reversible work. Ask only when an answer would
change scope, route, risk, or approval. For governed work, state the intended route before the first
state-changing action and keep evidence grounded in live files.

An **AIDLC scope root** is the invocation root, or a nested directory below it, that contains both
`.ai/README.md` and `docs/spec/README.md`. For each affected path, walk upward to the invocation
root and use the nearest scope root. A parent scope must not claim files owned by a nested scope.
Medium, large, or still-uncertain work spanning multiple scopes needs one approved spec per scope.

## Discovery

Follow `.ai/repo-map-protocol.md`. Use map queries for structural discovery when available, and use
bounded exact/path search for concrete filenames, symbols, or literals. Query results are hints; read
the real source, tests, specs, ADRs, and blueprints before editing or reviewing.

## Routing

Run skill `classify-change` before governed implementation. It produces a Triage record with one of
these `next` routes:

- `direct-execute` — trivial, reversible, no contract/state/topology/integration impact. The primary
  agent may edit directly after stating intent.
- `direct-intent` — small low-risk work where a short inline intent improves visibility. Proceed
  after posting intent unless new evidence raises risk.
- `bounded-investigation` — read-only investigation can resolve tier or scope. Reclassify after the
  bounded read.
- `draft-spec` — medium, large, high-risk, or still-uncertain work. Delegate `architect`; wait for
  explicit user approval; flip only `status: draft` to `status: approved`; then delegate
  `implementer`; run `reviewer` before reporting complete or merging.
- `ask-user` — one or two focused questions whose answers materially change route or scope.

Tier by semantic risk, not raw file count. Public behavior is a signal to size by blast radius, not
an automatic spec trigger. Schemas, module contracts, owned state, integration boundaries, workflow
topology, graph topology, coordination cost, rollback risk, and user impact drive risk. Multi-file
mechanical work may be small; a one-file contract or state change may need a spec.

## Persona Chain

Use personas by reference instead of copying their bodies into root prompts:

- `architect` (`.ai/personas/architect.md`) writes scope-local specs and approval briefs for
  `draft-spec` work.
- `implementer` (`.ai/personas/implementer.md`) edits within an approved spec or an allowed
  low-risk route, runs the required Make gates, and performs blueprint sanity.
- `reviewer` (`.ai/personas/reviewer.md`) is mandatory for approved-spec work and optional only when
  the user explicitly asks for review of low-risk work.

For specs with work packages, use skill `orchestrate-spec`: execute waves in order, keep one writer
per path per active wave, freeze shared contracts in wave 0, reserve the final integration wave for
wire-up and blueprint sync, and review the full diff before completion.

## Hard Rules

1. Execute repository gates through `Makefile` targets only.
2. Follow the active domain profile in `docs/architecture/<domain>.md`.
3. Cross-cutting architectural decisions require an ADR before implementation.
4. Medium, large, high-risk, or still-uncertain governed work requires an approved spec before code.
5. Approved-spec work is not complete until independent reviewer checks the diff against the spec.
6. Preserve consumer-owned installed skill trees and local task records; never add broad payload
   copies for `.ai/skills/installed/**` or `docs/tasks/**`.
