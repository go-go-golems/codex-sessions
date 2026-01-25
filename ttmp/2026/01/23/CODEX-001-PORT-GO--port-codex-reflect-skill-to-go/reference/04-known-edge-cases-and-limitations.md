---
Title: Known Edge Cases and Limitations
Ticket: CODEX-001-PORT-GO
Status: active
Topics:
    - backend
    - chat
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/codex-sessions/index_build.go
      Note: Index build defaults and flags
    - Path: cmd/codex-sessions/search.go
      Note: Index vs scan semantics
    - Path: internal/indexdb/schema.go
      Note: Index schema and FTS tables
    - Path: internal/sessions/facets.go
      Note: Facet extraction heuristics
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-25T09:29:50.618657755-05:00
WhatFor: ""
WhenToUse: ""
---


# Known Edge Cases and Limitations

## Goal

Document the current limitations and “sharp edges” of the Go port so users understand what to trust, what is heuristic, and what to do when behavior differs from expectations.

## Context

This Go port reads Codex’s local JSONL session archive under `~/.codex/sessions` and derives “normalized” views (timeline, facets, export shapes) plus an optional SQLite/FTS index. The upstream JSONL format is not guaranteed stable, so most parsing and extraction is best-effort.

## Quick Reference

### General

- **Heuristic vs authoritative**:
  - `session_meta` / timestamps are authoritative when present and parseable.
  - Title derivation, path detection, and error signals are heuristic.
- **Most recent session skipped by default**: `list`, `search`, and `index build` skip the newest started_at session unless `--include-most-recent` is set.
- **Glazed JSON output with zero rows**: some runs may emit no bytes (not `[]`) when there are no matching rows.

### Facets (tools/paths/errors/texts)

- Tool calls/outputs are extracted from known shapes (notably `response_item.payload.type=custom_tool_call` and `custom_tool_call_output`) and correlated via `call_id`.
- Nested text collection scans for keys named `text` and may include non-message text fields (useful for debugging, but noisy).
- Path extraction is regex-based and can produce false positives (especially for short relative paths).
- Error extraction is regex-based and can miss structured errors or produce false positives on log-like text.

### Search backends (important semantics)

| Backend | Trigger | Matching semantics | When to use |
|---|---|---|---|
| `backend=index` | `--use-index` + index exists + not `--case-sensitive` | SQLite FTS5 **token** matching (not substring) | Fast full-archive search |
| `backend=scan` (no column) | index missing, `--use-index=false`, or `--case-sensitive=true` | **substring** search on normalized message text | Exact substring recall or case-sensitive matching |

Notes:
- FTS queries like `foo bar` behave like AND across tokens; punctuation and case are normalized by the tokenizer.
- For substring-like needs (e.g., partial file path fragments), the streaming scan backend is often more intuitive.

### Indexing

- Default index path: `<sessions-root>/session_index.sqlite` (usually `~/.codex/sessions/session_index.sqlite`).
- Index build is incremental based on `conversation_updated_at` (reindexes sessions whose updated_at increased).
- Index build uses **one transaction per session** so partial failures don’t corrupt unrelated sessions.
- `--include-tool-outputs` is **off by default** (reduces index size and avoids persisting potentially sensitive output beyond what’s already in JSONL; you can opt in).

### Reflection parity (not implemented yet)

- The Go command `codex-sessions reflect` is not implemented yet.
- The planned parity behavior (from the Python tool) is:
  - create a temporary JSONL copy with a new id
  - prefix first user message with `[SELF-REFLECTION] `
  - run `codex exec resume <copy_id> -` with stdin prompt
  - extract last assistant response
  - delete copy
  - cache to `<sessions-root>/reflection_cache/<session_id>-<prompt_key>.json`

## Usage Examples

```bash
# Build a small index sample (5 sessions) and inspect stats
go run ./cmd/codex-sessions index build --include-most-recent --limit 5 --output table
go run ./cmd/codex-sessions index stats --output table

# Indexed search (fast): tools scope, return 5 matches
go run ./cmd/codex-sessions search --query apply --scope tools --per-message --max-results 5 --output table

# Streaming substring search (intuitive for partial strings)
go run ./cmd/codex-sessions search --use-index=false --query \"rollout-\" --include-most-recent --limit 50 --output table
```

## Related

- `ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/design-doc/01-analysis-current-python-session-reflection-cli.md`
- `ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/design-doc/02-design-go-conversation-parsing-indexing-and-glazed-cli.md`
- `ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/03-test-report-go-cli-smoke-test-codex-sessions.md`
