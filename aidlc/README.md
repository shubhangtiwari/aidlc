# aidlc CLI Reference

`aidlc` is the native CLI for installing, updating, and upgrading the AIDLC governance harness in a
repository. It copies the portable payload, renders IDE-specific files, tracks sync state in
`aidlc.lock.json`, and upgrades the installed CLI binary from GitHub release assets.

This reference documents CLI behavior only. Repository architecture analysis and blueprint creation
are performed by the generated agent harness after `aidlc init <ide>` has created the IDE files.

## Commands

```text
aidlc doctor [flags]
aidlc init <claude|codex|cursor|copilot|windsurf|all> [flags]
aidlc map [flags]
aidlc query [flags] <search terms>
aidlc skill <install|list|remove> [flags]
aidlc update [flags]
aidlc upgrade [flags]
aidlc validate [flags]
aidlc benchmark retrieval [flags]
aidlc version
```

## Setup Role

`aidlc init <ide>` is the first setup step. It installs the AIDLC payload and generates the files
that your assistant reads, such as `AGENTS.md`, `CLAUDE.md`, `.codex/**`, `.claude/**`,
`.cursor/**`, `.github/copilot-instructions.md`, or `.windsurfrules`.

After that, use the generated harness from your IDE and ask the agent to initialise the harness or
initialise architecture. That agent-led initialization reads the repository, suggests a best-fit
architecture profile, writes architecture docs, creates module blueprints, and keeps ADRs available
for cross-cutting decisions. The CLI does not infer those project-specific docs by itself.

## `aidlc init`

```text
aidlc init <claude|codex|cursor|copilot|windsurf|all> [flags]
```

`init` copies only paths listed in `.ai/template-manifest.yaml` from the configured source into the
current directory, then runs native IDE generation.

Successful and partial runs write `aidlc.lock.json` at the target repository root. The lock records
the selected concrete IDEs in `workspace.ides`; `init all` stores every concrete IDE instead of the
literal `all` value.

Existing identical files are skipped. Divergent files are reported as conflicts and are not
overwritten. When conflicts exist, `init` still writes non-conflicting payload files, generates the
requested IDE files, writes an honest partial lock, and exits `1`.

With `--force`, conflicting public payload destinations become explicit overwrite decisions.
Forced runs replace those divergent payload files, record them as clean tracked files in
`aidlc.lock.json`, and exit `0` when no non-force error occurs.

## `aidlc map`

```text
aidlc map [flags]
```

`map` builds the repository navigation index under `docs/map/` for the selected repository root.
By default it scans the current directory, writes deterministic JSONL shards, writes
`docs/map/index.json`, rebuilds the derived `docs/map/repo-map.sqlite` query cache, prints a stable
plain-text summary, and exits `0`.

With `--check`, `map` does not rebuild artifacts. It compares the content hashes in
`docs/map/index.json` with the current files, prints `repo map: fresh` when the map is current or a
deterministic stale report when it is not, exits `0` when fresh, exits `1` when stale, and exits `2`
for invalid usage or unreadable map state.

## `aidlc query`

```text
aidlc query [flags] <search terms>
```

`query` searches the repository map for the selected repository root and prints ranked
tab-separated rows as `<path>\t<score>\t<snippet>`. It uses `docs/map/repo-map.sqlite` when present
and falls back to a JSONL scan of `docs/map/` artifacts when the cache is absent or raw cache
retrieval returns no useful rows while fallback has matches. `--shard` forces the JSONL path for
one shard.

Raw query text and structured plans may use optional bounded exact search when they contain
high-signal path, symbol, or literal clues. Exact search uses local `rg` when it is available,
passes fixed-string arguments directly without a shell, validates slash-relative path hints, and
falls back to map results with diagnostics when `rg` is missing, times out, or truncates output.
`--diagnostics` writes those notes to stderr. `--format json` returns JSON rows and may include the
same diagnostics.

Successful queries exit `0`, including empty result sets, which print no rows. Empty search terms,
negative limits, invalid usage, or unreadable map state exit `2`.

## `aidlc skill`

```text
aidlc skill install [--dir DIR] LOCAL_DIR
aidlc skill list [--dir DIR] [--format text|json]
aidlc skill remove [--dir DIR] NAME
```

`skill install` copies a validated local skill tree into `.ai/skills/installed/<name>/` without
executing files or following symlinks. `LOCAL_DIR` must contain `SKILL.md` with frontmatter `name`
and `description`; the name must be slash-safe kebab-case and match the source directory name.

Installed skills are consumer-owned local state. `aidlc init` and `aidlc update` preserve installed
sources, including `update --force`, and project them into native skill folders for Codex, Cursor,
and Claude Code. Copilot and Windsurf receive root guidance only. `skill remove` deletes the
installed source and only generated native projections whose checksums still match the lock record;
manual projection edits are preserved and reported.

## `aidlc validate`

```text
aidlc validate [--dir DIR] [--spec PATH] [--task-record PATH] [--base REV] \
  [--changed-file PATH ...] [--high-risk] [--format text|json]
```

`validate` is a read-only governance evidence check. It can verify that a supplied spec is approved,
changed files stay inside the selected scope and approved spec, a task record matches the v1 schema,
the task record digest matches the current dirty-tree contents of owned files, and check/review
evidence points at the current revision and digest. With `--high-risk`, validation requires both an
approved spec and a task record with review evidence.

Validation does not authenticate human approval, prove model review quality, prove host write
isolation, or compute real LLM tokens or cost. It reports only artifacts the local CLI can inspect.
Passing validation exits `0`, validation failures exit `1`, and usage or unreadable state exits `2`.

## `aidlc benchmark`

```text
aidlc benchmark retrieval [--dir DIR] --queries PATH [--output PATH]
```

`benchmark retrieval` runs deterministic local retrieval fixtures and emits a v1 benchmark record as
JSON. The record includes command, revision when available, dirty-tree content digest for the query
set, expected and matched paths, recall@10, precision@10, critical misses, repeated reads, retries,
output bytes, output words, source-heavy baseline bytes, compact reduction, and optional
host-reported token or cost fields when a host provides them.

When `--output` is set, the file must not already exist and is written inside the selected
repository. Without `--output`, the JSON record is printed to stdout.

## `aidlc update`

```text
aidlc update [flags]
```

`update` reads `aidlc.lock.json`, falling back to legacy `.aidlc/manifest.json` when the root lock
is absent. It fetches or reads the configured upstream source/ref, applies manifest-aware safe
updates, regenerates the IDE files persisted in `workspace.ides`, and writes the root lock after a
clean non-dry-run update.

Divergent local files are reported as conflicts and are not overwritten. Files removed upstream are
reported but not deleted from the target repository.

With `--force`, otherwise conflicting public payload destinations become overwrite decisions.
Forced update still never deletes files removed upstream and never overwrites private paths,
local-only files, installed skill sources, task evidence, or files outside
`.ai/template-manifest.yaml`.

## `aidlc upgrade`

```text
aidlc upgrade [flags]
```

`upgrade` updates a release-asset installation of the CLI binary. By default it resolves the latest
release from `shubhangtiwari/aidlc`, selects the asset for the current OS and architecture,
downloads `checksums.txt` and the release archive, verifies the archive SHA-256, extracts the
`aidlc` executable, and replaces the binary in the directory of the running executable.

If the latest release matches the current version, `upgrade` exits successfully without writing.
Explicit `--version vX.Y.Z` or `--version aidlc/vX.Y.Z` requests reinstall that release even when
the current version matches. `--dry-run` resolves the release, asset, and destination, then prints
the plan without downloading archives, extracting files, or writing the install destination.

For Homebrew-managed installs, upgrade through Homebrew instead so the package manager keeps
ownership of the installed files:

```sh
brew upgrade shubhangtiwari/aidlc/aidlc
```

## `aidlc version`

```text
aidlc version
```

Prints the CLI version:

```text
aidlc <version>
```

Development builds print `aidlc dev` unless release packaging injects a version.

## `aidlc doctor`

```text
aidlc doctor [flags]
```

`doctor` prints plain-text diagnostics for the current CLI installation and a selected repository
directory. It reports the current version, running executable path, whether `aidlc` is discoverable
through the current process `PATH`, supported common install-location candidates, whether
`.ai/Makefile.inc` exists, and whether the root `Makefile` includes that helper when present.

`doctor` does not mutate `PATH`, write files, download release assets, or invoke Bash, Make, git, or
runtime tools. Healthy diagnostics exit `0`. Installation or Make helper findings exit `1` with
next steps for sanitized IDE shells and CI, including `AIDLC_BIN` and install-directory guidance.
Invalid flags or invalid `--dir` values exit `2`.

## Common Flags

These flags apply to `init` and `update`:

| Flag | Meaning |
| --- | --- |
| `--source github|local` | Template source kind. Defaults to `github` for `init` and to the target lock source for `update`. |
| `--url URL` | GitHub repository URL. Default: `https://github.com/shubhangtiwari/aidlc`. |
| `--ref REF` | GitHub ref or local source label. Default: `main`. |
| `--path PATH` | Local source path for `--source local`. |
| `--dry-run` | Print planned changes without writing files. |
| `--force` | Overwrite divergent public payload files. |

Local source mode is intended for development and tests:

```text
aidlc init codex --source local --path /path/to/aidlc
aidlc update --source local --path /path/to/aidlc --ref main
```

## Map and Query Flags

| Flag | Meaning |
| --- | --- |
| `--dir DIR` | Repository root to map or query. Default: `.`. |
| `--check` | For `map`, check whether existing `docs/map/` artifacts are fresh instead of rebuilding them. |
| `--limit N` | For `query`, maximum ranked rows to print. Default: `10`. |
| `--shard NAME` | For `query`, search one JSONL shard directly instead of using the SQLite cache. |
| `--diagnostics` | For `query`, write retrieval diagnostics and omission notes to stderr. |
| `--format tsv|json` | For `query`, choose TSV rows or JSON output. |

## Skill Flags

| Flag | Meaning |
| --- | --- |
| `--dir DIR` | Repository root for installed skill state. Default: `.`. |
| `--format text|json` | For `skill list`, choose text rows or JSON output. |

## Validate Flags

| Flag | Meaning |
| --- | --- |
| `--dir DIR` | Repository root to validate. Default: `.`. |
| `--spec PATH` | Approved spec path, slash-relative to `--dir`. |
| `--task-record PATH` | Task record JSON evidence path, slash-relative to `--dir`. |
| `--base REV` | Git revision for changed-file discovery. |
| `--changed-file PATH` | Changed file path to validate; may be repeated. |
| `--high-risk` | Require approved spec and task record review evidence. |
| `--format text|json` | Choose text or JSON validation output. |

## Benchmark Flags

| Flag | Meaning |
| --- | --- |
| `--dir DIR` | Repository root to benchmark. Default: `.`. |
| `--queries PATH` | JSON retrieval query fixture path, slash-relative to `--dir`. |
| `--output PATH` | Optional new JSON output path, slash-relative to `--dir`. |

## Doctor Flags

| Flag | Meaning |
| --- | --- |
| `--dir DIR` | Repository root to inspect. Default: `.`. |

## Upgrade Flags

| Flag | Meaning |
| --- | --- |
| `--repo owner/repo` | GitHub repository for release lookup. Default: `shubhangtiwari/aidlc`. |
| `--version latest|TAG` | Release selector. Default: `latest`. Explicit tags may be `vX.Y.Z` or `aidlc/vX.Y.Z`. |
| `--install-dir DIR` | Directory containing the `aidlc` executable to replace. Defaults to the directory of the running executable. |
| `--dry-run` | Print the resolved release, asset, and destination without downloading archives or writing files. |

## Output

Mutating `init` and `update` output is deterministic plain text:

```text
◆ plan
<decision-state> <path> <reason>
✓ written
write <path> <comment>
✦ generated
generate <path> <comment>
```

Decision states include `create`, `skip`, `update-clean`, `overwrite`, `conflict`, and
`removed-upstream`. Forced conflict bypasses print as `overwrite` rows, not `conflict` rows.
`--dry-run --force` prints planned overwrite rows but does not write payload files, generated IDE
files, `aidlc.lock.json`, or a legacy manifest migration.

`upgrade` output is also deterministic:

```text
current version: <version>
target version: <version>
release tag: <tag>
selected asset: <asset>
destination: <path>
status: installed|skipped|dry-run
```

## Exit Codes

| Code | Meaning |
| --- | --- |
| `0` | Success, including forced overwrites, dry runs, completed upgrades, and already-latest upgrade no-ops. |
| `1` | One or more manifest-managed files conflict with local changes during `init` or `update`, `validate` found evidence failures, or `doctor` found installation/Make helper actions. |
| `2` | Usage, source, fetch, manifest, release lookup, download, checksum, extraction, install, generation, lock, or write error. |

For `aidlc upgrade`, errors exit `2` with an `aidlc upgrade:` stderr prefix.
