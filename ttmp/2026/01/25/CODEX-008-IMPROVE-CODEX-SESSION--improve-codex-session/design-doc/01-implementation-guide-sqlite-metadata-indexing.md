---
Title: 'Implementation Guide: SQLite Metadata & Indexing'
Ticket: CODEX-008-IMPROVE-CODEX-SESSION
Status: active
Topics:
    - codex
    - performance
    - docs
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: codex-sessions/cmd/codex-session/list.go
      Note: Default list path to switch to SQLite
    - Path: codex-sessions/internal/indexdb/build.go
      Note: Index build pipeline and metadata population
    - Path: codex-sessions/internal/indexdb/schema.go
      Note: Schema changes for full metadata
    - Path: codex-sessions/internal/sessions/reflection_copy.go
      Note: Content-based reflection-copy detection
ExternalSources: []
Summary: Implementation plan to store full session metadata in SQLite, persist reflection-copy flags, default to DB reads/writes, and auto-reindex on staleness.
LastUpdated: 2026-01-26T10:30:00-05:00
WhatFor: Guide implementation of SQLite-first metadata + indexing and staleness-aware refresh behavior.
WhenToUse: Use when implementing or reviewing the session index refresh and list/read path changes.
---


# Executive Summary

This design makes SQLite the **default source of truth** for session list/search metadata, while maintaining a safe fallback path to filesystem scans. It introduces a richer `sessions` table (full metadata + file signature + `is_reflection_copy`) and redefines index freshness using lightweight file signatures rather than full rescans. The list command will read from SQLite by default, auto-reindex on staleness unless explicitly disabled, and only fall back to filesystem scans when the index is absent or explicitly bypassed.

# Problem Statement

The current `codex-session list` command is slow because it:

- Walks the filesystem for all session files.
- Performs **content-based reflection-copy checks** before filters/limits.
- Scans **entire JSONL files** to compute `updated_at` and titles.
- Does not use the SQLite index for list output, even though the index already stores most list fields.

The SQLite index is optional and currently used only for search. Even when built, it does **not** serve as the default metadata cache, and reindex decisions still require full scans, which negates performance gains.

# Proposed Solution

## Goals (explicit user requirements)

- Store **full metadata** in SQLite.
- Add a **metadata key/value table** for quick querying.
- Persist `is_reflection_copy` in SQLite.
- Use the DB if available (default read + write path).
- Reindex automatically on staleness, unless explicitly disabled.

## Core Changes

1. **Enrich the `sessions` table** with full metadata + file signature + reflection-copy info.
2. **Add a metadata K/V table** for fast filter/query by arbitrary metadata fields.
3. **Make SQLite the default read path** for list output.
4. **Auto-reindex** based on file signature staleness (mtime/size/hash) unless disabled.
5. **Keep fallback filesystem scan** behavior for missing/invalid DB or explicit flags.

# Design Details

## 1) Schema: full metadata + file signature

### Existing schema (summary)

Current `sessions` table:

- `session_id` (PK)
- `project`
- `started_at`
- `updated_at`
- `title`
- `source_path`
- `indexed_at`

### Proposed additions

**Full metadata fields** (all JSON preserved + common fields indexed):

- `meta_json` TEXT — raw session_meta payload (full JSON)
- `cwd` TEXT — from session_meta payload
- `host` TEXT — if present in payload (future-proof)
- `model` TEXT — if present in payload
- `client` TEXT — if present (or derive from payload)
- `session_version` TEXT — if present

**File signature fields** (cheap staleness detection):

- `source_mtime` INTEGER (unix seconds) or TEXT (RFC3339)
- `source_size` INTEGER
- `source_hash` TEXT (optional, computed only on demand)

**Reflection-copy flag**:

- `is_reflection_copy` INTEGER (0/1)

> Callout: **“Full metadata”** means keeping a lossless representation of the session_meta payload in `meta_json`, while denormalizing frequent keys into columns for fast filtering.

## 1b) Metadata key/value table (quick querying)

Add a separate table to index arbitrary metadata keys without schema churn:

```
CREATE TABLE IF NOT EXISTS session_meta_kv (
  session_id TEXT NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  value_type TEXT,  -- optional: string|number|bool|json
  PRIMARY KEY (session_id, key)
);
CREATE INDEX IF NOT EXISTS idx_session_meta_kv_key_value ON session_meta_kv(key, value);
```

Populate it by flattening the `session_meta` payload:

- Scalars become `key -> value` pairs.
- Arrays can be stored as repeated rows or JSON-encoded string (choose one; document it).
- Nested objects can be flattened with dot-notation keys (e.g., `client.version`).

This table supports fast queries like:

```sql
SELECT s.session_id, s.title
FROM sessions s
JOIN session_meta_kv kv ON kv.session_id = s.session_id
WHERE kv.key = 'model' AND kv.value = 'gpt-4.1';
```

### Schema reset (no backward compatibility)

Per the updated requirement, we **do not** support backward-compatible migrations. Instead:

- Bump `schemaVersion` in `indexdb/schema.go`.
- If `user_version` differs, **drop and recreate** all index tables.
- Rebuild via `codex-session index build`.

This intentionally discards prior index data in favor of a clean schema.

## 2) SQLite-first read path

### Default behavior (list)

- If `session_index.sqlite` exists and schema is current, `codex-session list` reads **from SQLite**.
- Only fallback to filesystem scan if:
  - `--no-index` (new flag) or `--force-fs` is set, or
  - DB is missing/invalid.

### SQL query (illustrative)

```sql
SELECT
  session_id,
  project,
  started_at,
  updated_at,
  title,
  source_path,
  is_reflection_copy
FROM sessions
WHERE (? = '' OR project = ?)
  AND (? = '' OR started_at >= ?)
  AND (? = '' OR started_at <= ?)
  AND (? = 1 OR is_reflection_copy = 0)
ORDER BY started_at ASC;
```

Then apply:

- `include-most-recent` rule
- `limit` rule

Optionally apply those in SQL (window function) to avoid client-side sorting.

## 3) Staleness detection & auto-reindex

### Staleness signals (cheap)

Staleness is triggered by **file signature** differences:

- `source_mtime` differs
- `source_size` differs
- `source_hash` (optional) differs

### Reindex policy

- **Default:** auto-reindex stale sessions on access.
- **Opt-out:** `--no-reindex` or `--reindex=never`.

### Reindex strategy

- When listing/searching:
  - Load candidate rows.
  - Compare file signature with filesystem `os.Stat`.
  - If stale and reindex is enabled, enqueue reindex for those sessions.

### Pseudocode

```go
if dbAvailable && !settings.NoIndex {
    rows := indexdb.ListSessions(db, filters)
    stale := indexdb.FindStale(rows, sessionsRoot)
    if !settings.NoReindex && len(stale) > 0 {
        indexdb.ReindexSessions(db, stale)
        rows = indexdb.ListSessions(db, filters) // refresh after reindex
    }
    emit rows
    return
}

// fallback: filesystem scan
```

## 4) Persisting `is_reflection_copy`

- Compute `is_reflection_copy` during indexing (or first DB import).
- Store boolean in SQLite and filter at query time.
- Avoid file scans in list path.

### Benefit

Reflection-copy detection becomes a one-time cost instead of per-list scan.

# Design Decisions

- **SQLite as default cache:** Removes redundant file scans for list output and scales to large archives.
- **File signature–based staleness:** Cheap, accurate enough for timestamps and titles in practice.
- **Full metadata preservation:** `meta_json` preserves all future fields even if schema lags.

# Alternatives Considered

- **Pure filesystem scanning:** Lowest complexity, highest latency; does not meet performance requirement.
- **Only in-memory caching:** Fast for single runs but no persistence across invocations.
- **Hash-only staleness:** Accurate but expensive; used only optionally for corrupted/ambiguous cases.

# Implementation Plan

## Step 1: Schema + index helpers

- Update `indexdb/schema.go` with new columns and schema version.
- Add helper queries:
  - `ListSessions` (filters)
  - `GetSessionByPath`
  - `UpdateSessionSignature`

## Step 2: Index build + metadata capture

- Extend `BuildSessionIndex` to:
  - Store `meta_json` (raw payload from first line).
  - Set `cwd`, `project`, `started_at`, `title`, `updated_at`.
  - Compute and store `is_reflection_copy`.
  - Store `source_mtime` and `source_size`.

## Step 3: List command uses SQLite by default

- Add flags:
  - `--no-index` (force filesystem scan)
  - `--no-reindex` (disable auto-reindex)
- Wire logic:
  - Attempt SQLite read.
  - If stale and reindex enabled, reindex targeted sessions.
  - Fall back to filesystem scan only when needed.

## Step 4: Staleness logic (shared)

- `indexdb` package exposes:
  - `IsStale(row, os.Stat)`
  - `FindStale(rows, root)`
- Use in list/search/index build logic.

## Step 5: Docs + CLI help

- Update CLI docs/help to explain:
  - Default DB behavior
  - Reindex policy
  - How to force filesystem scan

# Open Questions

- Should reindex run synchronously during list (blocking) or asynchronously with partial results?
- If JSONL timestamps can be updated without file mtime changes, do we require hash checks?
- Should we add a `--reindex=auto|never|always` option instead of boolean?

# Appendix: Data Structures (illustrative)

```go
type SessionRow struct {
    SessionID string
    Project   string
    StartedAt time.Time
    UpdatedAt time.Time
    Title     string
    SourcePath string
    IsReflectionCopy bool
    MetaJSON string
    Cwd string
    SourceMtime int64
    SourceSize  int64
}
```
