# AIDLC

MIT License - Copyright (c) 2026 [Shubhang Tiwari](mailto:shubh.bitsmith@gmail.com).

This repository hosts the AIDLC governance template and native CLI for AI-assisted repositories. It contains
project-neutral operating rules, personas, skills, documentation templates, and initialization
tooling for common coding assistants.

It intentionally does not include project code, dependency manifests, architecture profiles, quality
gate definitions, or placeholder scaffolding. Fork the template, run the initializer, then document
the real project structure as it takes shape.

## Contents

- `.ai/` - portable AI personas, skills, templates, delegation, scripts, and optional model defaults.
- `aidlc/` - native CLI source and command reference for initializing and updating governance files.
- `docs/spec/` - spec-first workflow documentation and tracker.
- `docs/adr/` - architecture decision record guidance.
- `docs/blueprints/` - module blueprint guidance and template.
- `.ai/scripts/finalize_spec.sh` - post-merge spec finalization helper.

## Initialize

Governance projection comes entirely from `.ai/` and `docs/`. Use the native CLI to initialize a
repository for one IDE or all supported IDEs:

```sh
aidlc init codex
aidlc init claude
aidlc init cursor
aidlc init copilot
aidlc init windsurf
aidlc init all
```

This writes assistant-specific files such as `AGENTS.md`, `.codex/agents/*.toml`,
`.agents/skills/*/SKILL.md`, `CLAUDE.md`, or `.cursor/rules/` (gitignored and not committed in
this template). Cursor output includes an always-applied AI DLC discovery rule so Composer, Claude,
Gemini, and other Cursor Agent models can find generated personas and skills. Regenerate after
changing `.ai/`; do not edit generated IDE files by hand.

This repository also keeps Makefile wrappers for local governance workflows:

```sh
make init codex
make update
```

Those targets delegate to native `aidlc init` and `aidlc update`; they are repository execution
wrappers, not separate shell compatibility commands.

Generated assistant files do not infer project, dependency, or toolchain facts. Forked projects
should document those facts explicitly in their own architecture, blueprint, ADR, and spec files when
they become relevant.

## Install `aidlc`

`aidlc` is the native CLI for initializing and updating AIDLC governance files without depending on
the template `Makefile` or shell scripts.

Install with Go:

```sh
go install github.com/shubhangtiwari/aidlc/aidlc/cmd/aidlc@latest
```

Install from the latest GitHub release on macOS or Linux after release assets are published:

```sh
curl -fsSL https://raw.githubusercontent.com/shubhangtiwari/aidlc/main/aidlc/scripts/install.sh | sh
```

Install from the latest GitHub release on Windows after release assets are published:

```powershell
iwr https://raw.githubusercontent.com/shubhangtiwari/aidlc/main/aidlc/scripts/install.ps1 -UseB | iex
```

For local development from this repository:

```sh
cd aidlc
go install ./cmd/aidlc
```

Make sure the install directory is on your `PATH`, then verify:

```sh
aidlc version
```

## Release `aidlc`

The release workflow builds native archives for macOS, Linux, and Windows, writes `checksums.txt`,
and publishes them to the GitHub release for an existing `aidlc/v*` tag.

```sh
make aidlc-release-check
git tag -a aidlc/v0.1.0 -m "aidlc v0.1.0"
git push origin aidlc/v0.1.0
```

The tag prefix is intentional because the Go module lives under `aidlc/`.

## Use `aidlc`

Initialize a repository for one IDE or all supported IDEs:

```sh
aidlc init codex
aidlc init claude
aidlc init cursor
aidlc init copilot
aidlc init windsurf
aidlc init all
```

`aidlc init <ide>` copies the public AIDLC payload into the current repository and generates the
selected IDE files. Successful runs write `aidlc.lock.json` at the repository root. The lock records
the selected concrete IDEs in `workspace.ides`, so `aidlc init all` stores every supported IDE
instead of the literal `all` value.

Update an initialized repository from the recorded upstream source and regenerate the IDE files
stored in `workspace.ides`:

```sh
aidlc update
```

Preview changes without writing files:

```sh
aidlc init codex --dry-run
aidlc update --dry-run
```

Use a local template checkout while developing or testing AIDLC changes:

```sh
aidlc init codex --source local --path /path/to/aidlc
aidlc update --source local --path /path/to/aidlc --ref main
```

Divergent local files are reported as conflicts and are not overwritten. See
[`aidlc/README.md`](aidlc/README.md) for the full command reference and exit codes.

## Customize Architecture

Use the `init-architecture` skill in `.ai/skills/` after forking. It analyzes the repo and creates
`docs/ARCHITECTURE.md` from real project evidence. Module contracts live in `docs/blueprints/` as
the project grows.
