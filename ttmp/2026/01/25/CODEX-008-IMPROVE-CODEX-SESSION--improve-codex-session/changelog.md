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

