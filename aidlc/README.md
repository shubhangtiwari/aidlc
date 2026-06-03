# aidlc

`aidlc` is the native CLI for initializing and updating AIDLC governance payloads.
It uses only Go standard-library filesystem, archive, HTTP, checksum, and rendering
logic.

This README is the canonical command reference for the native CLI. Repository
Makefile targets such as `make init <ide>` and `make update` may wrap these
commands for local workflows, but the supported user-facing behavior is defined
by `aidlc init` and `aidlc update`.

## Commands

```text
aidlc init <claude|codex|cursor|copilot|windsurf|all> [flags]
aidlc update [flags]
aidlc version
```

`aidlc init <ide>` copies only paths listed in `.ai/template-manifest.yaml` from
the configured source into the current directory, then runs native IDE generation.
Existing identical files are skipped. Divergent files are reported as conflicts
and are not overwritten. Successful runs write `aidlc.lock.json` at the target
repository root with the selected concrete IDEs in `workspace.ides`; `init all`
stores every concrete IDE rather than the aggregate `all` value.

`aidlc update` reads `aidlc.lock.json`, falling back to legacy
`.aidlc/manifest.json` when the root lock is absent, fetches or reads the
configured upstream source/ref, and applies only manifest-aware safe updates.
After a clean non-dry-run update, it regenerates the IDE files listed in
`workspace.ides` and writes the root lock. Divergent local files are reported as
conflicts and are not overwritten. Files removed upstream are reported but not
deleted from the target repository.

## Flags

```text
--source github|local   Template source kind. Defaults to github for init and to
                        the target manifest source for update.
--url URL               GitHub repository URL. Default:
                        https://github.com/shubhangtiwari/aidlc
--ref REF               GitHub ref or local source label. Default: main.
--path PATH             Local source path for --source local.
--dry-run               Print planned changes without writing files.
```

Local source mode is intended for development and tests:

```text
aidlc init codex --source local --path /path/to/aidlc
aidlc update --source local --path /path/to/aidlc --ref main
```

## Exit Codes

```text
0  success
1  one or more manifest-managed files conflict with local changes
2  usage, source, fetch, manifest, or write error
```

`aidlc version` prints `aidlc <version>`. Development builds print `aidlc dev`
unless release packaging injects a version.
