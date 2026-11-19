# RAG rollback runbook

The AI index is versioned (`ai_kb_v<timestamp>`) and searches through
`ai_kb_current`. Old versions are retained for seven days and remain in the
dual-write registry while retained.

1. Inspect versions:
   `redis-cli --scan --pattern 'ai_kb_v*'`
   `redis-cli GET ai:rag:current`
2. Choose a previous concrete index and its prefix.
3. Switch the alias atomically:
   `FT.ALIASUPDATE ai_kb_current ai_kb_v<timestamp>`
4. Publish the matching current pointer:
   `SET ai:rag:current '{"index":"ai_kb_current","prefix":"ai_kb_v<timestamp>:","created_at":...}'`
5. Restart AI replicas so they load the pointer (non-leaders do not rebuild).
6. Verify: search "耳机" returns documents, `mall_ai_rag_search_total` grows, and
   the eval set still passes. If the old version is stale, run reconciliation
   by restarting one leader Pod or wait for the 15-minute worker.

Only switch back to a version that was built from the same catalog schema. Do
not delete a version while a rollback is active.
