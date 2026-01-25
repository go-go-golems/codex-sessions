---
Title: 'Test Report: Go CLI Smoke Test (codex-sessions)'
Ticket: CODEX-001-PORT-GO
Status: active
Topics:
    - backend
    - chat
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../../../../.codex/sessions/session_index.sqlite
      Note: Full-archive index built during validation (406 sessions
    - Path: cmd/codex-sessions/export.go
      Note: Export command validated in follow-up run (commit 39e2894)
    - Path: cmd/codex-sessions/index_build.go
      Note: Index build command tested (commit e9d44ff)
    - Path: cmd/codex-sessions/index_stats.go
      Note: Index stats command tested (commit e9d44ff)
    - Path: cmd/codex-sessions/list.go
      Note: List command
    - Path: cmd/codex-sessions/main.go
      Note: CLI entrypoint
    - Path: cmd/codex-sessions/projects.go
      Note: Projects command
    - Path: cmd/codex-sessions/reflect.go
      Note: Reflect command tested (commit 80e630b)
    - Path: cmd/codex-sessions/search.go
      Note: Search command
    - Path: cmd/codex-sessions/show.go
      Note: Show command
    - Path: internal/indexdb
      Note: Index implementation
    - Path: internal/reflect
      Note: Reflection pipeline implementation
    - Path: internal/sessions
      Note: Parsing and normalization
    - Path: internal/sessions/facets.go
      Note: Tool facet extraction fixed for custom_tool_call shapes (commit 39e2894)
ExternalSources: []
Summary: Smoke test results for the Go Glazed CLI against the real ~/.codex/sessions archive (projects/list/show/search/export/index/reflect).
LastUpdated: 2026-01-24T19:20:47.940213553-05:00
WhatFor: ""
WhenToUse: ""
---






# Test Report: Go CLI Smoke Test (codex-sessions)

## Goal

Validate that the in-progress Go port (Glazed CLI) runs successfully against the real local Codex sessions archive, produces correct-looking output, and surfaces obvious gaps/UX issues before deeper implementation (facets, indexing, reflection parity).

## Context

**Date run:** 2026-01-25 (UTC)

**Repo:** `/home/manuel/code/others/llms/Codex-Reflect-Skill`

**Git HEAD tested:** `784bbd6`

**Host OS:** Ubuntu Linux 6.8 (x86_64)

**Go:** `go1.25.5 linux/amd64`

**Sessions root:** `/home/manuel/.codex/sessions`

**Rollout JSONL files discovered:** 401

Notes:
- Tests intentionally used the non-indexed streaming approach (no SQLite/FTS yet).
- Outputs were captured in `/tmp` for inspection.

## Quick Reference

### Commands executed

Unit tests:

```bash
go test ./... -count=1
```

Projects count:

```bash
go run ./cmd/codex-sessions projects --output table
go run ./cmd/codex-sessions projects --output json > /tmp/codex_sessions_projects.json
```

List sessions (include most recent, last 5):

```bash
go run ./cmd/codex-sessions list --include-most-recent --limit 5 --output table
go run ./cmd/codex-sessions list --include-most-recent --limit 5 --output json > /tmp/codex_sessions_list.json
```

Show one session by file path:

```bash
go run ./cmd/codex-sessions show --path /home/manuel/.codex/sessions/2026/01/24/rollout-2026-01-24T13-39-26-019bf14d-cd4e-7c22-ac48-a9fb1e3d4d89.jsonl --max-chars 300 --output json > /tmp/codex_sessions_show.json
```

Search (streaming scan) for a substring:

```bash
go run ./cmd/codex-sessions search --query error --include-most-recent --limit 20 --output json > /tmp/codex_sessions_search.json
```

### Results summary

- `go test`: PASS
- `projects`: OK (100 project rows emitted; includes `Codex-Reflect-Skill` marked `current=true`)
- `list`: OK (5 rows; title derived; updated_at derived; paths correct)
- `show`: OK (39 normalized message rows; includes both `event_msg` and `response_item` sources)
- `search`: OK (20 rows for the test run; matching behavior as expected for substring search)

### Captured artifacts

- `/tmp/codex_sessions_projects.json` (projects rows, JSON)
- `/tmp/codex_sessions_list.json` (list rows, JSON)
- `/tmp/codex_sessions_show.json` (show rows, JSON)
- `/tmp/codex_sessions_search.json` (search rows, JSON)
- `/tmp/codex_sessions_projects_sample.txt` (table sample, truncated)
- `/tmp/codex_sessions_list_sample.txt` (table sample, truncated)
- `/tmp/codex_sessions_show_sample.txt` (table sample, truncated)
- `/tmp/codex_sessions_search_sample.txt` (table sample, truncated)

### Performance notes (built binary)

To avoid `go run` compile overhead, I also built a binary to `/tmp/codex-sessions` and timed it:

- `projects` (401 files scanned for first-line meta): ~0.04s
- `list --limit 50` (401 first-line metas + scan timestamps + title extraction): ~2.17s
- `show` (single file, 39 messages): ~0.20s
- `search --limit 50` (streaming scan across 50 sessions): ~8.11s

These timings are acceptable for a non-indexed baseline; an index/FTS path will be needed for interactive search across the full archive.

### reMarkable upload

Uploaded as a PDF via `remarquee`:

- Remote dir: `/ai/2026/01/25/CODEX-001-PORT-GO`
- Document name: `CODEX-001-Go-CLI-Smoke-Test` (PDF generated from a temp copy of this markdown)

### Issues / gaps observed

1. **Multiline text in table output**: `search` snippets and `show` message text can contain newlines, which makes table output hard to read and can explode output size.
   - Status: FIXED in a later commit (see follow-up run section below).

2. **Reflection-copy filtering gap**: discovery currently excludes filenames containing `-copy`, but does not detect “[SELF-REFLECTION]” copies by content (the Python tool does).
   - Suggested fix: implement a fast prefix check against early user messages, and optionally skip those files during discovery/list/search unless explicitly requested.

3. **Search semantics**: `--limit` currently limits the number of sessions scanned (matching Python semantics), not “number of matches returned”. This is fine but should be documented in `--help` strings.

## Follow-up Run: Export + Tool Facets (Real Session Shapes)

**Date run:** 2026-01-25 (UTC)

**Git HEAD tested:** `39e2894`

### Commands executed

```bash
go test ./... -count=1

go run ./cmd/codex-sessions show --path /home/manuel/.codex/sessions/2026/01/24/rollout-2026-01-24T13-39-26-019bf14d-cd4e-7c22-ac48-a9fb1e3d4d89.jsonl --view tools --limit 8 --single-line --output table

go run ./cmd/codex-sessions export --path /home/manuel/.codex/sessions/2026/01/24/rollout-2026-01-24T13-39-26-019bf14d-cd4e-7c22-ac48-a9fb1e3d4d89.jsonl --shape document --extract facets --output json > /tmp/codex-export-facets.json
jq '.[0].document.facets.tool_calls|length' /tmp/codex-export-facets.json
jq '.[0].document.facets.tool_outputs|length' /tmp/codex-export-facets.json
```

### Results summary

- `go test`: PASS
- `show --view tools`: OK (renders tool call/output rows; readable with `--single-line`)
- `export --shape document --extract facets`: OK
  - tool_calls length: 76
  - tool_outputs length: 76

### Notes

- Real Codex sessions in this archive represent tool calls as `response_item.payload.type=custom_tool_call` and tool outputs as `custom_tool_call_output`, with linkage via `call_id`.
- Tool output payloads do not necessarily include the tool name, so correct extraction requires correlating output rows to earlier calls using `call_id`.

## Follow-up Run: SQLite/FTS Index (Build + Stats + Search)

**Date run:** 2026-01-25 (UTC)

**Git HEAD tested:** `e9d44ff`

### Commands executed

```bash
go test ./... -count=1

go run ./cmd/codex-sessions index build --include-most-recent --limit 5 --output json > /tmp/codex_index_build.json
go run ./cmd/codex-sessions index stats --output json > /tmp/codex_index_stats.json

go run ./cmd/codex-sessions search --query apply --scope tools --per-message --max-results 5 --output json > /tmp/codex_index_search_tools.json
go run ./cmd/codex-sessions search --query Create --scope messages --max-results 5 --output json > /tmp/codex_index_search_messages.json
```

### Results summary

- `go test`: PASS
- `index build` (5 sessions): OK (rows show `status=indexed|skipped`, index file created/updated)
- `index stats`: OK (shows row counts; last indexed timestamp present)
- `search` (indexed): OK (emits `backend=index` and uses FTS scope selection)

### Captured artifacts

- `/tmp/codex_index_build.json` (index build rows)
- `/tmp/codex_index_stats.json` (index stats row)
- `/tmp/codex_index_search_tools.json` (indexed search, tools scope)
- `/tmp/codex_index_search_messages.json` (indexed search, messages scope)

## Follow-up Run: Reflect (Cache + Codex Resume)

**Date run:** 2026-01-25 (UTC)

**Git HEAD tested:** `80e630b`

### Commands executed

```bash
go test ./... -count=1

# Dry run (no codex invocation)
go run ./cmd/codex-sessions reflect --dry-run --include-most-recent --limit 2 --extra-metadata --output table

# Real run (one session id)
go run ./cmd/codex-sessions reflect --session-id 019bf592-c4a2-7972-8e78-3c566986b19f --extra-metadata --codex-timeout-seconds 120 --output json > /tmp/codex_reflect_one.json

# Cache reuse check
go run ./cmd/codex-sessions reflect --session-id 019bf592-c4a2-7972-8e78-3c566986b19f --output json > /tmp/codex_reflect_one_cached.json
```

### Results summary

- `go test`: PASS
- `reflect --dry-run`: OK (selection + cache paths computed without running codex)
- `reflect` (one session): OK (reflection generated; cache entry created under `~/.codex/sessions/reflection_cache/`)
- `reflect` (second run): OK (returns `status=cached`, `cached=true`)

### Captured artifacts

- `/tmp/codex_reflect_one.json` (reflection row + metadata)
- `/tmp/codex_reflect_one_cached.json` (cache reuse check)

## Follow-up Run: Full-Archive Index Build + Perf Snapshot

**Date run:** 2026-01-25 (UTC)

**Git HEAD tested:** `e5787ee`

### Commands executed

```bash
# Full archive index build (~400 sessions)
/usr/bin/time -p go run ./cmd/codex-sessions index build --include-most-recent --limit 0 --output json > /tmp/codex_index_build_full.json
go run ./cmd/codex-sessions index stats --output json > /tmp/codex_index_stats_full.json

# Perf snapshot (indexed vs scan)
/usr/bin/time -p go run ./cmd/codex-sessions search --query apply_patch --scope tools --per-message --max-results 20 --output json > /tmp/codex_search_index_perf.json
/usr/bin/time -p go run ./cmd/codex-sessions search --use-index=false --query apply_patch --include-most-recent --limit 50 --output json > /tmp/codex_search_scan_perf.json
```

### Results summary

- Full index build time (from `time -p`): ~124.45s
- Index size: ~120 MB (`~/.codex/sessions/session_index.sqlite`)
- `index stats` (after build):
  - sessions: 406
  - messages: 10869
  - tool_calls: 11646
  - tool_outputs: 0 (not indexed by default)
  - paths: 102187
  - errors: 1478
- Perf snapshot:
  - indexed search (tools scope, 20 hits): ~0.15s
  - scan search (substring, 50 sessions scanned): ~8.08s

### Captured artifacts

- `/tmp/codex_index_build_full.json` (index build rows, 406 sessions)
- `/tmp/codex_index_stats_full.json` (index stats after full build)
- `/tmp/codex_search_index_perf.json` (indexed search output)
- `/tmp/codex_search_scan_perf.json` (scan backend output)

## Usage Examples

```bash
# List projects and counts
go run ./cmd/codex-sessions projects --output table

# List the most recent 10 sessions (including most recent)
go run ./cmd/codex-sessions list --include-most-recent --limit 10 --output table

# Show a session by id (searches under sessions root)
go run ./cmd/codex-sessions show --session-id <uuid> --output table

# Search messages
go run ./cmd/codex-sessions search --query \"upload\" --include-most-recent --limit 50 --output table
```

## Related

- `ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/design-doc/02-design-go-conversation-parsing-indexing-and-glazed-cli.md`
- `ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/01-diary.md`
