---
# Suggested path: docs/tasks/<task-id>.task.json
# Task records are local evidence and are not broad public payload.
schema_version: 1
---

```json
{
  "schema_version": 1,
  "id": "task-<epoch>-<slug>",
  "spec": "docs/spec/<epoch>-<slug>.md",
  "risk": "high",
  "scope_root": ".",
  "base_revision": "HEAD~1",
  "owned_files": [],
  "content_digest": "sha256:<dirty-tree-owned-files-digest>",
  "acceptance": [],
  "decisions": [
    {
      "summary": "<decision made>",
      "sources": ["docs/adr/<epoch>-<slug>.md"],
      "fresh_at": "<YYYY-MM-DD>"
    }
  ],
  "checks": [
    {
      "command": "make aidlc-test",
      "status": "passed",
      "revision": "HEAD",
      "content_digest": "sha256:<dirty-tree-owned-files-digest>"
    }
  ],
  "review": {
    "kind": "independent",
    "evidence": "<review summary or URL>",
    "revision": "HEAD",
    "content_digest": "sha256:<dirty-tree-owned-files-digest>"
  },
  "open_next_steps": []
}
```

Task records capture evidence the local CLI can inspect: schema version, scope, owned files,
dirty-tree content digest, command checks, and review references. They do not prove human approval,
host write isolation, model identity, actual token usage, or production cost savings.
