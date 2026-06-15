AIDLC_BIN ?= aidlc
AI_MAP_DIR ?= .
AI_MAP_INCLUDE ?=
AI_QUERY ?=
AI_QUERY_LIMIT ?= 10
AI_QUERY_SHARD ?=

.PHONY: ai-map ai-map-check ai-query

ai-map:
	$(AIDLC_BIN) map --dir "$(AI_MAP_DIR)" $(if $(AI_MAP_INCLUDE),--include "$(AI_MAP_INCLUDE)")

ai-map-check:
	$(AIDLC_BIN) map --dir "$(AI_MAP_DIR)" --check

ai-query:
	@if [ -z "$(AI_QUERY)" ]; then \
		printf '%s\n' 'usage: make ai-query AI_QUERY="search terms" [AI_QUERY_LIMIT=10] [AI_QUERY_SHARD=shard]'; \
		exit 2; \
	fi
	$(AIDLC_BIN) query --dir "$(AI_MAP_DIR)" --limit "$(AI_QUERY_LIMIT)" $(if $(AI_QUERY_SHARD),--shard "$(AI_QUERY_SHARD)") "$(AI_QUERY)"
