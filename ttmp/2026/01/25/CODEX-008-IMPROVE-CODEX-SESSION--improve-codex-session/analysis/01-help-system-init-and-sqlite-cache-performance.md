---
Title: Help System Init and SQLite Cache Performance
Ticket: CODEX-008-IMPROVE-CODEX-SESSION
Status: active
Topics:
    - codex
    - performance
    - docs
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: codex-sessions/cmd/codex-session/list.go
      Note: List pipeline and metadata scans
    - Path: codex-sessions/cmd/codex-session/main.go
      Note: Codex CLI entry point missing help system wiring
    - Path: codex-sessions/internal/indexdb/build.go
      Note: SQLite index build and caching decision logic
    - Path: codex-sessions/internal/sessions/conversation.go
      Note: ConversationUpdatedAt and ConversationTitle full-file scans
    - Path: codex-sessions/internal/sessions/discover.go
      Note: Filesystem scan and reflection-copy filtering
    - Path: glazed/cmd/glaze/main.go
      Note: Reference implementation of help system initialization
    - Path: glazed/pkg/help/cmd/cobra.go
      Note: SetupCobraRootCommand defines help integration and templates
ExternalSources: []
Summary: Analysis of Codex session CLI help-system initialization gaps and list performance bottlenecks tied to SQLite indexing/caching.
LastUpdated: 2026-01-25T19:01:00-05:00
WhatFor: Guide a robust help-system setup and a faster list path using cached SQLite metadata.
WhenToUse: Use when wiring help output or redesigning session list/index performance.
---


# Help System Initialization Analysis

## Current Reference Implementation (glazed)

The Glazed CLI (`glazed/cmd/glaze/main.go`) shows the canonical help-system wiring. The relevant symbols and their roles are:

- `help.NewHelpSystem()` (in `glazed/pkg/help/help.go`): creates the `*help.HelpSystem` backed by an in-memory store.
- `doc.AddDocToHelpSystem(helpSystem)` (in `glazed/pkg/doc/doc.go`): loads embedded Markdown help sections (YAML frontmatter + content) into the help system via `LoadSectionsFromFS`.
- `help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)` (in `glazed/pkg/help/cmd/cobra.go`): replaces Cobra help/usage funcs and templates, registers the `help` subcommand, and adds the `--long-help` flag. The helper also wires the help UI if present.

In `glazed/pkg/doc/doc.go`, the help docs are embedded directly:

```go
//go:embed *
var docFS embed.FS

func AddDocToHelpSystem(helpSystem *help.HelpSystem) error {
    return helpSystem.LoadSectionsFromFS(docFS, ".")
}
```

This `doc` package pattern is referenced in Glazed docs (e.g., `glazed/pkg/doc/topics/14-writing-help-entries.md`) and is the expected way to supply help sections in applications built on Glazed.

## Current Codex-Session CLI State (gaps)

The Codex CLI (`codex-sessions/cmd/codex-session/main.go`) builds a root Cobra command and registers Glazed commands, but it does **not** set up the help system.

Key gaps:

- No `help.NewHelpSystem()` instance is created.
- No embedded documentation is loaded (no `doc.AddDocToHelpSystem` equivalent in the repo).
- No call to `help_cmd.SetupCobraRootCommand`, so Cobra uses default help templates and lacks the Glazed help UI + topic browsing features.

Because the Glazed commands are built with `cli.BuildCobraCommand` and include `ShortHelpLayers` via `schema.DefaultSlug`, the help system’s layering behavior is already compatible — it just isn’t initialized.

## Proposed Initialization Plan (codex-session)

### Files and symbols to add

- New package (recommended): `codex-sessions/internal/doc` or `codex-sessions/pkg/doc`
  - `doc.go`: `AddDocToHelpSystem(*help.HelpSystem) error` with `//go:embed`.
  - Place help Markdown sections with YAML frontmatter in the same folder or subfolders.
- Update `codex-sessions/cmd/codex-session/main.go`:
  - import `github.com/go-go-golems/glazed/pkg/help`
  - import `help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"`
  - import `codex-sessions/.../doc`
  - add `help_cmd.SetupCobraRootCommand` after help system creation

### Pseudocode (main.go)

```go
rootCmd := &cobra.Command{Use: "codex-session", Short: "..."}

helpSystem := help.NewHelpSystem()
if err := doc.AddDocToHelpSystem(helpSystem); err != nil {
    cobra.CheckErr(err)
}
// Optional: route help to stderr so Glazed output stays clean
// help_cmd.SetHelpWriter(os.Stderr)
help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

// build & add commands (existing BuildCobraCommand calls)
rootCmd.AddCommand(...)

cobra.CheckErr(rootCmd.Execute())
```

### Notes / Constraints

- The help system expects Markdown files with YAML frontmatter fields like `Title`, `Slug`, `Topics`, `Commands`, `Flags`, and `SectionType` (see Glazed docs in `glazed/pkg/doc/topics/01-help-system.md`).
- The Glazed help templates (in `glazed/pkg/help/cmd/cobra.go`) respect the `shortHelpLayers` Cobra annotation; this is already set by `cli.BuildCobraCommand` when `ShortHelpLayers` is provided.
- Glazed help now defaults to stdout. If Codex CLI output is typically piped, use `help_cmd.SetHelpWriter(os.Stderr)` once at startup to preserve machine-readable stdout.

# List Performance + SQLite Cache Analysis

## Current list pipeline

The `codex-session list` command runs a filesystem-based scan, not the SQLite index. The critical path is:

1. `sessions.DiscoverRolloutFilesWithOptions` (`codex-sessions/internal/sessions/discover.go`)
   - `filepath.WalkDir` over the session root.
   - For each JSONL file, optionally calls `sessions.IsReflectionCopy` (`reflection_copy.go`), which can scan up to 2000 JSONL lines per file (`WalkJSONLLines`).
2. For each path:
   - `sessions.ReadSessionMeta` (`parser.go`) reads only the first line to get `session_id`, `timestamp`, and `cwd` (project derivation).
   - filters by `project`, `since`, `until`.
3. After filtering + sorting + limit:
   - `sessions.ConversationUpdatedAt` (`conversation.go`) scans **all lines** in each file to compute max timestamp.
   - `sessions.ConversationTitle` (`conversation.go`) scans the file (until first user message) to derive the title.

### Why it’s slow

- The reflection-copy check happens **before** any filtering, so all files are scanned even when only a few are returned.
- The updated-at and title computations rescan full files even when a cached index exists.
- These scans use JSON parsing per line and will scale linearly with session size.

## Existing SQLite index (indexdb)

The repo already contains an optional SQLite/FTS index (`codex-sessions/internal/indexdb`). The `sessions` table contains the exact columns the list command needs:

- `sessions` table columns (from `indexdb/schema.go`):
  - `session_id`, `project`, `started_at`, `updated_at`, `title`, `source_path`, `indexed_at`
- `indexdb.BuildSessionIndex` (`indexdb/build.go`) populates these columns while building the FTS index.

### Cache invalidation logic today

`BuildSessionIndex` checks whether to rebuild using:

- `shouldReindex(existingUpdatedAt, newUpdatedAt)`
- **But** `newUpdatedAt` is computed by rescanning the JSONL file (`sessions.ConversationUpdatedAt`), so it pays the full scan cost before deciding to skip.

That means the SQLite index is not acting as a metadata cache for list or index refresh decisions — it’s only a storage sink after the expensive work is done.

## Performance opportunities (high-value)

### 1) Use SQLite metadata for list output

**Idea:** if `session_index.sqlite` exists and has rows, read from it for list output (fast SQL scan + sort). Fall back to filesystem scan if index missing or empty.

Benefits:
- Avoid per-file JSONL scans for `updated_at` and `title`.
- Avoid reflection-copy checks on the filesystem path for list output.

Pseudocode sketch:

```go
if useIndex { // new flag or auto-detect index file
    db := indexdb.Open(indexPath)
    indexdb.EnsureSchema(db)
    rows := QuerySessions(db, project, since, until, includeMostRecent, limit)
    emit rows
    return
}
// fallback to filesystem scan (existing behavior)
```

Potential SQL shape:

```sql
SELECT session_id, project, started_at, updated_at, title, source_path
FROM sessions
WHERE (? = '' OR project = ?)
  AND (? = '' OR started_at >= ?)
  AND (? = '' OR started_at <= ?)
ORDER BY started_at ASC
```

Then apply the `include-most-recent` and `limit` logic in SQL or in-memory. Sorting by `started_at` keeps parity with current behavior.

### 2) Add minimal “metadata cache” columns to sessions table

Add file-signature columns (e.g., `source_mtime`, `source_size`, maybe `source_hash`) so that reindex decisions can skip full scans when a file hasn’t changed.

Proposed fields:
- `source_mtime` (integer epoch or RFC3339)
- `source_size` (bytes)
- `source_hash` (optional, for extra safety)

Then `shouldReindex` becomes a cheap check:

```go
if fileSignatureUnchanged(existing) && !opts.Force {
    skip
}
// else rescan and update
```

### 3) Incremental update-at computation (avoid full scan)

If we store the last processed line offset or last timestamp in SQLite, we can scan **only new lines** to update `updated_at` and `title`:

```text
if fileSize == cachedSize:
    updated_at = cached_updated_at
else:
    seek to cachedSize
    scan new lines, update max timestamp
```

This makes index refresh proportional to new data, not total data size.

### 4) Defer reflection-copy detection

Reflection-copy checks currently happen during file discovery, before any filters or limits. Move the check later:

- First filter by project + date using `ReadSessionMeta`.
- Only then call `IsReflectionCopy` on the remaining candidates.

Or store `is_reflection_copy` in the SQLite sessions table and filter in SQL.

### 5) Optional “fast list” mode

Expose a `--fast` or `--use-index` flag that:

- Skips `ConversationUpdatedAt` and `ConversationTitle` file scans.
- Uses cached values from SQLite or `os.Stat` heuristics.

This provides an immediate escape hatch even if the index is stale.

## Suggested implementation sequence

1. Add a `ListSessions` query helper to `internal/indexdb` and wire a new `--use-index` (or `--index-path` reuse) to `codex-session list`.
2. Store file signature metadata in the `sessions` table and extend `BuildSessionIndex` to skip scans when unchanged.
3. Move reflection-copy checks after filtering, or persist `is_reflection_copy` in SQLite.
4. (Optional) add incremental scan behavior for `updated_at`.

## Risks and correctness considerations

- Using SQLite for list output changes the trust boundary: results depend on index freshness. Consider explicit flags or warnings when index data is stale.
- If `updated_at` is computed from timestamps inside the JSONL, file mtime alone may not always reflect semantics; mtime is a heuristic, not a guarantee.
- Reflection-copy detection depends on content, not just filename. If you avoid scanning, you need a cached field that was computed by a prior scan.

## Key Files and Symbols

- `codex-sessions/cmd/codex-session/main.go` — root Cobra command setup.
- `codex-sessions/cmd/codex-session/list.go` — list pipeline and output fields.
- `codex-sessions/internal/sessions/discover.go` — filesystem discovery and reflection-copy checks.
- `codex-sessions/internal/sessions/reflection_copy.go` — `IsReflectionCopy` implementation.
- `codex-sessions/internal/sessions/conversation.go` — `ConversationUpdatedAt`, `ConversationTitle` (full-file scans).
- `codex-sessions/internal/indexdb/schema.go` — sessions table schema for cached metadata.
- `codex-sessions/internal/indexdb/build.go` — `BuildSessionIndex` and caching decision path.
- `glazed/cmd/glaze/main.go` — reference help system setup.
- `glazed/pkg/doc/doc.go` + `glazed/pkg/help/cmd/cobra.go` — help-system load + Cobra integration.
