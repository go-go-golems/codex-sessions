# Changelog

## 2026-01-25

- Initial workspace created


## 2026-01-25

Step 3: add schema v2 migration for metadata + meta_kv (commit 13fda78)

### Related Files

- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/internal/indexdb/schema.go — Schema migration v2 with metadata columns + session_meta_kv


## 2026-01-25

Step 4: store metadata columns + reflection flag in index build (commit 3cfdaee)

### Related Files

- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/internal/indexdb/build.go — Persist meta_json
- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/internal/sessions/parser.go — ReadSessionMetaPayload helper


## 2026-01-25

Step 5: add session_meta_kv extraction + upsert (commit ee71fe3)

### Related Files

- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/internal/indexdb/build.go — Flatten session_meta into K/V rows


## 2026-01-25

Step 6: SQLite-first list/search with staleness refresh (commit 91d1415)

### Related Files

- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/cmd/codex-session/list.go — SQLite-first list path with reindex controls
- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/cmd/codex-session/search.go — Indexed search now refreshes stale rows
- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/internal/indexdb/list.go — ListSessions + staleness detection helpers


## 2026-01-25

Step 7: reset schema + add tool call args columns (commit 40b5089)

### Related Files

- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/internal/indexdb/build.go — Store arguments_flat/JSON for tool calls
- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/internal/indexdb/schema.go — Reset schema on version change
- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/internal/sessions/facets.go — Capture tool call_id


## 2026-01-26

Step 8: add tool/arg filters to indexed search (commit 1d97a54)

### Related Files

- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/cmd/codex-session/search.go — Wire --tool/--arg flags (key:value)
- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/internal/indexdb/build.go — Tokenize tool-call arguments for FTS
- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/internal/indexdb/search.go — Compose tool arg FTS query


## 2026-01-26

Step 9: backfill list when index is sparse (commit b186982)

### Related Files

- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/cmd/codex-session/list.go — Backfill list when rows < limit


## 2026-01-26

Step 10: allow tool-only search (commit 3e12f53)

### Related Files

- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/cmd/codex-session/search.go — Relax validation for tool-only
- /home/manuel/workspaces/2026-01-25/improve-codex-session/codex-sessions/internal/indexdb/search.go — Tool-only query path

