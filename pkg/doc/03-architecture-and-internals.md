---
Title: Architecture and Internals
Slug: codex-session-architecture
Short: Detailed architecture of codex-session command wiring, session parsing, index subsystem, search execution, and operational safeguards.
Topics:
- codex-session
- architecture
- internals
- search
- indexdb
Commands:
- search
- index build
- index stats
- reflect
- traces md
Flags:
- use-index
- stale-index-policy
- raw-fts-query
- scope
- include-tool-calls
- include-tool-outputs
- case-sensitive
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

This document covers how `codex-session` is structured internally, how command wiring maps to runtime behavior, and where the key correctness and reliability boundaries live. It is intended for maintainers changing search/index behavior or extending command surfaces.

## System boundaries and responsibilities

This section covers the major packages and why each exists.

- `cmd/codex-session`: CLI entrypoint, Cobra tree construction, Glazed command adapters.
- `internal/sessions`: filesystem discovery, JSONL parsing, metadata extraction, message/facet helpers.
- `internal/indexdb`: SQLite schema, build/update paths, indexed query execution.
- `internal/reflect`: codex-driven reflection execution and cache handling.
- `internal/tracesmd`: markdown trace renderer over selected response items.
- `pkg/doc`: embedded help pages loaded into Glazed help system.

## Command wiring architecture

This section covers root command assembly and why helper-based wiring exists.

`buildRootCommand()` assembles a grouped CLI tree:

```text
codex-session
  projects
  list
  show
  search
  export
  reflect
  index
    build
    stats
  cleanup
    reflection-copies
  traces
    md
```

`buildGlazedCommand()` and `addGlazedCommand()` centralize parser config and command build error handling, reducing copy/paste drift and making tree structure testable in `main_wiring_test.go`.

## Dataflow architecture

This section covers runtime flow from JSONL files to outputs.

```text
~/.codex/sessions/*.jsonl
      |
      v
internal/sessions discovery + parsing
      |
      +--------------------------+
      |                          |
      v                          v
fallback scan path          index build path
(search --use-index=false)  (index build)
      |                          |
      v                          v
substring matching          SQLite + FTS tables
      |                          |
      +------------+-------------+
                   |
                   v
             indexed search path
             (search --use-index)
```

## Search execution model

This section covers backend selection and why policy flags exist.

At runtime, `search` chooses backend using a guard sequence:

1. `--use-index=false` => fallback scan.
2. `--case-sensitive=true` => fallback scan (indexed path is case-insensitive).
3. Missing index file => fallback scan.
4. Otherwise indexed path, with stale policy check.

Stale policy (`--stale-index-policy`):

- `ignore`: proceed indexed silently.
- `warn`: proceed indexed, emit warning.
- `fallback`: emit warning and use fallback scan.
- `error`: fail fast.

Conceptual pseudocode:

```text
if !useIndex or caseSensitive or !indexExists:
  return fallbackSearch()

if isIndexStale(selection):
  switch stalePolicy:
    ignore   -> indexedSearch()
    warn     -> warn(); indexedSearch()
    fallback -> warn(); fallbackSearch()
    error    -> fail()

return indexedSearch()
```

## Query semantics and correctness contract

This section covers query parsing behavior and why literal default mode is required.

- Default indexed mode treats `--query` as literal text and escapes/quotes for safe FTS matching.
- `--raw-fts-query` opts into direct FTS syntax and is intended for advanced users.
- Scope contract:
  - `messages`: user/assistant message content.
  - `tools`: tool call arguments and tool outputs.
  - `all`: union of messages and tools.

This contract is protected by tests in `internal/indexdb/indexdb_test.go` and command-level stale behavior tests in `cmd/codex-session/search_stale_test.go`.

## Index subsystem details

This section covers persistence model and update strategy.

- Index file default: `<sessions-root>/session_index.sqlite`.
- Builder walks selected sessions and updates rows for changed/new files.
- Optional inclusion of tool calls and tool outputs controls index coverage and sensitivity footprint.
- Stats command reports indexed counts and index metadata for health checks.

Design tradeoff:

- Including tool outputs improves diagnostic search power.
- It can increase index size and may retain sensitive payload data.

## Reflection and traces subsystems

This section covers post-processing architecture.

`reflect` pipeline:

1. Select sessions (same filtering patterns as list/search).
2. Resolve prompt source (preset/file/text).
3. Evaluate cache reuse mode.
4. Execute codex calls (parallel or sequential).
5. Emit result rows with optional metadata.

`traces md` pipeline:

1. Select sessions.
2. Parse `response_item` payloads.
3. Filter payload types.
4. Apply truncation controls.
5. Render markdown report.

## Extension points for maintainers

This section covers where to implement future features safely.

- New top-level command: add constructor + one line in `buildRootCommand()`.
- New search mode: extend `SearchSettings` and backend selection in `cmd/codex-session/search.go`.
- New indexed scope: extend scope enum and query implementation in `internal/indexdb/search.go`.
- New help docs: add `pkg/doc/*.md` with unique `Slug` frontmatter.

## Observability and verification strategy

This section covers practical checks that catch most regressions.

- Unit tests: `go test ./...`.
- Command tree: `go test ./cmd/codex-session -run TestBuildRootCommandWiring`.
- Help docs loaded: `go test ./pkg/doc -run TestAddDocToHelpSystemLoadsExpectedSections`.
- CLI docs discoverability:

```bash
codex-session help --topics
codex-session help codex-session-getting-started
codex-session help codex-session-reference-examples
codex-session help codex-session-architecture
```

## Troubleshooting

| Problem | Cause | Solution |
| --- | --- | --- |
| New help page does not show up | Missing embed include or slug collision | Ensure file is under `pkg/doc`, frontmatter `Slug` is unique, and `AddDocToHelpSystem` loads successfully |
| Search behavior changed unexpectedly after refactor | Backend selection condition changed | Re-run stale/index/fallback tests and compare with reference scripts under ticket `scripts/` |
| Indexed and fallback parity differs for edge cases | Scope mapping or query semantics drift | Run both modes explicitly and confirm whether `raw-fts-query` or stale policy is involved |
| Root help formatting looks wrong | Help system not wired on root command | Verify `help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)` is called in `main()` |
| Command added but not reachable | Registration missing in root builder | Add command via `addGlazedCommand` and extend wiring test expectations |

## See Also

- `codex-session help codex-session-getting-started`
- `codex-session help codex-session-reference-examples`
