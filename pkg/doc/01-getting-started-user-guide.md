---
Title: Getting Started and User Guide
Slug: codex-session-getting-started
Short: Install codex-session, build an index, search safely, and run reflection and traces workflows end-to-end.
Topics:
- codex-session
- getting-started
- search
- indexing
- reflection
Commands:
- projects
- list
- index build
- index stats
- search
- show
- export
- reflect
- cleanup reflection-copies
- traces md
Flags:
- sessions-root
- query
- use-index
- stale-index-policy
- raw-fts-query
- include-most-recent
- include-reflection-copies
- max-results
- project
- since
- until
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

This guide covers the full operator workflow for `codex-session`: how to discover sessions, build and verify the index, run reliable searches, inspect a session in depth, and produce reflection or trace outputs. The workflow is built to reduce surprises in production use and to make behavior explicit when data freshness or parsing semantics matter.

## Prerequisites

This section covers what you need before running commands and why each prerequisite matters. The CLI reads local JSONL session archives and can optionally maintain a local SQLite index for fast search.

- Go 1.25+ if running from source.
- Local Codex sessions under `~/.codex/sessions` (or a custom root).
- Optional but recommended: `jq` for JSON output inspection.

Install from source:

```bash
go install github.com/go-go-golems/codex-session/cmd/codex-session@latest
```

Sanity check:

```bash
codex-session --help
codex-session help --topics
```

## Step 1: Discover session inventory

This section covers initial discovery and how to validate that your filters select the intended dataset. This matters because later index and search operations only work on the session files you include.

List projects:

```bash
codex-session projects --output table
```

List recent sessions:

```bash
codex-session list --limit 20 --output table
```

Filter by project/time window:

```bash
codex-session list \
  --project /home/manuel/workspaces/2026-03-02/fix-codex-sessions \
  --since 2026-03-01 \
  --until 2026-03-03 \
  --limit 50 \
  --output table
```

Include reflection copies only when needed:

```bash
codex-session list --include-reflection-copies --limit 20 --output table
```

## Step 2: Build and verify the index

This section covers index lifecycle and verification steps. Indexed search is much faster than fallback scanning for large archives, but it must be current to avoid stale results.

Build index with safe defaults:

```bash
codex-session index build --output table
```

Build for a narrowed scope:

```bash
codex-session index build \
  --project /home/manuel/workspaces/2026-03-02/fix-codex-sessions \
  --since 2026-03-01 \
  --until 2026-03-03 \
  --include-most-recent \
  --limit 200 \
  --output table
```

Inspect index stats:

```bash
codex-session index stats --output table
```

If you changed indexing settings and want a clean rebuild:

```bash
codex-session index build --force --output table
```

## Step 3: Search with explicit semantics

This section covers search behavior, backend selection, and query safety. It matters because indexed and fallback paths have different tradeoffs, and punctuation-heavy queries can be misinterpreted if you intentionally opt into raw FTS mode.

Default search (literal query semantics):

```bash
codex-session search --query "CODEX-002" --output table
```

Search tools only:

```bash
codex-session search --query "functions.shell_command" --scope tools --output table
```

One row per message hit:

```bash
codex-session search --query "stale-index-policy" --show-matches --output table
```

Force fallback scanner (parity checks or case-sensitive needs):

```bash
codex-session search --query "RawQuery" --use-index=false --case-sensitive=true --output table
```

Explicit stale index policy:

```bash
codex-session search --query "CODEX-001" --stale-index-policy=fallback --output table
codex-session search --query "CODEX-001" --stale-index-policy=error --output table
```

Use raw FTS only when you intentionally want FTS operators:

```bash
codex-session search --query 'token NEAR("session", 3)' --raw-fts-query --output table
```

## Step 4: Inspect and export session details

This section covers deep inspection and normalized export shapes. It matters when search gives you candidate sessions and you need richer context before making decisions.

Inspect timeline view by session id:

```bash
codex-session show --session-id <SESSION_ID> --view timeline --output table
```

Inspect tool/path/error facets:

```bash
codex-session show --session-id <SESSION_ID> --view tools --output table
codex-session show --session-id <SESSION_ID> --view paths --output table
codex-session show --session-id <SESSION_ID> --view errors --output table
```

Export one session as document shape:

```bash
codex-session export --session-id <SESSION_ID> --shape document --extract all --output json
```

Export row-oriented representation:

```bash
codex-session export --session-id <SESSION_ID> --shape rows --extract timeline --output json
```

## Step 5: Reflection and trace workflows

This section covers post-processing flows that are commonly used by teams: reflection copy generation and trace markdown reporting.

Run reflection on selected sessions:

```bash
codex-session reflect \
  --project /home/manuel/workspaces/2026-03-02/fix-codex-sessions \
  --limit 5 \
  --prompt-preset reflection \
  --max-workers 2 \
  --output table
```

Dry-run reflection selection and cache status only:

```bash
codex-session reflect --limit 5 --dry-run --output table
```

Generate markdown traces:

```bash
codex-session traces md \
  --limit 2 \
  --entries-per-file 10 \
  --md-output trace_report.md
```

Cleanup reflection copies safely:

```bash
codex-session cleanup reflection-copies --dry-run --output table
codex-session cleanup reflection-copies --dry-run=false --mode trash --output table
```

## Operational runbook

This section covers a repeatable daily/weekly sequence and why it keeps behavior predictable.

1. `projects` and `list` to confirm selection scope.
2. `index build` for current scope.
3. `index stats` to validate index shape.
4. `search` with explicit stale policy.
5. `show` or `export` for deep investigation.
6. `reflect` or `traces md` for reporting.

## Troubleshooting

| Problem | Cause | Solution |
| --- | --- | --- |
| `unknown flag: --print-schema` on root command | Root command is grouping-only | Run schema/help flags on concrete subcommands, for example `codex-session search --print-schema` |
| Search misses recent messages in indexed mode | Index is stale relative to session files | Rebuild with `index build --force`, or run `search --stale-index-policy=fallback` |
| Query with punctuation fails only in raw mode | `--raw-fts-query` treats input as FTS syntax | Remove `--raw-fts-query` for literal matching |
| `--case-sensitive=true` seems slower | Case-sensitive matching runs fallback scanner | Use case-insensitive indexed mode when possible |
| Reflection includes unexpected sessions | Filters and session-id/session-ids precedence misunderstood | Prefer explicit `--session-id` or `--session-ids`, otherwise verify `--project/--since/--until/--limit` |

## See Also

- `codex-session help codex-session-reference-examples`
- `codex-session help codex-session-architecture`
