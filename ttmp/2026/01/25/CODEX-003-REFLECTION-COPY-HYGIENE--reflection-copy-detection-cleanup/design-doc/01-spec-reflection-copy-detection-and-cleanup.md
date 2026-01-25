---
Title: 'Spec: Reflection Copy Detection and Cleanup'
Ticket: CODEX-003-REFLECTION-COPY-HYGIENE
Status: active
Topics:
    - backend
    - chat
    - go
    - cleanup
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-25T12:41:57.286347888-05:00
WhatFor: ""
WhenToUse: ""
---

# Spec: Reflection Copy Detection and Cleanup

## Executive Summary

Add robust detection of self-reflection “copy” sessions and a cleanup command to remove orphaned copies. The goal is to match the original Python tool’s behavior (content-based detection via the `[SELF-REFLECTION] ` prefix) rather than relying on filename heuristics alone.

## Problem Statement

The Go CLI currently filters likely reflection copies mainly by filename patterns (e.g., `-copy`) and does not provide a dedicated cleanup utility. In practice:

- Reflection copies may not contain `-copy` in the filename.
- A crash or interruption can leave behind reflection copy files, polluting discovery/indexing/search and wasting disk.
- Users want a safe, explicit tool to list and delete reflection copies.

## Proposed Solution

### A) Content-based “reflection copy” detection

Implement a Go analogue of Python’s `session_io.is_reflection_copy(lines, prefix)`:

- Read a session JSONL file and detect whether the *first user message* is prefixed with `[SELF-REFLECTION] `.
- Must handle both representations:
  - `event_msg.payload.type=user_message` with `payload.message`
  - `response_item.payload.type=message, role=user` with `content[].type=input_text`
- Keep the scan cheap:
  - stop as soon as we find the first user message(s)
  - do not load the full file

Add a flag on discovery/list/search/index build:

- `--include-reflection-copies` (default: false)

Behavior:

- When false, skip reflection copies during discovery and downstream commands.
- When true, include them (for debugging/forensics).

### B) Cleanup command

Add a new command:

- `codex-sessions cleanup reflection-copies`

Flags:

- `--sessions-root` (default `~/.codex/sessions`)
- `--prefix` (default `[SELF-REFLECTION] `)
- `--dry-run` (default true or false; decide in implementation)
- `--limit` optional safety limit
- `--include-most-recent` N/A (cleanup is not time-filtered unless we add `--since/--until`)

Output:

- One row per matched file with:
  - `path`, `session_id` (best-effort), `project` (best-effort), `status` (`would_delete|deleted|error`), `error`

Safety:

- Default to `--dry-run` to avoid accidental deletion.
- Only delete when `--dry-run=false`.

## Design Decisions

- Prefer **content-based detection** over filename heuristics to match Python behavior and avoid false negatives.
- Make cleanup **explicit** and **safe by default** (`--dry-run`).
- Keep detection cheap by scanning for the first user message rather than parsing the entire JSONL.

## Alternatives Considered

- Only filename heuristics (`-copy`): insufficient; misses real-world copies.
- “Delete any session created by reflect”: not reliably encoded in JSONL; the prefix is the stable marker.
- Store “reflection copy” marker in a sidecar file: would require mutations and doesn’t help for existing copies.

## Implementation Plan

1. Add `sessions.IsReflectionCopy(path, prefix)` (best-effort; streaming scan).
2. Update session discovery and all commands that use it (`list`, `search`, `index build`, etc.) to skip reflection copies unless `--include-reflection-copies`.
3. Add `cleanup reflection-copies` command with `--dry-run` and row output.
4. Add unit tests with small JSONL fixtures for:
   - event-based prefix detection
   - response_item-based prefix detection
   - mixed sessions (both representations)
5. Add ticket smoke-test section + reMarkable export.

## Open Questions

- Should `cleanup reflection-copies` support `--since/--until` filters?
- Should we treat sessions with *either* representation prefixed as a copy, or require both when both exist?

## References

- Python reference implementation: `scripts/session_io.py` (`is_reflection_copy`, `prefix_first_user_message`)
- Python cleanup tool: `scripts/cleanup_reflection_copies.py`
