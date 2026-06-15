AI_MAP_DIR ?= .
AI_QUERY ?=
AI_QUERY_LIMIT ?= 10
AI_QUERY_SHARD ?=

.PHONY: ai-map ai-map-check ai-query

ai-map:
	aidlc map --dir "$(AI_MAP_DIR)"

ai-map-check:
	aidlc map --dir "$(AI_MAP_DIR)" --check

ai-query:
	@if [ -z "$(AI_QUERY)" ]; then \
		printf '%s\n' 'usage: make ai-query AI_QUERY="search terms" [AI_QUERY_LIMIT=10] [AI_QUERY_SHARD=shard]'; \
		exit 2; \
	fi
	aidlc query --dir "$(AI_MAP_DIR)" --limit "$(AI_QUERY_LIMIT)" $(if $(AI_QUERY_SHARD),--shard "$(AI_QUERY_SHARD)") "$(AI_QUERY)"
