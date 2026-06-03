---
# epoch: date +%s at file creation; slug: kebab-case feature name (2–6 words)
# owner: git config user.name (trimmed), else whoami — do not invent
# path: <scope-root>/docs/spec/<epoch>-<slug>.md; paths are relative to that AIDLC scope root
id: spec-<epoch>-<slug>
status: draft
owner: <from git user.name or whoami>
tier: medium
domain: software
created: <YYYY-MM-DD>
branch:
implementing-pr:
blueprint-deltas:
  - module: <module-name>
    sections: []
related-adrs: []
related-prds: []
changelog-entry: |-
  <one-line summary; lands in changelog or release notes on merge>
work_packages:
  - id: WP-M0
    title: Shared models and contracts
    domain: software
    layer: models
    depends_on: []
    wave: 0
    files: []
    gates:
      - make test-unit
    done_when:
      - Types compile; unit tests for wave 0 pass
---

# <Spec title>

## Context

Why now. What triggered this. One paragraph.

## Goal

One behavioral sentence describing what will be true when this is shipped.

## Non-goals

Explicit list of what this spec is not solving.

- ...

## Constraints

Layer rules, performance, compatibility, governance. Anything that narrows the design space.
Include scope ownership when relevant: this spec may own only files in its resolved AIDLC scope and
must not claim files below a nested initialized scope.

- ...

## Affected files

Concrete paths the implementation will touch, relative to this spec's AIDLC scope root unless noted.
Avoid broad directory globs.

- `path/to/file`
- ...

## Work packages

| ID | Title | Domain | Layer | Wave | Depends on | Parallel? |
| --- | --- | --- | --- | --- | --- | --- |
| WP-M0 | ... | software | models | 0 | — | alone |
| ... | ... | ... | ... | ... | ... | ... |

## Dependency tree

```mermaid
flowchart TD
  WP-M0 --> WP-S1
  WP-M0 --> WP-I1
```

## Parallel execution plan

| Wave | Work packages | Max parallel implementers |
| --- | --- | --- |
| 0 | WP-M0 | 1 |
| 1 | WP-I1, WP-U1 | 2 |
| 2 | WP-INT | 1 |

## Blueprint deltas

Which blueprint sections change and how. Required for medium/large: list concrete edits, or write
**None** with a one-line reason when no blueprint-owned concern changes.

- **`docs/blueprints/<module>.md` § <section>**: <what changes>

## Test plan

Named scenarios mapped to test gates.

- `tests/unit/<...>/test_<...>` — <scenario>
- `tests/integration/<...>/test_<...>` — <scenario>

## Open questions

Must be empty before status flips from `draft` to `approved`. If a question cannot be resolved, it
becomes a constraint or a non-goal.

- None.

## Implementation notes

Filled during execution. Amendments and discoveries go here, each with a short justification and
the date.

- ...
