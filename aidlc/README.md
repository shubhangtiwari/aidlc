# aidlc CLI Reference

`aidlc` is the native CLI for installing, updating, and upgrading the AIDLC governance harness in a
repository. It copies the portable payload, renders IDE-specific files, tracks sync state in
`aidlc.lock.json`, and upgrades the installed CLI binary from GitHub release assets.

This reference documents CLI behavior only. Repository architecture analysis and blueprint creation
are performed by the generated agent harness after `aidlc init <ide>` has created the IDE files.

## Commands

```text
aidlc init <claude|codex|cursor|copilot|windsurf|all> [flags]
aidlc map [flags]
aidlc query [flags] <search terms>
aidlc update [flags]
aidlc upgrade [flags]
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
and falls back to a JSONL scan of `docs/map/` artifacts when the cache is absent. `--shard` forces
the JSONL path for one shard.

Successful queries exit `0`, including empty result sets, which print no rows. Empty search terms,
negative limits, invalid usage, or unreadable map state exit `2`.

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
local-only files, or files outside `.ai/template-manifest.yaml`.

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
| `1` | One or more manifest-managed files conflict with local changes during `init` or `update`. |
| `2` | Usage, source, fetch, manifest, release lookup, download, checksum, extraction, install, generation, lock, or write error. |

For `aidlc upgrade`, errors exit `2` with an `aidlc upgrade:` stderr prefix.
