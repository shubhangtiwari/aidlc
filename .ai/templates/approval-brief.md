# Approval Brief (chat-only)

Human-facing summary for spec approval. **Not** written to the repository. The architect posts
this in chat after drafting `<scope-root>/docs/spec/<epoch>-<slug>.md` and **stops** for approval.

Target length: **~250–500 words** (medium detail — not a one-liner, not a full spec).

## When to use

| Tier | Approval brief |
| --- | --- |
| Trivial / small | Main agent: shorter inline summary in chat; no spec; then `implementer` |
| Medium / large | Full brief below after scoped spec file(s) are saved |

## Required blocks

Use these headings in chat. Synthesize from the spec — **do not** paste spec frontmatter, mermaid,
tables, or the full spec body.

### Feature and why

2–4 sentences in plain language: what triggered this, what capability or fix we are adding, and
what will be true when it ships.

### What will change

Bullets describing behavior and contract changes. Prefer user-visible outcomes over layer jargon.
Include integration or data impacts when relevant.

### Files and areas

Grouped list of concrete paths (from spec `Affected files` and work package `files`). For each group,
note **new** vs **modified**. Group by scope root first when one user request produced multiple
scoped draft specs; otherwise group by work package, layer, or module — whichever is clearest.

### How we will build it

Short execution overview. For medium/large specs, summarize waves (for example: "Wave 0: shared
models; Wave 1: two parallel integration WPs; Wave 2: service + API; final: integration tests and
blueprint updates"). Skip this subsection for small changes.

### Out of scope

1–3 bullets from spec non-goals — only items that prevent misunderstanding at approval time.

### Approval ask

- Spec path: `<scope-root>/docs/spec/<epoch>-<slug>.md`
- Tier: `<tier>`
- Status: `draft` (flip to `approved` after you confirm)
- Explicit question: approve to proceed with implementation?

For one user request that spans multiple AIDLC scopes, list every scoped spec path and ask whether
the user approves all listed drafts. A single approval brief may cover all of them, but each scoped
spec remains a separate governing artifact for implementation and review.

## Do not include in chat

- Full YAML frontmatter or `work_packages` block
- Mermaid dependency diagrams (summarize in prose instead)
- Duplicate spec section tables
- Implementation notes template or reviewer checklist
- Raw copy-paste of the spec file

## Token discipline

- Write the spec **once** to disk.
- Compose the brief **from** the spec (synthesis), not by copying it.
- If the user asks to "explain the plan," expand in brief style — still do not dump the spec file.
- If the user wants a spec amendment, update the file and post an updated brief; do not resend the
  unchanged full spec.

## After approval

User confirms (or flips `status: approved` in each spec frontmatter). Implementer reads the
**spec file only** — not the chat history.
