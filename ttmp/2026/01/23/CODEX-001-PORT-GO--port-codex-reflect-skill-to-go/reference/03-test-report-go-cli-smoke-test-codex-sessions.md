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
    - Path: cmd/codex-sessions/list.go
      Note: List command
    - Path: cmd/codex-sessions/main.go
      Note: CLI entrypoint
    - Path: cmd/codex-sessions/projects.go
      Note: Projects command
    - Path: cmd/codex-sessions/search.go
      Note: Search command
    - Path: cmd/codex-sessions/show.go
      Note: Show command
    - Path: internal/sessions
      Note: Parsing and normalization
ExternalSources: []
Summary: Smoke test results for the Go Glazed CLI against the real ~/.codex/sessions archive (projects/list/show/search).
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

### Issues / gaps observed

1. **Multiline text in table output**: `search` snippets and `show` message text can contain newlines, which makes table output hard to read and can explode output size.
   - Suggested fix: add an option like `--single-line` that replaces newlines with literal `\\n` or collapses whitespace for display columns.

2. **Reflection-copy filtering gap**: discovery currently excludes filenames containing `-copy`, but does not detect “[SELF-REFLECTION]” copies by content (the Python tool does).
   - Suggested fix: implement a fast prefix check against early user messages, and optionally skip those files during discovery/list/search unless explicitly requested.

3. **Search semantics**: `--limit` currently limits the number of sessions scanned (matching Python semantics), not “number of matches returned”. This is fine but should be documented in `--help` strings.

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
