AI_MAP_DIR ?= .
AI_MAP_INCLUDE ?=
AI_QUERY ?=
AI_QUERY_LIMIT ?= 10
AI_QUERY_SHARD ?=
AIDLC_SYSTEM_BIN_DIRS ?= /opt/homebrew/bin /usr/local/bin

AIDLC_RESOLVE = aidlc_bin="$$( \
	if [ -n "$(AIDLC_BIN)" ]; then \
		printf '%s\n' "$(AIDLC_BIN)"; \
	elif command -v aidlc >/dev/null 2>&1; then \
		command -v aidlc; \
	else \
		if [ -n "$${LOCALAPPDATA:-}" ]; then \
			for candidate in "$$LOCALAPPDATA/Programs/aidlc/bin/aidlc.exe" "$$LOCALAPPDATA/Programs/aidlc/bin/aidlc"; do \
				if [ -x "$$candidate" ]; then printf '%s\n' "$$candidate"; exit 0; fi; \
			done; \
		fi; \
		if [ -n "$${HOME:-}" ]; then \
			for candidate in "$$HOME/.local/bin/aidlc" "$$HOME/bin/aidlc"; do \
				if [ -x "$$candidate" ]; then printf '%s\n' "$$candidate"; exit 0; fi; \
			done; \
		fi; \
		for dir in $(AIDLC_SYSTEM_BIN_DIRS); do \
			candidate="$$dir/aidlc"; \
			if [ -x "$$candidate" ]; then printf '%s\n' "$$candidate"; exit 0; fi; \
		done; \
		if [ -n "$${HOME:-}" ]; then \
			for candidate in "$$HOME/.local/bin/aidlc.exe" "$$HOME/bin/aidlc.exe"; do \
				if [ -x "$$candidate" ]; then printf '%s\n' "$$candidate"; exit 0; fi; \
			done; \
		fi; \
	fi \
)"; \
if [ -z "$$aidlc_bin" ]; then \
	printf '%s\n' 'aidlc executable not found.' >&2; \
	printf '%s\n' 'Set AIDLC_BIN=/path/to/aidlc, or install aidlc and ensure it is visible to this shell.' >&2; \
	printf '%s\n' 'The Make helper checks PATH, $$LOCALAPPDATA/Programs/aidlc/bin/aidlc.exe, $$HOME/.local/bin/aidlc, $$HOME/bin/aidlc, /opt/homebrew/bin/aidlc, and /usr/local/bin/aidlc.' >&2; \
	printf '%s\n' 'For sanitized IDE shells or CI, pass AIDLC_BIN explicitly.' >&2; \
	printf '%s\n' 'Installer guidance: use AIDLC_INSTALL_DIR for the Unix installer, or rerun the PowerShell installer so %LOCALAPPDATA%\Programs\aidlc\bin is available to new shells through the user PATH.' >&2; \
	printf '%s\n' 'After installation, run make ai-doctor for diagnostics.' >&2; \
	exit 2; \
fi

.PHONY: ai-map ai-map-check ai-query ai-doctor

ai-map:
	@$(AIDLC_RESOLVE); \
	"$$aidlc_bin" map --dir "$(AI_MAP_DIR)" $(if $(AI_MAP_INCLUDE),--include "$(AI_MAP_INCLUDE)")

ai-map-check:
	@$(AIDLC_RESOLVE); \
	"$$aidlc_bin" map --dir "$(AI_MAP_DIR)" --check

ai-query:
	@if [ -z "$(AI_QUERY)" ]; then \
		printf '%s\n' 'usage: make ai-query AI_QUERY="search terms" [AI_QUERY_LIMIT=10] [AI_QUERY_SHARD=shard]'; \
		exit 2; \
	fi
	@$(AIDLC_RESOLVE); \
	"$$aidlc_bin" query --dir "$(AI_MAP_DIR)" --limit "$(AI_QUERY_LIMIT)" $(if $(AI_QUERY_SHARD),--shard "$(AI_QUERY_SHARD)") "$(AI_QUERY)"

ai-doctor:
	@$(AIDLC_RESOLVE); \
	if ! "$$aidlc_bin" doctor --help >/dev/null 2>&1; then \
		printf '%s\n' 'resolved aidlc executable does not support the doctor command.' >&2; \
		printf 'Resolved aidlc: %s\n' "$$aidlc_bin" >&2; \
		printf '%s\n' 'Upgrade or reinstall aidlc, or set AIDLC_BIN=/path/to/newer/aidlc.' >&2; \
		printf '%s\n' 'For sanitized IDE shells or CI, pass AIDLC_BIN explicitly.' >&2; \
		exit 2; \
	fi; \
	"$$aidlc_bin" doctor --dir "$(AI_MAP_DIR)"
