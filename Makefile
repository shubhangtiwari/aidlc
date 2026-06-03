# aidlc Makefile
#
# Governance targets (template-managed):
#   init           Generate IDE-specific entrypoints (CLAUDE.md, AGENTS.md, .agents/, .codex/, .cursor/, ...)
#                  from .ai/ guidance. Re-run after editing anything under .ai/.
#   update         Pull the latest .ai/ from the upstream AIDLC repository (overwrites local .ai/).
#                  Use ARGS="--dry-run" to preview, "--ref REF" to pin a branch/tag.
#   finalize-spec  Post-merge spec cleanup: marks spec status: implemented, removes in-flight
#                  artifacts, commits, optionally pushes. Wired into CI by init-architecture.
#
# Project targets:
#   install        Download Go dependencies for the isolated aidlc module.
#   run            Run the local aidlc CLI help from the isolated Go module.
#   test           Run the repository governance and aidlc gates.

.PHONY: help init update finalize-spec install run test validate-governance \
        aidlc-test aidlc-release-check claude codex cursor copilot windsurf all

BASH ?= bash
AIDLC_GOTOOLCHAIN ?= go1.25.5
INIT_ARG := $(word 2,$(MAKECMDGOALS))
FINALIZE_ARGS := $(filter-out finalize-spec,$(MAKECMDGOALS))

help:
	@echo "usage:"
	@echo "  make init <claude|codex|cursor|copilot|windsurf|all>   # generate IDE entrypoints from .ai/"
	@echo "  make update [ARGS=\"--ref REF|--dry-run|--yes|--url URL\"]  # sync .ai/ from upstream AIDLC repository"
	@echo "  make finalize-spec [ARGS=\"--dry-run|--spec PATH|--branch NAME|--push\"]  # post-merge spec cleanup"
	@echo ""
	@echo "  make aidlc-test          # run Go tests for the isolated aidlc module"
	@echo "  make aidlc-release-check # validate aidlc release packaging prerequisites"
	@echo "  make validate-governance # validate governance docs and public payload manifest"
	@echo "  make install   # download Go dependencies for the isolated aidlc module"
	@echo "  make run       # run the local aidlc CLI help"
	@echo "  make test      # run the repository governance and aidlc gates"

# --- Governance targets ------------------------------------------------------

# Generate per-IDE entrypoint files from .ai/ guidance.
# Reads .ai/personas/, .ai/skills/, .ai/README.md (INIT block), and .ai/models.defaults.toml,
# and writes IDE-native files (CLAUDE.md, AGENTS.md, .claude/skills/, .codex/agents/,
# .codex/skills/, .cursor/rules/, .cursor/skills/, ...).
# Generated files are gitignored and must not be hand-edited; re-run after changing .ai/.
init:
	@if [ -z "$(INIT_ARG)" ]; then \
		echo "usage: make init <claude|codex|cursor|copilot|windsurf|all>"; exit 2; \
	fi
	@chmod +x .ai/scripts/ai_init.sh
	@case "$(INIT_ARG)" in \
		claude|codex|cursor|copilot|windsurf|all) \
			$(BASH) .ai/scripts/ai_init.sh "$(INIT_ARG)" ;; \
		*) echo "unknown init target '$(INIT_ARG)'"; \
		   echo "usage: make init <claude|codex|cursor|copilot|windsurf|all>"; exit 2 ;; \
	esac

# Pull the latest .ai/ from the upstream AIDLC repository and overwrite the local copy.
# Shallow-clones upstream, rsyncs .ai/ with --delete, prompts before applying.
# Useful so consumers can pick up new personas, skills, scripts, and references without re-forking.
update:
	@chmod +x .ai/scripts/ai_update.sh
	@$(BASH) .ai/scripts/ai_update.sh $(ARGS)

# Post-merge spec finalization: flips the merged spec to status: implemented, removes the
# in-flight pointer, commits the change, and optionally pushes. Intended to run on the default
# branch after a spec PR merges (init-architecture wires it into CI).
finalize-spec:
	@chmod +x .ai/scripts/finalize_spec.sh
	@$(BASH) .ai/scripts/finalize_spec.sh $(ARGS)

# --- Project targets ---------------------------------------------------------

install:
	@cd aidlc && go mod download

run:
	@cd aidlc && go run ./cmd/aidlc --help

test:
	@$(MAKE) validate-governance
	@$(MAKE) aidlc-test

aidlc-test:
	@cd aidlc && go test ./...

aidlc-release-check:
	@GOTOOLCHAIN=$(AIDLC_GOTOOLCHAIN) aidlc/scripts/verify-release.sh

validate-governance:
	@test -f docs/ARCHITECTURE.md
	@test -f docs/architecture/software.md
	@test -f docs/adr/1780346463-aidlc-cli-distribution-and-sync.md
	@test -f docs/blueprints/aidlc.md
	@test -f docs/blueprints/template-payload.md
	@test -f .ai/template-manifest.yaml
	@grep -q "docs/spec/\\[0-9\\]\\*-\\*.md" .ai/template-manifest.yaml
	@grep -q "docs/adr/\\[0-9\\]\\*-\\*.md" .ai/template-manifest.yaml
	@grep -q "docs/ARCHITECTURE.md" .ai/template-manifest.yaml
	@grep -q "docs/architecture/\\*\\*" .ai/template-manifest.yaml
	@grep -q "aidlc/\\*\\*" .ai/template-manifest.yaml
	@grep -q ".github/\\*\\*" .ai/template-manifest.yaml
	@grep -q "release/\\*\\*" .ai/template-manifest.yaml
	@test -f aidlc/go.mod
	@test ! -f go.mod
	@test ! -f go.sum

# Sentinel rule so 'make init <name>' doesn't try to build <name> as a separate target.
claude codex cursor copilot windsurf all:
	@:
