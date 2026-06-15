# Repo-Map Agent Protocol

This repository may provide an agent navigation map under `docs/map/`. The map is the first
discovery mechanism for finding likely files; it is not a replacement for reading source, tests,
specs, or blueprints. It is available to every agent role, including the main session, architect,
implementer, and reviewer.

## Operating Rules

1. Before broad source exploration, consult the repo map with `make ai-query AI_QUERY="<task terms>"`
   or `aidlc query "<task terms>"`. Do this before broad conventional discovery tools such as
   `rg --files`, `find`, tree listings, or speculative file reads.
2. Treat query results as navigation hints only. Open and read the real files before making any
   edit, review finding, or architectural claim.
3. If the map is missing or stale and the user has not requested a read-only session, the main
   session must regenerate it with `make ai-map`, then verify with `make ai-map-check`, then query
   again.
4. Fall back to conventional exploration only when the map is missing and cannot be generated,
   map commands fail, query results do not include useful paths, or the needed information is not
   represented in the map. State the fallback reason.
5. Do not edit generated files in `docs/map/` by hand. Regenerate them from the repository state.
6. Do not persist repo-specific map state under `.ai/`. The `.ai/` tree is static delivery and
   operating guidance; `docs/map/` is the per-repository navigation aid state.

## Makefile Integration

Add this include to the target repository root `Makefile`:

```make
-include .ai/Makefile.inc
```

Then use:

```sh
make ai-map
make ai-map-check
make ai-query AI_QUERY="authentication command routing"
```

`docs/map/*.jsonl` and `docs/map/index.json` are the canonical committed map artifacts.
`docs/map/repo-map.sqlite` is a derived local cache and is ignored by git.
