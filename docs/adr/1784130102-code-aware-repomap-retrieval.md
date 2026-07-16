# ADR-1784130102: Use Code-Aware Lexical Repo-Map Retrieval

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Shubhang Tiwari

## Context

Repo-map query currently uses deterministic JSONL shards and a derived pure-Go SQLite FTS5 cache.
That design is fast, local, reviewable, and compatible with ADR-1781478129's CGO-disabled
six-target release constraint. The previous draft explored vector and embedding retrieval, but the
final direction removes vectors entirely. The user wants better repository search without
embeddings, vector tables, local model runtimes, API calls, network services, cloud transfer, or
new runtime dependencies.

The calling LLM can still help by generating a structured search plan: question, lexical terms,
phrases, symbol candidates, path hints, file globs, and limits. AIDLC should execute that plan
deterministically against local map artifacts, then return compact ranked snippets. The model then
reads the real files behind the bounded result set before making claims or edits.

## Decision

Use code-aware lexical hybrid retrieval:

- Do not add embeddings, vector search, model runtimes, API calls, network services, hosted search,
  or new runtime dependencies.
- Keep canonical JSONL shards as the committed map source of truth and keep
  `docs/map/repo-map.sqlite` as a derived ignored cache.
- Add a public `SearchPlanV1` JSON input contract for `aidlc query` because the calling LLM needs a
  stable way to express planned retrieval. Preserve simple raw-text `aidlc query <terms>` by
  compiling raw text into a minimal `SearchPlanV1` internally.
- Execute retrieval through deterministic local channels: SQLite FTS over existing text records,
  JSONL fallback scan, path/name search, recursive glob matching, identifier expansion,
  symbol/declaration records, import/test relationship expansion, and bounded snippet extraction.
  Cache-backed structured plans are hybrid execution: SQLite FTS handles text channels while
  deterministic JSONL channels handle paths, recursive globs, symbols, source chunks, imports, and
  test links before rank fusion. Structured plan mode must not become fallback-only when the cache
  is present.
- Add a canonical `symbols.jsonl` shard for extracted declarations and symbols. Extraction is
  language-aware but conservative, regex/token based, and local; it is not an AST or language-server
  integration.
- Fuse ranked candidate lists with deterministic weighted reciprocal-rank fusion, de-duplicate by
  path, and return compact snippets with line ranges and source shard evidence.
- Keep `modernc.org/sqlite` and FTS5 as the only search cache dependency. `go.mod` and `go.sum`
  should not gain search/model/vector dependencies.

## Consequences

- AIDLC search becomes more code-aware without privacy, cost, setup, or binary-distribution costs
  from embedding providers.
- Structured plan mode becomes a public CLI contract and must be versioned, validated, documented,
  and tested for malformed input.
- Raw text query behavior remains supported and backward compatible, but advanced callers should
  prefer the JSON search plan for better recall and lower token waste.
- Map generation adds symbol extraction and one new committed shard, but must remain fast and
  deterministic.
- Lexical retrieval will not understand true semantic similarity by itself. Quality depends on the
  calling LLM producing good terms, symbols, and path hints, plus AIDLC's identifier expansion and
  relationship traversal.

## Alternatives Considered

| Alternative | Why rejected |
| --- | --- |
| Vector or embedding search | Explicitly rejected by final requirement. It adds vectors/model/runtime concerns and is no longer in scope. |
| Local embedding runtime such as `llama-server` | Still an embedding/model runtime. Rejected by final requirement to remove vector search entirely. |
| Hosted search or OpenAI embeddings | Rejected: introduces API calls, API keys, subscription/cloud concerns, and data transfer. |
| Let the model search the repository directly | Bypasses repo-map-first governance, wastes context, and weakens evidence discipline. The model should plan retrieval, not roam the tree. |
| Hide the structured plan as an internal-only implementation detail | Rejected because the calling LLM is the planner. A stable public JSON contract makes the boundary testable and reusable while preserving raw-text CLI compatibility. |
| Add a separate search engine dependency | Rejected to preserve pure-Go static release simplicity and avoid new runtime dependencies. SQLite FTS5 plus JSONL fallback is sufficient for this tier. |
