---
Title: 'Spec: Traces Enhancements'
Ticket: CODEX-007-TRACES-ENHANCEMENTS
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
LastUpdated: 2026-01-25T14:09:57.956711052-05:00
WhatFor: ""
WhenToUse: ""
---

# Spec: Traces Enhancements

## Executive Summary

Enhance `codex-sessions traces md` with better triage ergonomics:

- Include entry metadata (line number + timestamp + tool name when available).
- Allow filtering to specific `response_item.payload.type` values.
- Optionally include a truncated “raw payload” rendering for schema debugging.

## Problem Statement

The current Markdown export is useful, but it can still be noisy and lacks context:

- It’s hard to correlate an excerpt back to the source JSONL without line numbers/timestamps.
- There’s no way to focus on only tool calls/results vs message payloads.
- Sometimes you need to see the raw payload shape (beyond extracted text/arguments/output) to debug schema drift.

## Proposed Solution

### CLI UX

Extend `codex-sessions traces md` with:

- `--include-entry-metadata` (default: true): add per-entry `line_no`, `timestamp`, and `tool_name` when present.
- `--payload-types` (comma-separated): only include response items whose `payload.type` matches one of these values.
- `--include-raw-payload` (default: false): add a `**payload**` fenced section rendering the truncated payload object.

### Rendering behavior

- Metadata is shown as short bullet lines immediately under each entry header.
- Payload rendering uses deterministic ordering (sorted map keys) and respects existing truncation knobs.

## Design Decisions

- Keep Markdown as the primary output target (fast human review + good reMarkable UX).
- Keep truncation default values unchanged to avoid exploding report size.
- Avoid adding more output files/modes in this ticket; focus on usability within the existing report format.

## Alternatives Considered

- Export to JSON and render externally: less convenient; Markdown is the primary workflow here.
- Include metadata always without a flag: might be noisy for some users; make it configurable.

## Implementation Plan

1. Extend `internal/tracesmd.Options` and renderer to support metadata, filtering, and raw payload sections.
2. Add CLI flags and wire them into `internal/tracesmd`.
3. Add unit tests (filtering + metadata + raw payload).
4. Add smoke test report and upload bundle to reMarkable.

## Open Questions

- Should `--payload-types` accept aliases (e.g., `tool` meaning `tool_call` + `tool_result`)?
- Should raw payload rendering include the full outer `response_item` envelope or payload only? (Proposal: payload only.)

## References

- CODEX-004 baseline: `codex-sessions traces md` initial Markdown exporter.
