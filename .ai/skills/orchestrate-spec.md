---
name: orchestrate-spec
description: Executes an approved spec by work-package waves, delegating one implementer per work package per wave. Use when a spec has work_packages and parallel execution is desired after status is approved.
---

# Skill: Orchestrate Spec Execution

Portable playbook for parallel implementer delegation. Authority comes from `.ai/README.md` and
the governing spec.

## Prerequisites

- Spec at `<scope-root>/docs/spec/<epoch>-<slug>.md` with `status: approved` in frontmatter.
- Spec defines `work_packages` with `wave`, `depends_on`, `files`, `gates`, and `done_when`.
- Single branch for all work packages unless the user directs otherwise.
- For a user request with multiple scoped specs, run this playbook once per approved scoped spec.

## Method

1. **Load spec.** Parse frontmatter `work_packages` and the **Parallel execution plan** section.
2. **Sort waves** ascending (0, 1, 2, …).
3. **For each wave:**
   - Confirm all `depends_on` WPs from prior waves are done.
   - Delegate **one implementer per WP** in this wave with a brief containing:
     - Spec path and WP id
     - Allowed file paths only
     - Domain profile: `<scope-root>/docs/architecture/<domain>.md`
     - Gates to run (`make` targets)
     - `done_when` criteria
     - Owning scope root; paths are relative to that scope unless the spec says otherwise
   - Cap concurrent implementers at 3–6; queue excess WPs in the same wave.
   - Wait for all WPs in the wave to report done or escalated before the next wave.
4. **After final wave (mandatory):** main session delegates `reviewer` on the full branch diff vs
   spec. Do not report implementation complete, open a PR, or close the governed workflow until
   `reviewer` returns. Skipping this step violates Hard Rule 6 in `.ai/README.md`.
5. **Escalation:** material discovery → stop WP, note in spec `Implementation notes`, architect
   amends spec before continuing.

## Wave 0 reminder

Wave 0 freezes shared contracts and genuinely shared pure helpers. Later WPs must import wave-0
symbols; they must not duplicate or rename them without architect amendment.

## WP-INT

If the spec declares a wire-up work package (`WP-INT` or similar), run it after all feature WPs
complete. It owns cross-cutting edits: integration tests, blueprint sync, shared barrels.

## Outcome

- All WPs marked done with gates passed.
- Spec `Implementation notes` updated for any in-scope discoveries.
- `reviewer` has run on the full diff vs spec (required before merge or “done” for medium/large).
- Then post-merge finalize via `make finalize-spec`.
