---
Title: 'Design: Go Conversation Parsing, Indexing, and Glazed CLI'
Ticket: CODEX-001-PORT-GO
Status: active
Topics:
    - backend
    - chat
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: references/README.md
      Note: Baseline behavior + cache/output expectations to preserve
    - Path: references/cli.md
      Note: Existing CLI surface area to match/extend
    - Path: scripts/parse_traces.py
      Note: Hints for traversing nested payloads (text/arguments/output)
    - Path: scripts/reflect_sessions/codex.py
      Note: Parity reflection generation via codex binary
    - Path: ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/02-glazed-notes-build-first-command.md
      Note: Glazed patterns (decode settings + emit rows)
ExternalSources: []
Summary: Proposed Go architecture for parsing Codex session JSONL, extracting queryable data, optional indexing, and a Glazed-based CLI suite (tables/JSON/CSV/YAML).
LastUpdated: 2026-01-23T23:49:15.12187319-05:00
WhatFor: ""
WhenToUse: ""
---


# Design: Go Conversation Parsing, Indexing, and Glazed CLI

## Executive Summary

Build a Go CLI suite (tentatively `codex-sessions`) that reads Codex session JSONL histories from a local sessions root (default `~/.codex/sessions`), parses them into a tolerant internal event model, extracts multiple “facets” (metadata, message timeline, tool calls, paths, commands, errors, etc.), and supports multiple query strategies:

1. **Metadata scan** (fast, no index): list/filter sessions by project/time/id/title/updated_at.
2. **On-the-fly search** (no index): grep-like text search over message text and selected payload fields.
3. **Indexed search** (optional SQLite + FTS): fast full-text search and structured filters across many sessions.
4. **Reflection generation** (optional, parity): run `codex exec resume` on a temporary session copy and cache results per session+prompt.

All user-facing commands should be implemented with Glazed so the same command yields table/JSON/CSV/YAML without format-specific code.

## Problem Statement

The current Python CLI is reflection-focused: it can filter sessions and produce a single paragraph “reflection” per session, but it does not provide a general-purpose way to query past conversations (e.g., “show sessions mentioning a file”, “list tool invocations”, “search for a specific error”, “export a normalized message timeline”).

We need a Go port that:

- Keeps reflection parity where needed, but
- Adds robust parsing + extraction + query capabilities, and
- Offers a coherent CLI suite with structured outputs (Glazed).

## Proposed Solution

### Component overview

```
sessions root (rollout-*.jsonl)
        |
        v
 discover -> parse JSONL -> normalize events/messages -> extract facets
        |                        |                         |
        |                        |                         +--> rows (Glazed)
        |                        |
        |                        +--> optional SQLite index (sessions/messages/events)
        |
        +--> optional reflection (copy+resume codex) + cache
```

### Goals

- **Tolerant parsing**: handle unknown line types / payload shapes gracefully.
- **Streaming**: parse large JSONL without loading the entire file into memory.
- **Separation of concerns**: raw parsing vs. normalization vs. extraction vs. indexing.
- **Multi-output CLI**: every command produces rows; Glazed handles formatting.

### Non-goals (initially)

- Perfect schema knowledge of every Codex log line type; unknowns should be preserved as raw JSON.
- Replacing Codex’s own session format; we only read it and optionally create temporary copies.
- Remote access / network features.

## Design Decisions

### 1) Internal event model: “raw envelope + best-effort decode”

Codex JSONL lines appear to have at least:

- `type` (string)
- `payload` (object) for many line types
- `timestamp` (string) for some line types

Design:

- Parse each line into a `RawLine{ Type string; Timestamp *time.Time; Payload json.RawMessage; Raw json.RawMessage }`.
- Use a registry of decoders for known `type`s (`session_meta`, `event_msg`, `response_item`, …).
- For unknown types, retain `Raw` and optionally surface minimal metadata (line number, type, timestamp if present).

This mirrors the Python approach (decode only what you need) but makes “unknowns” explicit and queryable.

### 2) Normalized message timeline

Provide a stable “conversation” representation that downstream commands can rely on, regardless of whether the original log used `event_msg` or `response_item`:

- `Message{ Role user|assistant|system; Text string; Segments []Segment; Timestamp time.Time; Source string; ToolCalls []ToolCall; ToolOutputs []ToolOutput }`

Notes:

- For `response_item` messages, extract `input_text`/`output_text` segments (as Python does).
- Additionally scan payload trees for common fields used in traces: `text`, `arguments`, `output`, `reasoning` (pattern suggested by `scripts/parse_traces.py`).
- Keep both “best text” and “raw segments” so output commands can be strict or lossy depending on flags.

### 3) Facet extraction (multiple query paths)

We want different “ways to query past conversations” without always relying on an LLM reflection. Proposed facets:

- **Session metadata**: session_id, started_at, updated_at, cwd/project, title, file path.
- **Message text**: normalized per role; optional concatenation per turn.
- **Tools**:
  - tool name (e.g., `functions.shell_command`)
  - tool arguments (parsed if JSON, else raw string)
  - tool output (truncated for indexing; full available for export)
- **Paths**:
  - absolute/relative paths detected in messages and tool args/outputs
  - repo-relative normalization when `cwd` is inside a git repo (optional enhancement)
- **Commands**:
  - shell commands detected from tool calls and/or message blocks
- **Errors**:
  - stderr snippets, exit codes, stack traces (best-effort detection)
- **Prompts**:
  - prompt preset/file/inline label used for reflection generation (parity)

Extraction should be configurable:

- `--extract minimal` (metadata + titles)
- `--extract timeline` (messages)
- `--extract tools` (tool calls/outputs)
- `--extract all` (everything above)

### 4) Optional indexing: SQLite + FTS

To support fast interactive query across large session archives, add an optional index:

- Storage location: default `~/.codex/sessions/session_index.sqlite` (override with `--index-path`), or store under an explicit cache dir.
- Tables:
  - `sessions(session_id PRIMARY KEY, started_at, updated_at, project, cwd, title, source_path)`
  - `messages(id INTEGER PK, session_id, ts, role, text, source_type)`
  - `tool_calls(id INTEGER PK, session_id, ts, tool_name, arguments_json, arguments_text)`
  - `tool_outputs(id INTEGER PK, session_id, ts, tool_name, output_text)`
  - `events(id INTEGER PK, session_id, ts, type, raw_json)` (optional)
- FTS:
  - `messages_fts` (FTS5) over `text` with `session_id` and `ts` as stored columns.

Index build strategy:

- Streaming parse each JSONL.
- Upsert into `sessions` (use `updated_at` to decide whether reindex is needed).
- Insert messages/tools/events.
- Use transactions per session file for performance.

### 5) Reflection generation: parity via “copy + codex resume”

Keep the same safety properties as the Python approach:

- Never mutate original JSONL.
- Create a temporary copy with a new UUID and a visible `[SELF-REFLECTION] ` prefix on the first user message.
- Run `codex exec resume <copy_id> -` with `--sandbox read-only --ask-for-approval never` by default.
- Extract reflection as “last assistant output_text” in the copied JSONL.
- Delete the copy.

Cache:

- Store per `(session_id, prompt_cache_key)` exactly as the Python tool does today, so caches remain compatible if desired.

### 6) Glazed CLI: commands as row producers

Each command implements a Glazed `GlazeCommand` that:

- Defines flags/parameters via Glazed schema/fields.
- Decodes settings via `values.DecodeSectionInto(...)` (per `glaze help build-first-command`).
- Adds rows (`types.Row`) to the processor.

The CLI should never print ad-hoc output by default; any “pretty timeline” output should either:

- be implemented as structured rows (one row per message, with columns like `ts`, `role`, `text`), or
- live behind an explicit `--format raw` / `--render markdown` opt-in.

## CLI Suite (Proposed)

Command naming is flexible; examples assume `codex-sessions` as the binary and `sessions` as the conceptual domain.

### `codex-sessions list`

- Purpose: fast session listing (metadata scan or index-backed).
- Columns: `session_id`, `project`, `started_at`, `updated_at`, `title`, `source_path`.
- Key flags: `--project`, `--since`, `--until`, `--limit`, `--include-most-recent`, `--sessions-root`, `--index-path`, `--use-index`.

### `codex-sessions show`

- Purpose: show a single session.
- Modes: `--view timeline|tools|raw`
- Emits rows for each message/tool event so Glazed can output JSON/CSV.

### `codex-sessions export`

- Purpose: export normalized JSON (or NDJSON) for downstream processing.
- Flags: `--session-id`, `--extract minimal|timeline|tools|all`, `--truncate-bytes`, `--include-raw`.

### `codex-sessions search`

- Purpose: query across sessions.
- Backends:
  - default: index-backed (FTS) if index exists
  - fallback: streaming scan if no index
- Flags: `--query`, `--project`, `--since`, `--until`, `--role`, `--tool`, `--path`.

### `codex-sessions index build`

- Purpose: build or refresh SQLite index.
- Flags: `--sessions-root`, `--index-path`, `--refresh-mode never|auto|always` (for index rebuild behavior).

### `codex-sessions stats`

- Purpose: derived metrics (counts per project/day, top tools, most common errors).
- Can be index-backed for speed; streaming fallback possible.

### `codex-sessions reflect`

- Purpose: generate reflections (parity) for selected sessions.
- Reuse Python-compatible flags: `--prompt-preset`, `--prompt-file`, `--prompt-text`, `--refresh-mode`, `--prefix`, `--codex-path`, `--codex-sandbox`, `--codex-approval`, `--codex-timeout-seconds`.

## Alternatives Considered

### No index at all

Pros: simpler, fewer moving parts. Cons: queries become slow on large archives; repeated scans are expensive.

Decision: support both; start with streaming scan commands, then add optional index for speed.

### BoltDB/badger instead of SQLite

Pros: pure KV, simple embedding. Cons: poorer ad-hoc querying, no FTS equivalent without extra work.

Decision: SQLite is the best “queryable local store” baseline and supports FTS.

### Parse Codex stdout instead of reading the copied JSONL

Pros: avoids reliance on session log mutation. Cons: requires strong guarantees about stdout/stderr, especially if Codex outputs non-content frames or debug logs.

Decision: keep parity behavior (read JSONL) initially; optionally add a “stdout-mode” if Codex offers a stable machine-readable mode.

## Implementation Plan

1. Define Go packages:
   - `sessions/discover`: find session files under root
   - `sessions/jsonl`: streaming line reader + tolerant envelope parser
   - `sessions/normalize`: message timeline builder + text extraction
   - `sessions/extract`: facets (tools/paths/errors)
   - `sessions/index`: SQLite schema + build/query helpers
   - `sessions/reflect`: copy/prefix/resume + cache semantics
   - `cmd/...`: Glazed commands
2. Implement `list`, `show`, and `export` (streaming-only) first.
3. Add SQLite index build + index-backed search.
4. Port reflection generation + caching/prompt presets.
5. Validate against a real `~/.codex/sessions` archive and document edge cases.

## Open Questions

- What is the authoritative schema for Codex session JSONL (and how often does it change)?
- How should we treat “projects” when `cwd` is missing or misleading?
- Do we need to extract non-text content (images, audio) or just metadata?
- How should we handle privacy/safety when indexing tool outputs that may contain secrets?

## References

- `ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/design-doc/01-analysis-current-python-session-reflection-cli.md`
- `ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/02-glazed-notes-build-first-command.md`

## Design Decisions

<!-- Document key design decisions and rationale -->

## Alternatives Considered

<!-- List alternative approaches that were considered and why they were rejected -->

## Implementation Plan

<!-- Outline the steps to implement this design -->

## Open Questions

<!-- List any unresolved questions or concerns -->

## References

<!-- Link to related documents, RFCs, or external resources -->
