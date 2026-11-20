# RAG rollback runbook

The AI index is versioned (`ai_kb_v<timestamp>`) and searches through
`ai_kb_current`. Old versions are retained for seven days and remain in the
dual-write registry while retained.

1. Inspect versions:
   `redis-cli --scan --pattern 'ai_kb_v*'`
   `redis-cli GET ai:rag:current`
2. Choose a previous concrete index and its prefix. Verify
   `FT.INFO <index>` reports the expected `num_docs` (the demo index is 64);
   never switch to a 0-document failed build.
3. Switch the alias atomically:
   `FT.ALIASUPDATE ai_kb_current ai_kb_v<timestamp>`
4. Invalidate the version-bound query caches (`ai:rag:q:*`, `ai:rag:sem:*`);
   otherwise a rollback can still serve cached documents from the old target.
5. Publish the matching current pointer:
   `SET ai:rag:current '{"index":"ai_kb_current","prefix":"ai_kb_v<timestamp>:","created_at":...}'`
6. AI Pods reload the pointer within `RAG_ALIAS_REFRESH` (default 1m); no
   restart is needed. They only load the alias and never rebuild it.
7. Verify: search "耳机" returns documents, `mall_ai_rag_search_total` grows, and
   the eval set still passes. If the old version is stale, force a rebuild with
   the `ai-rag-build` Job (`RAG_BUILD_FORCE=true`) or wait for the 15-minute
   reconciliation worker.

Only switch back to a version that was built from the same catalog schema. Do
not delete a version while a rollback is active.
