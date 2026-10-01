# Software Domain Profile

## Scope

This profile governs source code under `aidlc/`. Root `.ai/` files are template payload data and are
governed by `docs/blueprints/template-payload.md`.

## Layers

| Layer | Paths | Responsibility |
| --- | --- | --- |
| Interface | `aidlc/cmd/aidlc`, `aidlc/internal/cli` | Parse user input, present help and errors, map command exits. |
| Application | `aidlc/internal/commands`, `aidlc/internal/generator`, `aidlc/internal/skills`, `aidlc/internal/sync` | Coordinate init/update flows, installed-skill lifecycle, generation, planning, and filesystem decisions. |
| Contracts | `aidlc/internal/contract`, `aidlc/internal/payload` | Shared command DTOs, manifests, IDE enums, payload allowlist schema, and path policy. |
| Infrastructure | `aidlc/internal/source`, `aidlc/internal/install`, `aidlc/internal/search`, `aidlc/scripts`, release config | Fetch upstream sources, run bounded local exact search adapters, validate release artifacts, and integrate with distribution systems. |
| Test Support | `aidlc/internal/testutil`, `aidlc/testdata` | Fixtures and helpers for Go tests only. |

## Dependency Direction

- Interface may depend on application and contracts.
- Composition-root exception: `aidlc/internal/cli` may import infrastructure implementations only
  to bind concrete adapters to application/contract interfaces at command construction time. This
  exception is limited to dependency assembly; CLI code must not contain business logic,
  persistence logic, scanning/query behavior, or direct infrastructure operations beyond
  constructing and passing dependencies.
- Application may depend on contracts, payload policy, and infrastructure interfaces.
- Contracts and payload policy must not depend on interface, application orchestration, or
  infrastructure packages.
- Infrastructure may depend on contracts but must not call interface packages.
- Test support may depend on production packages only from tests.

## Boundary Rules

- Normal `aidlc init` and `aidlc update` flows use native Go filesystem, archive, checksum, HTTP,
  and rendering logic. They must not shell out to Bash, Make, rsync, or git.
- Public payload membership must be read from `.ai/template-manifest.yaml`; code must not infer
  broad directory membership such as `docs/**`.
- The root repository must not gain root-level Go manifests.
- Cross-cutting CLI contract changes require blueprint updates and, when architectural, an ADR.
- Bounded local exact search may shell out to `rg` only through `aidlc/internal/search` and only
  when wired by the CLI composition root into `repomap/model` interfaces. Application packages must
  not import or invoke that adapter directly.

## Test Gates

- `make aidlc-test` for the isolated Go module.
- `make aidlc-release-check` for release configuration and installer validation.
- `make validate-governance` for manifest and architecture invariants.
- `make test` for the aggregate template gate.
