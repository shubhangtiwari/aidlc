# aidlc

`aidlc` is the native CLI for initializing and updating AIDLC governance payloads.
It uses only Go standard-library filesystem, archive, HTTP, checksum, and rendering
logic.

This README is the canonical command reference for the native CLI. Repository
Makefile targets such as `make init <ide>` and `make update` may wrap payload
commands for local workflows, but the supported user-facing behavior is defined
by `aidlc init`, `aidlc update`, `aidlc upgrade`, and `aidlc version`.

## Commands

```text
aidlc init <claude|codex|cursor|copilot|windsurf|all> [flags]
aidlc update [flags]
aidlc upgrade [flags]
aidlc version
```

`aidlc init <ide>` copies only paths listed in `.ai/template-manifest.yaml` from
the configured source into the current directory, then runs native IDE generation.
Existing identical files are skipped. Divergent files are reported as conflicts
and are not overwritten. When conflicts exist, init still writes non-conflicting
payload files, generates the requested IDE files, writes `aidlc.lock.json`, and
exits `1`. The partial lock records only accepted upstream payload files; it
does not mark conflicted paths as clean. Successful and partial runs write
`aidlc.lock.json` at the target repository root with the selected concrete IDEs
in `workspace.ides`; `init all` stores every concrete IDE rather than the
aggregate `all` value.

`aidlc init <ide> --force` converts conflicting public payload destinations into
explicit overwrite decisions. It replaces those divergent payload files with the
selected upstream content, writes other payload files, generates the requested
IDE files, records overwritten paths as clean tracked files in `aidlc.lock.json`,
and exits `0` when no non-force error occurs.

`aidlc update` reads `aidlc.lock.json`, falling back to legacy
`.aidlc/manifest.json` when the root lock is absent, fetches or reads the
configured upstream source/ref, and applies only manifest-aware safe updates.
After a clean non-dry-run update, it regenerates the IDE files listed in
`workspace.ides` and writes the root lock. Divergent local files are reported as
conflicts and are not overwritten. Files removed upstream are reported but not
deleted from the target repository.

`aidlc update --force` converts otherwise conflicting public payload destinations
into overwrite decisions, replaces them with the selected upstream content,
regenerates only the persisted `workspace.ides` surfaces, and writes
`aidlc.lock.json` for the accepted upstream plan. If only legacy
`.aidlc/manifest.json` exists, forced update may migrate to the root lock only
during a non-dry-run mutation. `--force` never deletes files removed upstream and
never overwrites private paths, local-only files, or files outside
`.ai/template-manifest.yaml`.

`aidlc upgrade` updates the installed CLI binary from GitHub release assets. By
default it resolves the latest release from `shubhangtiwari/aidlc`, selects the
asset for the current OS and architecture, downloads `checksums.txt` and the
release archive, verifies the archive SHA-256, extracts the `aidlc` executable,
and replaces the binary in the directory of the running executable. If the
resolved latest release matches the current version, the command exits
successfully without writing. Explicit `--version vX.Y.Z` or
`--version aidlc/vX.Y.Z` requests reinstall that release even when the current
version matches. `--dry-run` resolves the release, asset, and destination, then
prints the plan without downloading archives, extracting files, or writing the
install destination.

Upgrade output is deterministic plain text:

```text
current version: <version>
target version: <version>
release tag: <tag>
selected asset: <asset>
destination: <path>
status: installed|skipped|dry-run
```

Mutating init and update output is grouped into deterministic plain-text
sections:

```text
◆ plan
<decision-state> <path> <reason>
✓ written
write <path> <comment>
✦ generated
generate <path> <comment>
```

Decision states include `create`, `skip`, `update-clean`, `overwrite`,
`conflict`, and `removed-upstream`. Forced conflict bypasses are printed as
`overwrite` rows, not `conflict` rows. `--dry-run --force` prints the planned
`overwrite` rows but omits written and generated sections because it does not
write payload files, generated IDE files, the root lock, or a legacy manifest
migration.

## Flags

```text
--source github|local   Template source kind. Defaults to github for init and to
                        the target manifest source for update.
--url URL               GitHub repository URL. Default:
                        https://github.com/shubhangtiwari/aidlc
--ref REF               GitHub ref or local source label. Default: main.
--path PATH             Local source path for --source local.
--dry-run               Print planned changes without writing files.
--force                 Overwrite divergent public payload files.
```

Local source mode is intended for development and tests:

```text
aidlc init codex --source local --path /path/to/aidlc
aidlc update --source local --path /path/to/aidlc --ref main
```

Upgrade-specific flags:

```text
--repo owner/repo      GitHub repository for release lookup. Default:
                       shubhangtiwari/aidlc.
--version latest|TAG   Release selector. Default: latest. Explicit tags may be
                       vX.Y.Z or aidlc/vX.Y.Z and normalize to vX.Y.Z output.
--install-dir DIR      Directory containing the aidlc executable to replace.
                       Default: directory of the running executable.
--dry-run              Print the resolved release, asset, and destination
                       without downloading archives or writing files.
```

Installer scripts (`aidlc/scripts/install.sh` and `aidlc/scripts/install.ps1`)
bootstrap a machine by downloading published release assets. `aidlc upgrade`
uses the same release asset naming scheme and `checksums.txt` verification for
an already installed CLI, but performs release lookup, checksum verification,
archive extraction, and binary replacement natively rather than invoking the
installer scripts or shelling out to Bash, Make, rsync, git, curl, tar, zip, or
PowerShell.

## Exit Codes

```text
0  success
1  one or more manifest-managed files conflict with local changes
2  usage, source, fetch, manifest, release lookup, download, checksum,
   extraction, install, or write error
```

For `aidlc init` and `aidlc update`, exit `0` includes successful forced
overwrites and dry-run force previews. Non-forced conflicts still exit `1`.
Write, generation, lock, source, fetch, and usage errors are not downgraded by
`--force`; they exit `2`.

For `aidlc upgrade`, exit `0` covers a completed install, a dry-run, and an
already-latest no-op. Release lookup, download, checksum, extraction, install,
unsupported platform, and invalid flag errors exit `2` with an
`aidlc upgrade:` stderr prefix.

`aidlc version` prints `aidlc <version>`. Development builds print `aidlc dev`
unless release packaging injects a version.
