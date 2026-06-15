---
name: init-architecture
description: Initializes or refreshes repository architecture documentation, module blueprints, and repo-map artifacts by analyzing the codebase and recommending a reference profile. Use when the user asks to initialize the project, initialize architecture, bootstrap governance, set up layer rules, generate blueprints, generate the repo map, or when docs/ARCHITECTURE.md is missing, empty, or still the generic template without project-specific facts.
---

# Skill: Initialize Architecture

Establish **`docs/ARCHITECTURE.md`** as the canonical architecture document for the forked project.
The user must confirm the choice before any write.

## Triggers

- User says: initialize project, init architecture, bootstrap repo, set up layers, define
  architecture, architecture missing.
- `docs/ARCHITECTURE.md` does not exist, is empty, or is clearly the uncustomized template.
- Legacy root `ARCHITECTURE.md` on a fork — offer migration to `docs/ARCHITECTURE.md` only.

## Hard rules

- **Read-only analysis first.** Do not write until the user picks an option.
- **One canonical file:** `docs/ARCHITECTURE.md`. Never recreate root `ARCHITECTURE.md`.
- Do not edit application source unless the user explicitly asks to scaffold directories after
  approval.

## Phase 1 — Discover (read-only)

Collect evidence and record a **Repository signals** summary.

### Manifest and toolchain

Detect: language/runtime manifest at the repo root, data-pipeline manifests, ML/notebook trees,
infrastructure trees, application source layout.

### Layout heuristics

| Signal category | Suggests profile |
| --- | --- |
| Source tree organized by horizontal tiers (entry/services/data) | `tiered-service` |
| Data-pipeline manifests, transform/staging/marts trees | `data-engineering` |
| Notebook trees, training/eval directories, ML registries | `data-science` |
| Multiple strong signals at separate roots | `polyglot-monorepo` |
| Config / docs / governance only, no source | `minimal-tooling` |

Each profile lives in its own folder under `.ai/references/architectures/<id>/` and ships:

- `architecture.md` — layer laws, rules, reviewer checklist.
- `blueprint-template.md` — module/package blueprint scaffold for that profile (absent for
  `minimal-tooling`, which has no source tree).

Read both when picking a profile.

### Existing docs

Read: `docs/ARCHITECTURE.md`, `docs/architecture/*`, ADR titles, blueprints, `.ai/`, `Makefile`.

### Blueprint and repo-map readiness

- Identify owned source modules/packages that need concrete blueprints. Use manifest boundaries,
  package roots, service/app directories, and existing blueprints as evidence. Do not create one
  blueprint per incidental folder.
- Check whether `.ai/Makefile.inc` exists and whether the root `Makefile` already includes it.
  Record whether `make ai-map` can be run after writes.

### Git and CI

- `[ -d .git ]` — required for Phase 5 pipeline bootstrap.
- Detect CI host from repo conventions (workflow directory or pipeline file at the root).
- Default branch: `main` or `master` from `git symbolic-ref` / remote HEAD.
- Existing `make finalize-spec` or workflow named `finalize-spec` — patch only, do not duplicate.

## Phase 2 — Recommend

Present **three options**:

| Option | When |
| --- | --- |
| **A — Adopt reference** `<id>` | Signals match one profile best |
| **B — Keep and refine existing** | Non-empty project-specific architecture already |
| **C — Define new** | Greenfield, unique stack, or blend |

For **A**, rank top 1–3 references with confidence (high / medium / low) and why.

## Onboarding flow to present before writes

Before Phase 3 writes, clearly state the initialization steps that will run so the user sees the
process as an onboarding flow:

1. Confirm the architecture profile choice.
2. Create or refine `docs/ARCHITECTURE.md` and any domain layer docs under `docs/architecture/`.
3. Generate or refresh concrete module blueprints for real source modules.
4. Ensure root Makefile integration by adding `-include .ai/Makefile.inc` exactly once to an
   existing root `Makefile`, or by creating a minimal root `Makefile` when none exists.
5. Generate the repo map with `make ai-map`, then verify freshness with `make ai-map-check`.
6. Wire or confirm post-merge `finalize-spec` CI when the repository is a git checkout.
7. Run the validation gates and record completed, skipped, or failed steps in the Initialization
   record.

## Phase 3 — Compose (after user approval)

Write `docs/ARCHITECTURE.md` with: purpose, `primary_domain`, agent protocol, **real** directory
map, layer laws from chosen reference (paths adapted), testing/Makefile section, artifact taxonomy,
**Initialization record** (date, option, reference id).

Create or update `docs/architecture/<domain>.md` when mixed workloads need domain statutes.

Start from `.ai/references/architecture-template.md` for option **C**.

### Seed `docs/blueprints/module-template.md`

Copy `.ai/references/architectures/<chosen-id>/blueprint-template.md` to
`docs/blueprints/module-template.md` so future module blueprints start from the layer map of the
chosen profile. Rules:

- Overwrite any existing `docs/blueprints/module-template.md` only if the user approved option A
  or C (they explicitly chose a profile). For option B, leave the existing template alone unless
  the user asks to refresh it.
- For `minimal-tooling`, skip — no blueprint template ships with that profile.
- For `polyglot-monorepo`, the copied template covers the per-package shape; record in the
  Initialization record which per-domain templates packages should also reference.

New ADRs use `docs/adr/<epoch>-<slug>.md` with heading `# ADR-<epoch>: <title>` (`date +%s` for
epoch). See `docs/adr/README.md`.

### Generate concrete module blueprints

Create or update concrete `docs/blueprints/<module>.md` files during initialization when source
modules/packages are identifiable from repository evidence.

Rules:

- Use the chosen profile's `blueprint-template.md` as the shape, adapted with real package purpose,
  boundary, layer map, contracts, owned state, integrations, and test gates.
- For `polyglot-monorepo`, create one blueprint per clear package/domain root rather than one
  generic monorepo blueprint. Use the per-domain profile template when a package clearly maps to
  tiered-service, data-engineering, or data-science.
- For `minimal-tooling`, create only blueprints for real governance/tooling modules already present;
  otherwise record `blueprints skipped (no source modules detected)`.
- Do not overwrite substantial existing blueprints without confirming whether to refine them.
- Do not invent modules for empty or speculative paths. If evidence is insufficient, list blueprint
  candidates in the Initialization record instead of creating boilerplate.

Record generated, refreshed, skipped, and candidate blueprints in `docs/ARCHITECTURE.md` under the
**Initialization record**.

## Phase 4 — Post-write

Remind the user to run `make init all` only after any source `.ai/` changes are complete. Module
blueprints should already be generated or explicitly recorded as skipped/candidates.

Record in **Initialization record** whether blueprint generation, repo-map generation, and Phase 5
ran, were skipped, or were already present.

### Wire and generate the repo map

After architecture docs and blueprints are written, generate the navigation map when the repo-map
helper is available:

1. If `.ai/Makefile.inc` exists, ensure the root `Makefile` contains this include exactly once:

   ```make
   -include .ai/Makefile.inc
   ```

   Preserve existing governance targets and unrelated Makefile content.
2. If no root `Makefile` exists, create a minimal root `Makefile` containing `-include
   .ai/Makefile.inc` and detected `install`, `run`, and `test` targets. Use the detection table in
   **Populate Makefile project targets** for recipes. If a target genuinely does not apply, keep the
   target and print a one-line explanation.
3. Run `make ai-map` from the repository root.
4. Run `make ai-map-check` to confirm the generated map is fresh.
5. Record the result in `docs/ARCHITECTURE.md` under **Initialization record**.

Generated map rules:

- `docs/map/*.jsonl` and `docs/map/index.json` are canonical generated repo-specific artifacts.
- `docs/map/repo-map.sqlite` is a derived local cache and must remain ignored.
- Never edit `docs/map/` files by hand; regenerate them with `make ai-map`.
- Never persist repo-specific map state under `.ai/`.
- If `.ai/Makefile.inc` is missing, `aidlc` is unavailable, or `make ai-map` fails for an
  environment reason, do not improvise with ad hoc commands. Record `repo map skipped` with the
  concrete reason and tell the user which prerequisite is missing.

### Populate Makefile project targets

The template ships placeholder `install`, `run`, and `test` targets in the root `Makefile`.
Replace those placeholder bodies with real recipes derived from the detected manifest and
chosen reference profile. Examples of what to emit:

| Stack signal | `install` | `run` | `test` |
| --- | --- | --- | --- |
| `package.json` | `npm ci` (or `pnpm i` / `yarn`) | `npm run dev` (or detected script) | `npm test` |
| `pyproject.toml` (uv) | `uv sync` | `uv run <entry>` | `uv run pytest` |
| `pyproject.toml` (poetry) | `poetry install` | `poetry run <entry>` | `poetry run pytest` |
| `requirements.txt` | `pip install -r requirements.txt` | `python -m <pkg>` | `pytest` |
| `Cargo.toml` | `cargo fetch` | `cargo run` | `cargo test` |
| `go.mod` | `go mod download` | `go run ./...` | `go test ./...` |
| Data-engineering (dbt) | `uv sync && dbt deps` | `dbt run` | `dbt test` |
| Polyglot monorepo | aggregate per-stack commands or delegate to a workspace tool | same | same |

Rules:

- **Preserve the governance targets** (`init`, `update`, `finalize-spec`) and their comments
  exactly — only edit the bodies of `install`, `run`, `test`.
- **Update the help text** for `install`/`run`/`test` to reflect the real commands.
- **Keep the .PHONY line** intact.
- If a target genuinely does not apply (e.g. `run` for a pure library), keep the target but
  print a one-line explanation instead of removing it.
- For polyglot monorepos, prefer a thin dispatcher (`$(MAKE) -C services/api install`, etc.)
  over inlining every sub-stack's commands.

Record the chosen recipes in `docs/ARCHITECTURE.md` under **Execution & Developer Experience**
so the source of truth lives next to the architecture decision.

## Phase 5 — Post-merge finalize CI (git repos only)

Run only after Phase 3 user approval (same write gate). **Skip entirely** if `[ ! -d .git ]` — log
`skipped CI bootstrap (not a git repository)` and do not create or edit pipeline files.

`make finalize-spec` is **post-merge cleanup** only (in-flight removal, `status: implemented`,
commit/push). Do not wire gates, PR creation, or ship semantics into this job.

### Detect host and emit

| CI host signal | Action |
| --- | --- |
| Workflow directory present (e.g. `.github/workflows/`) | Add a finalize-spec workflow next to existing workflows |
| Root pipeline file present (e.g. `.gitlab-ci.yml`) | Append a finalize-spec job/stage to the existing pipeline |
| `.git` only | Default to the host implied by `origin`; otherwise pick a sensible default and note it in the Initialization record |

Reference templates under `.ai/references/ci/` provide concrete starting points for common CI
hosts; copy and adapt the closest match.

### Idempotency

If any workflow/job already runs `make finalize-spec` or is named `finalize-spec`, do not add a
duplicate — note the existing path in `docs/ARCHITECTURE.md`.

### Create or patch

- **No CI file:** copy from the closest reference under `.ai/references/ci/` (adapt default branch
  name; include git `user.name` / `user.email` in CI before commit).
- **Existing CI:** append the finalize job/stage; do not remove unrelated jobs.

### Document

In `docs/ARCHITECTURE.md` **Execution & Developer Experience**, note:

- Post-merge: `make finalize-spec` or CI workflow path.
- Makefile gates (`lint`, `test-unit`, …) belong in a **separate** workflow, not finalize-spec.

### Hard rules (Phase 5)

- Never create CI without `.git`.
- Never trigger finalize on PR open — merged PR / default branch only.
- Never overwrite a full custom pipeline.

## Refusal

- Refuse to overwrite substantial custom `docs/ARCHITECTURE.md` without confirmation.
- Refuse to invent directory paths that do not exist unless the user approves scaffolding.
