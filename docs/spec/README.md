# Specs

A spec is a forward-looking blueprint for a single feature, fix, or refactor. One spec maps to one
PR or MR. Medium and large changes are approved before implementation begins.

## Human vs agent audiences

Specs under a scope-local `docs/spec/` directory are optimized for **agents**: YAML frontmatter,
work packages, dependency graphs, and precise file lists. Humans should **not** need to read the full
spec to approve work.

At draft stage, the **architect** posts an **approval brief** in chat (see
`.ai/templates/approval-brief.md`): medium detail covering why, what changes, and which files. After
approval, implementer and reviewer use the **spec file only** as the source of truth.

## Scope roots

The **invocation root** is the AIDLC root where the current agent session or prompt started. An
**AIDLC scope root** is the invocation root or a directory below it that contains both
`.ai/README.md` and `docs/spec/README.md`. Generated IDE files such as `AGENTS.md`, `.codex/**`,
`.cursor/**`, or `CLAUDE.md` are not scope markers.

Resolve the owning scope for each affected file by walking from the file's directory upward to the
invocation root and selecting the nearest AIDLC scope root. If no nested initialized scope is found,
the invocation root owns the path. Paths outside the invocation root are outside this workflow
unless the user explicitly changes the invocation scope or starts a separate governed session there.

A medium or large request that maps affected paths to more than one scope must produce one draft
spec per resolved scope. Each scoped spec may include only files owned by that scope. A parent scope
spec must not claim files below a nested initialized AIDLC scope. One approval brief may summarize
multiple scoped draft specs for a single user request, but implementation and review treat each
approved spec file as a separate governing artifact.

## When to write a spec

- **Trivial** changes: typo, rename, comment, or obvious one-line fix with no behavior, contract,
  state, integration, or topology change. No spec.
- **Small** changes: low-risk localized fix, single test, dependency bump, bounded helper use, or
  mechanical multi-file edit. No spec file when the change does not alter public behavior, schemas,
  module contracts, owned state, integration boundaries, workflow topology, or graph topology; the
  main agent states intent inline, the user confirms, then **`implementer`** applies the change.
- **Medium / Large** changes: public behavior changes, schemas or contracts, target state semantics,
  workflow or graph topology, integration boundaries, durable state, or coordination across modules
  that needs a plan. A full spec file is required and approved before code is written. A one-file
  change can still be medium or large when it carries these risks.

File count, line count, and directory spread are triage signals to inspect for coordination cost,
not automatic tier gates. Multi-file work can remain small when it is mechanical or tightly bounded
and leaves contracts, state, integrations, and topology unchanged.

No spec does **not** skip governance: trivial and small work still delegate **`implementer`** for
governed paths. The implementer runs a **blueprint sanity** check and updates `docs/blueprints/` when
the change affects contracts, owned state, integrations, topology, or read-only paths.

## Frontmatter fields

| Field | Required | Meaning |
| --- | --- | --- |
| `id` | yes | `spec-<epoch>-<slug>` (matches filename) |
| `status` | yes | `draft` → `approved` → `implemented` → `stale` |
| `owner` | yes | Spec author: `git config user.name`, else OS login (`whoami`) |
| `tier` | yes | trivial / small / medium / large |
| `domain` | yes | `software`, `data-engineering`, `data-science`, or `mixed` |
| `work_packages` | medium/large | Delegable units with waves and dependencies |

Optional sidecar: `<scope-root>/docs/spec/<epoch>-<slug>.work-packages.yaml` when YAML frontmatter
is too large.

## Work packages

Medium and large specs should define `work_packages` in frontmatter plus markdown sections for the
dependency tree and parallel execution plan. See `.ai/templates/spec.md`.

- **Wave 0** freezes shared contracts and shared pure helpers.
- **One writer per path** per active wave.
- Apply skill `orchestrate-spec` for parallel implementer delegation.

## Lifecycle

```text
draft → approved → implemented → stale
```

1. Draft from `.ai/templates/spec.md`.
2. Fill all required sections. `Open questions` must end empty.
3. Architect posts approval brief in chat; user approves and spec `status` becomes `approved`.
4. Implementer applies the spec (per work package when defined), including blueprint deltas. For
   trivial/small work without a spec, implementer still runs blueprint sanity in the same PR.
5. Open a PR or MR; add an entry to the owning scope's `docs/spec/.in-flight.yaml` (manual or
   automation).
6. After merge, run `make finalize-spec` or rely on CI — removes the in-flight entry and sets
   `status: implemented`.

## Naming

Use `<scope-root>/docs/spec/<epoch>-<slug>.md`:

| Token | Meaning | Example |
| --- | --- | --- |
| **epoch** | Unix time in seconds at creation (`date +%s`) | `1748092800` |
| **slug** | Short kebab-case feature name | `add-oauth-login` |

Set the epoch once when creating the file. Do not reuse another branch’s epoch. The slug is the
readable name only (2–6 words, kebab-case). Existing single-scope behavior is unchanged because the
invocation root is also a scope root.

Frontmatter `id:` must be `spec-<epoch>-<slug>` (e.g. `spec-1748092800-add-oauth-login`).

## Owner

Set `owner:` when drafting — do not leave a placeholder. Resolution order:

1. `git config user.name` (trimmed)
2. If empty, `whoami` (OS login name)

Override in frontmatter before approval if needed.

## Artifact Taxonomy

| Artifact | Level | Path |
| --- | --- | --- |
| PRD | Product motivation | `docs/PRD_*.md` |
| ADR | Architecture decision | `docs/adr/<epoch>-*.md` |
| Blueprint | Module-level living design | `docs/blueprints/<module>.md` |
| Spec | Feature change | `<scope-root>/docs/spec/<epoch>-*.md` |

Specs may span modules. They update blueprints; they do not replace ADRs.

## In-flight Tracker

`<scope-root>/docs/spec/.in-flight.yaml` lists specs whose PRs or MRs are open but not merged.

```yaml
in-flight:
  - spec: docs/spec/1748092800-add-oauth-login.md
    branch: feature/add-oauth-login
```

- **Add** an entry when a spec-backed PR is opened (manual or future automation).
- **Remove** on merge via `make finalize-spec` or a post-merge CI job (see `init-architecture`).

Projects may maintain the file manually or wire CI to call `make finalize-spec` with `--branch` set
to the merged PR head ref.
