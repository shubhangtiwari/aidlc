# Architecture

## Purpose

This repository distributes AIDLC governance assets and, under `aidlc/`, an installable CLI for
initializing and updating those assets in consumer repositories.

## Primary Domain

`software`

## Agent Protocol

Governed changes follow the repository AIDLC workflow:

- Classify governed changes before planning or implementation.
- Use approved specs for medium, large, or uncertain work.
- Apply implementation through the implementer persona.
- Run reviewer on medium, large, or uncertain work before reporting completion.
- Keep module blueprints current when contracts, owned state, integrations, topology, layer maps, or
  gates change.

## Directory Map

| Path | Owner | Notes |
| --- | --- | --- |
| `.ai/` | Root governance payload | Source of truth for portable personas, skills, templates, scripts, and generated IDE guidance. Public payload membership is controlled by `.ai/template-manifest.yaml`. |
| `docs/spec/` | Local planning artifacts | Repository-local specs. These are not public template payload. |
| `docs/adr/` | Local decision records | Repository-local ADRs except starter files explicitly listed in `.ai/template-manifest.yaml`. |
| `docs/blueprints/` | Local module contracts | Repository-local blueprints except starter files explicitly listed in `.ai/template-manifest.yaml`. |
| `docs/ARCHITECTURE.md` | Local architecture map | Repository-local architecture. Not public template payload. |
| `docs/architecture/` | Domain layer rules | Repository-local domain statutes. Not public template payload. |
| `aidlc/` | Installable CLI module | Isolated Go module for CLI contracts, native generation, sync, installation, and release checks. |
| `.github/` | Repository automation | Repository-local CI/release automation. Not public template payload. |

## Layer Rules

The `software` domain profile in `docs/architecture/software.md` defines layer ownership and
dependency direction for the `aidlc/` module. Root governance payload files are data/templates, not
runtime application layers.

## Execution & Developer Experience

The root `Makefile` remains the only supported command entrypoint.

| Target | Purpose |
| --- | --- |
| `make init <ide>` | Generate IDE-native files from `.ai/` for this repository or a consumer repo. |
| `make update` | Existing Bash-compatible update path for `.ai/`. |
| `make aidlc-test` | Run Go tests for the isolated `aidlc/` module. |
| `make aidlc-release-check` | Validate release packaging prerequisites without creating root-level Go manifests. |
| `make validate-governance` | Validate AIDLC governance docs, manifest exclusions, and CLI module boundaries. |
| `make install` | Download Go dependencies from the isolated `aidlc/` module. |
| `make run` | Run local `aidlc` CLI help from the isolated Go module. |
| `make test` | Aggregate governance validation and `aidlc` tests for this template repository. |
| `.github/workflows/finalize-spec.yml` | Post-merge spec cleanup workflow that runs `make finalize-spec` on merged pull requests. |

No root-level Go manifest is allowed. Go commands execute from `aidlc/`.

## Artifact Taxonomy

- Public template payload: explicitly allowlisted paths in `.ai/template-manifest.yaml`.
- Local governance artifacts: specs, ADRs, blueprints, architecture docs, and implementation notes
  for this repository.
- CLI implementation artifacts: `aidlc/**`, installers, release configuration, and CI.
- Generated IDE artifacts: `AGENTS.md`, `CLAUDE.md`, `.codex/**`, `.cursor/**`, and equivalent
  files produced by `make init` or future `aidlc init`.

## Initialization Record

- Date: 2026-06-02
- Source: `docs/spec/1780346463-add-aidlc-cli.md`, WP-M0
- Domain profile: `software`
- Decision: map root governance payload separately from the isolated `aidlc/` Go module, with a
  strict manifest allowlist controlling public payload.
- Refinement: 2026-06-03, Option B — kept the project-specific `software` profile, populated root
  `Makefile` project recipes for the isolated Go module, and wired post-merge spec finalization via
  `.github/workflows/finalize-spec.yml`.
