---
Title: 'Spec: Parallel Reflection Execution'
Ticket: CODEX-005-PARALLEL-REFLECT
Status: active
Topics:
    - backend
    - chat
    - go
    - performance
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-25T12:41:57.804010399-05:00
WhatFor: ""
WhenToUse: ""
---

# Spec: Parallel Reflection Execution

## Executive Summary

Add parallel execution to `codex-sessions reflect` (with a conservative default worker count and a sequential safety option) to match the original Python tool’s behavior and significantly reduce wall-clock time when generating many reflections.

## Problem Statement

The Go `reflect` command currently runs sequentially. For large batches, this is slow, and it regresses parity with the Python tool which uses a worker pool by default (with `--sequential` to disable parallelism).

## Proposed Solution

Add flags:

- `--max-workers <n>` (default 4)
- `--sequential` (alias for `--max-workers 1`)

Execution model:

- Determine selected sessions using existing selection semantics.
- Perform reflection generation concurrently in a bounded worker pool.
- Preserve stable output ordering (same order as selection) by collecting results into a slice and emitting rows after workers complete (Glazed processors are not assumed thread-safe).

Safety:

- Each session already uses a unique copy filename/id, so copies won’t collide.
- Cache writes must be per-session+prompt key; still, write operations should be atomic (write temp + rename) or guarded if needed.
- Provide good error reporting per session row (status + error).

## Design Decisions

- Bounded worker pool over “one goroutine per session” to avoid resource spikes.
- Emit rows after work completes to avoid thread-safety issues with Glazed processors.
- Keep defaults conservative (`max-workers=4`) since multiple Codex runs can be resource-heavy.

## Alternatives Considered

- Always sequential: too slow for large runs, diverges from Python.
- Unbounded parallelism: risks CPU/memory spikes and Codex contention.

## Implementation Plan

1. Add `--max-workers`/`--sequential` flags to `reflect`.
2. Refactor reflect generation into a per-session function returning a result struct (row fields).
3. Implement worker pool with ordering preservation.
4. Add tests for ordering and worker cap (unit-level where possible).
5. Add smoke test + performance note in ticket.

## Open Questions

- Should we add `--rate-limit` (sleep between launches) to be nicer to Codex?
- Should we add cancellation support per-session when the overall context is canceled?

## References

- Python parallel behavior: `scripts/reflect_sessions.py` (`ThreadPoolExecutor`, `--sequential`)
