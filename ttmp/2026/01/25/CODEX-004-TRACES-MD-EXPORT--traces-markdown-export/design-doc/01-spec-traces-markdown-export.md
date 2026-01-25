---
Title: 'Spec: Traces Markdown Export'
Ticket: CODEX-004-TRACES-MD-EXPORT
Status: active
Topics:
    - backend
    - chat
    - go
    - docs
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-25T12:41:57.561312699-05:00
WhatFor: ""
WhenToUse: ""
---

# Spec: Traces Markdown Export

## Executive Summary

Add a Go CLI command to export “trace examples” as Markdown, similar to `scripts/parse_traces.py`, to support debugging and human review of Codex session JSONL payloads. The output should be deterministic, truncated, and readable (multiline strings rendered safely for PDF export).

## Problem Statement

The Go CLI has powerful `show` and `export` commands, but there is no “one-shot” command that produces a curated, human-readable Markdown report of representative trace payloads for a set of sessions. The Python repo has `parse_traces.py`, which is often used to understand schema drift and tool payload shapes.

## Proposed Solution

Add a new command:

- `codex-sessions traces md`

Inputs:

- `--sessions-root` (default `~/.codex/sessions`)
- session selection options:
  - `--session-id` / `--session-ids`
  - or `--project`, `--since`, `--until`, `--limit`, `--include-most-recent`

Output:

- `--output` path (default `trace_examples.md`)
- optionally `--stdout` or allow `--output -`

Content:

- For each selected session:
  - session header (id/project/title/started/updated/path)
  - N representative `response_item` payloads (messages, custom_tool_call, custom_tool_call_output)
  - extracted snippets (texts, arguments, output, errors) with truncation

Formatting requirements:

- Deterministic ordering (by session started_at then by line number).
- Truncation knobs:
  - `--max-str-len` (default 2000)
  - `--max-list-len` (default 10)
  - `--entries-per-file` (default 20)
- Multiline strings should be rendered in fenced code blocks (avoid LaTeX `\n` pitfalls).

## Design Decisions

- Use fenced code blocks for payload excerpts to keep Pandoc/LaTeX stable.
- Reuse Go parsing helpers (`WalkJSONLLines`, `ExtractMessages`, facets) for metadata and filtering, but read raw JSON for trace excerpts.
- Prefer a single Markdown file output (easy to attach/upload).

## Alternatives Considered

- Rely only on `export --shape document`: too large/noisy for trace inspection; lacks curated excerpts.
- Emit JSON-only artifacts and leave rendering to users: less useful for quick understanding and for PDF/reMarkable workflows.

## Implementation Plan

1. Implement a small “trace excerpt” extractor that:
   - walks JSONL
   - captures representative `response_item` lines
   - truncates strings/lists in nested payloads
2. Add `codex-sessions traces md` command:
   - selection semantics aligned with `list`/`search`
   - output writing
3. Add unit tests around rendering/truncation (small fixtures).
4. Add ticket smoke-test + reMarkable upload.

## Open Questions

- Should we include “reasoning” fields when present, or avoid them for privacy/noise?
- Do we want multiple output modes: `md` vs `json`? (Start with `md` only.)

## References

- Python trace generator: `scripts/parse_traces.py`
