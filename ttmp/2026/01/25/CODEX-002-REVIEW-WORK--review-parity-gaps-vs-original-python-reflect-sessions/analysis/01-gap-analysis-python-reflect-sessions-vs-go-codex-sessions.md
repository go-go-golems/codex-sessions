---
Title: 'Gap Analysis: Python reflect_sessions vs Go codex-sessions'
Ticket: CODEX-002-REVIEW-WORK
Status: active
Topics:
    - backend
    - chat
    - review
    - go
    - python
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/codex-sessions/reflect.go
      Note: Go reflect command
    - Path: internal/reflect
      Note: Go prompt/cache/codex/copy implementation
    - Path: internal/sessions
      Note: Go session parsing and selection
    - Path: scripts/cleanup_reflection_copies.py
      Note: Python cleanup utility
    - Path: scripts/parse_traces.py
      Note: Python trace report generator
    - Path: scripts/reflect_sessions.py
      Note: Python top-level reflection CLI
    - Path: scripts/reflect_sessions/output.py
      Note: Python output schema and human rendering
    - Path: scripts/reflect_sessions/prompt.py
      Note: Python prompt labels/keys/versioning
    - Path: scripts/session_io.py
      Note: Python reflection copy creation/prefix logic
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-25T12:19:36.747878824-05:00
WhatFor: ""
WhenToUse: ""
---


# Gap Analysis: Python `reflect_sessions` vs Go `codex-sessions`

## Executive Summary

The Go port (`codex-sessions`) now covers the core functionality of the original Python tool and extends it with additional query/extraction/indexing commands. The biggest remaining gaps are not “missing commands”, but **behavioral compatibility and polish**:

1. **Cache compatibility gap (largest practical impact)**: the Go `reflect` command currently uses a different `prompt_label` → `prompt_cache_key` mapping than the Python tool, so it will not reuse the Python tool’s keyed caches (`<session_id>-<prompt_key>.json`) for prompt presets. This is fixable by aligning the prompt label scheme to the Python behavior.
2. **Reflection-copy hygiene**: Python skips reflection copies by **content** (`is_reflection_copy`) and ships a cleanup script for orphaned copies. The Go tool currently filters by filename (`-copy`) but not by content prefix and has no cleanup command.
3. **Output/UX parity**: Python offers project grouping and `human` output style, plus an integrated `--list-projects` mode. Go offers richer row-based outputs via Glazed, but not the same grouping payload and “one JSON blob per run” schema.
4. **Execution model**: Python can run reflections in parallel (ThreadPoolExecutor) with `--sequential` as a safety switch. Go runs sequentially only.
5. **Trace tooling**: Python’s `parse_traces.py` produces a formatted Markdown artifact for inspection; Go has `export/show` but no equivalent “pretty trace report”.

This ticket documents these gaps and suggests concrete, incremental follow-ups.

## Scope

Compared components:

- Python reflection tool:
  - `scripts/reflect_sessions.py`
  - `scripts/reflect_sessions/*`
  - `scripts/cleanup_reflection_copies.py`
  - `scripts/parse_traces.py`
  - prompt files under `scripts/prompts/*`
- Go CLI:
  - `cmd/codex-sessions/*`
  - `internal/sessions/*`
  - `internal/indexdb/*`
  - `internal/reflect/*`
  - `scripts/prompts/prompts.go` (embedded prompt bundle)

## Inventory: Original Python Features (What exists today)

### Reflection pipeline

- Session selection semantics (filters + default skip-most-recent + default limit).
- Prompt selection:
  - preset (`--prompt-preset`)
  - file (`--prompt-file`)
  - inline (`--prompt-text`)
- Prompt version state:
  - default prompt version file under `scripts/prompts/reflection_version.json`
  - per-file prompt version file `*_version.json` next to the prompt file
  - inline prompt version file under `<cache_dir>/prompt_versions/`
- Cache:
  - keyed cache: `<cache_dir>/<session_id>-<prompt_key>.json`
  - legacy cache: `<cache_dir>/<session_id>.json` reused only for default prompt
  - refresh modes: `never|auto|always` with `auto` refreshing only for select staleness reasons
- Reflection generation:
  - create a session copy with new UUID + synced session_meta id
  - prefix first user message (with `## my request for codex:` handling)
  - run `codex exec ... resume <copy_id> -` with prompt on stdin
  - extract reflection from the last assistant message
  - delete the copy
- Execution model:
  - parallel by default for >1 session (ThreadPoolExecutor), capped by `DEFAULT_MAX_WORKERS`
  - `--sequential` to force serial execution

### Output/UX

- `--output-style human|json|json_extra_metadata`
- `--list-projects` (after filtering) to print project counts with a “current project” marker
- JSON output schema is a single object with `projects: [{project, sessions: [...]}, ...]`, plus optional top-level run metadata.

### Hygiene utilities

- `cleanup_reflection_copies.py` deletes reflection copies by **content detection** (`is_reflection_copy`), not by filename alone.
- `parse_traces.py` generates a Markdown “trace examples” report from JSONL with truncation and pretty formatting.

## Inventory: Go `codex-sessions` (What we have)

### Query/extraction (Go-only extensions)

- `projects` / `list` / `show` / `search` / `export`
- `index build` / `index stats` (SQLite + FTS)

### Reflection

- `codex-sessions reflect` supports:
  - selection (filters, explicit ids)
  - prompt preset/file/text
  - caching keyed by prompt selection + legacy fallback for default preset
  - refresh modes
  - `codex exec resume` execution and reflection extraction
  - `--dry-run` and `--extra-metadata` fields

## Gap Matrix (Python → Go parity)

### 1) Cache and prompt compatibility (high priority)

**Python behavior**

- `prompt_label` for presets is the *prompt file path* (e.g. `scripts/prompts/reflection.md`).
- `prompt_cache_key = sha256(prompt_label)[:12]`.
- Cache entry path: `<cache_dir>/<session_id>-<prompt_cache_key>.json`.

**Go behavior (current)**

- For presets, prompt label is `preset:<name>` (e.g. `preset:reflection`).
- This yields a different cache key than Python, so Go will create a separate cache file even for the same prompt content.

**Impact**

- If you previously used the Python tool, Go will not reuse the keyed caches for presets, so your first Go run will regenerate reflections unnecessarily (expensive) and create duplicates.

**Recommended fix**

- Make Go compute `prompt_label` exactly like Python:
  - preset label: a canonical path string like `scripts/prompts/<preset>.md` (or its absolute path; pick one canonical form and stick to it)
  - inline label: `inline:<hash8>`
  - file label: resolved prompt file path
- Compute prompt key using the same `sha256(label)[:12]`.
- Keep legacy cache reuse only for the default prompt (same condition as Python: “prompt path matches the default prompt file”).

### 2) Reflection-copy filtering and cleanup (high priority)

**Python behavior**

- Session discovery loads lines and skips reflection copies using `is_reflection_copy(lines, prefix)`.
- Separate cleanup tool removes leftover copies based on content detection.

**Go behavior (current)**

- Discovery excludes filenames containing `-copy`, but does not detect “[SELF-REFLECTION]” copies by content.
- No cleanup command exists.

**Recommended fix**

- Add content-based reflection-copy detection to Go session discovery:
  - when scanning rollouts, read only enough to find the first user message(s) and check for the prefix (same logic as Python’s `session_io.is_reflection_copy`).
  - add a flag `--include-reflection-copies` for debugging, default false.
- Add a `codex-sessions cleanup reflection-copies` command (Go analogue of `cleanup_reflection_copies.py`) that:
  - lists matching files (`--dry-run`)
  - deletes them when not dry-run

### 3) Output schema + grouping parity (medium priority)

**Python behavior**

- Outputs a single object grouped by project:
  - `projects: [{project, sessions:[{conversation_started_at, conversation_updated_at, reflection_created_at, reflection, ...}]}]`
- `human` output style for readability.

**Go behavior (current)**

- Emits one row per session (Glazed).
- Users can render JSON/CSV/table, but it’s not the same “grouped run payload”.

**Recommended fix**

- Add a `codex-sessions reflect export` (or `--format python-json`) mode that emits the Python-compatible grouped JSON schema.
  - This is useful for backwards compatibility with existing consumers and for “one file per run” artifacts.
- Add a convenience `--list-projects` flag on `reflect` (or keep separate `projects` command, but parity users will expect it on `reflect`).

### 4) Parallel reflection execution (medium priority)

**Python behavior**

- Parallel by default (ThreadPoolExecutor) up to `DEFAULT_MAX_WORKERS`.
- `--sequential` flag available.

**Go behavior (current)**

- Sequential only.

**Recommended fix**

- Add parallelism with explicit guardrails:
  - `--max-workers` with a conservative default (e.g. 4)
  - `--sequential` as an alias to `--max-workers 1`
  - preserve output ordering by started_at to keep results stable

### 5) Trace tooling (lower priority)

**Python behavior**

- `parse_traces.py` produces a Markdown report emphasizing nested payloads, truncated multi-line strings, etc.

**Go behavior (current)**

- `export` can produce a normalized JSON document, and `show` can render raw/facets, but there’s no “human-readable trace report” generator.

**Recommended fix**

- Add `codex-sessions traces` that emits Markdown:
  - choose N sessions
  - extract representative response_items (tools/messages)
  - apply truncation rules similar to `parse_traces.py`

## Proposed Follow-up Work (Concrete)

If we want maximum parity and a clean migration path, the next implementation ticket could:

1. Align prompt labels/cache keys to Python’s scheme (so caches are shared).
2. Add content-based reflection copy detection + cleanup command.
3. Add a Python-compatible JSON export mode for `reflect`.
4. Add parallelism (`--max-workers`, `--sequential`).
5. Add an optional “traces report” command.

## Notes / Risks

- **Caching and prompt versioning**: Python updates prompt version state files; in Go, we should be careful not to “mutate repo files” unexpectedly. Storing prompt version state under the cache dir is safer operationally, but changes parity. If parity is more important than safety, we can emulate Python’s version file behavior more closely.
- **FTS search semantics vs substring scan**: Go now supports both backends; make sure documentation is explicit that they behave differently.
- **Secrets**: indexing tool outputs is opt-in; reflection caches already store prompts and reflections and may include sensitive content.
