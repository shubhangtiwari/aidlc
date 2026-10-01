# Repo-Map Agent Protocol

This repository may provide an agent navigation map under `docs/map/`. The map is a structural
navigation aid, not proof. It is useful for unknown-location work, module discovery, and relationship
questions; exact filenames, symbols, literals, and obvious paths may also be checked with bounded
local exact search such as `rg` before or alongside map queries.

## Operating Rules

1. For structural discovery, consult the map with `make ai-query AI_QUERY="<task terms>"` or
   `aidlc query "<task terms>"` before broad tree listings or speculative reads.
2. For concrete clues, use bounded exact/path search directly when it is the shortest honest route.
   Keep searches local, fixed-string when possible, and scoped to likely paths. Record when exact
   search or the map omitted relevant regions.
3. Treat all query results as hints. Open and read live source, tests, specs, ADRs, and blueprints
   before editing, reviewing, or making architectural claims.
4. If the map is missing or stale and the task needs structural discovery, the main session may
   regenerate it with `make ai-map` and verify with read-only `make ai-map-check`, unless the user
   requested a read-only session. On a first non-interactive run, pass `AI_MAP_INCLUDE`.
5. Fall back to conventional exploration when the map is unavailable, stale, insufficient, fails, or
   does not model the evidence needed. State the fallback reason when it affects confidence.
6. Do not edit generated files in `docs/map/` by hand. Regenerate them from repository state.
7. Do not persist repo-specific map state under `.ai/`. Static guidance lives in `.ai/`;
   per-repository navigation state lives under `docs/map/` and the root lock.

## Makefile Integration

Add this include to the target repository root `Makefile`:

```make
-include .ai/Makefile.inc
```

Then use:

```sh
make ai-map
make ai-map AI_MAP_INCLUDE=".ai,aidlc,docs"
make ai-map-check
make ai-query AI_QUERY="authentication command routing"
make ai-map AIDLC_BIN=/path/to/local/aidlc AI_MAP_INCLUDE=".ai,aidlc,docs"
```

The default `make ai-map` delegates whitelist behavior to `aidlc map`: it reuses the saved
`aidlc.lock.json` include list, or prompts on the first interactive run. In CI or other
non-interactive first-run environments, pass `AI_MAP_INCLUDE` with a comma-separated list of
slash-relative folders to make the include list explicit and save it before building. If no saved
include list exists and no explicit include list is supplied, non-interactive `make ai-map` fails
with guidance instead of guessing.

`make ai-map-check` runs `aidlc map --check`. It is read-only: it uses the saved include list and
does not prompt or write `aidlc.lock.json`.

`AIDLC_BIN` defaults to `aidlc` and may be set to a local development binary when validating
repository map changes before an updated CLI is installed.

`docs/map/*.jsonl` and `docs/map/index.json` are the canonical committed map artifacts.
`docs/map/repo-map.sqlite` is a derived local cache and is ignored by git.
