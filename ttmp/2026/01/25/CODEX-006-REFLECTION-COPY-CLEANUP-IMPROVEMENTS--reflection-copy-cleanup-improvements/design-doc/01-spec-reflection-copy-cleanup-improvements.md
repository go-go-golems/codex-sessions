---
Title: 'Spec: Reflection Copy Cleanup Improvements'
Ticket: CODEX-006-REFLECTION-COPY-CLEANUP-IMPROVEMENTS
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
LastUpdated: 2026-01-25T14:01:28.157327836-05:00
WhatFor: ""
WhenToUse: ""
---

# Spec: Reflection Copy Cleanup Improvements

## Executive Summary

Improve `codex-sessions cleanup reflection-copies` so it’s safer and more usable at scale:

- Add an explicit “trash” mode (move to a ticketed trash directory instead of deleting).
- Add selection filters (`--project`, `--since`, `--until`) to target specific ranges.
- Emit richer output (`size_bytes`, `dest_path`) so users can audit what happened and how much disk was reclaimed.

This extends CODEX-003 without changing the default safety posture (`--dry-run=true`).

## Problem Statement

The existing cleanup command works but has limitations:

- It only supports “dry-run list” vs “delete immediately”; there’s no reversible/safe cleanup path.
- It has no filters, so running it against large archives can be slow/noisy and makes it harder to target recent incidents.
- The output lacks basic operational details (file size, destination when moved), which makes it hard to audit or estimate impact.

## Proposed Solution

### CLI UX

Extend `codex-sessions cleanup reflection-copies` with:

- `--since`, `--until`: ISO date or datetime (reuse existing parsing helpers).
- `--project`: derived project label filter (cwd basename).
- `--mode` (choice): `delete|trash` (default `delete`), where:
  - `delete` removes files (only when `--dry-run=false`)
  - `trash` moves files into `<sessions-root>/trash/reflection-copies/<YYYY>/<MM>/<DD>/` (only when `--dry-run=false`)

Add output columns:

- `size_bytes`: source file size
- `dest_path`: empty for dry-run and delete; set for trash mode

### Implementation

- Extend `sessions.CleanupReflectionCopiesOptions` to include:
  - optional `Since`/`Until` timestamps
  - optional `Project` string
  - `Mode` (delete|trash)
- Add `sessions.MoveToTrash(srcPath, trashRoot)` helper:
  - create destination dir if missing
  - avoid collisions by suffixing a short timestamp (or random suffix) if necessary
  - return dest path for output
- Update command row output to include `size_bytes` and `dest_path`.

## Design Decisions

- Keep default `--dry-run=true` (safe by default).
- Use “trash” as a first-class action rather than relying on OS trash semantics (portable and predictable).
- Keep filters optional; default behavior remains “scan all”.

## Alternatives Considered

- Use system trash APIs: inconsistent across platforms, additional dependencies, harder to test.
- Add an interactive prompt: not automation-friendly; `--dry-run` + explicit `--dry-run=false` already provides safety.
- Add indexed copy markers: useful later, but filters + trash address current operational needs with minimal complexity.

## Implementation Plan

1. Extend `CleanupReflectionCopiesOptions` and result struct for size/dest.
2. Implement trash mode helper and wire it through `CleanupReflectionCopies`.
3. Add filters and tests for filter + trash behavior.
4. Update CLI command flags and output.
5. Add smoke test report and upload ticket bundle to reMarkable.

## Open Questions

- Should trash mode preserve original subdirectory structure exactly, or is a date-based bucket sufficient?
- Should we add `--max-bytes` / `--max-files` safety caps beyond the existing `--limit`?

## References

- CODEX-003 (baseline behavior): `codex-sessions cleanup reflection-copies` and content-based detection.
