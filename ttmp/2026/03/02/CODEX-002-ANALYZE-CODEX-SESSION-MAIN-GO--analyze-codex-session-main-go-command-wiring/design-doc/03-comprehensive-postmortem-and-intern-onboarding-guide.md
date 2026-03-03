---
Title: Comprehensive Postmortem and Intern Onboarding Guide
Ticket: CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO
Status: active
Topics:
    - codex-sessions
    - cli
    - wiring
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: README.md
      Note: Documented root command behavior contract
    - Path: cmd/codex-session/main.go
      Note: |-
        Root command wiring and builder refactor
        Refactored root command wiring helpers and grouped command construction
    - Path: cmd/codex-session/main_wiring_test.go
      Note: Root command-tree regression coverage
    - Path: cmd/codex-session/search.go
      Note: |-
        Search backend selection, stale-index policy, and raw-query flag
        Search command stale-index policy and backend selection contract
    - Path: cmd/codex-session/search_stale_test.go
      Note: Stale-index freshness detection unit tests
    - Path: internal/indexdb/build.go
      Note: Incremental index lifecycle and rebuild criteria
    - Path: internal/indexdb/indexdb_test.go
      Note: Regression tests for punctuation, scope, and raw-query behavior
    - Path: internal/indexdb/schema.go
      Note: SQLite schema and FTS tables
    - Path: internal/indexdb/search.go
      Note: |-
        FTS query normalization and RawQuery semantics
        Indexed search query literalization and raw-query support
    - Path: internal/sessions/discover.go
      Note: Session discovery and reflection-copy filtering behavior
    - Path: internal/sessions/messages.go
      Note: Message extraction contract used by fallback search
    - Path: ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/analysis/02-real-corpus-search-compare.txt
      Note: Stored real-corpus comparison results
    - Path: ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-behavior-audit.sh
      Note: Synthetic parity and stale-index reproducibility script
    - Path: ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-real-corpus-compare.sh
      Note: Real-corpus indexed-vs-fallback comparator
ExternalSources: []
Summary: Full postmortem of the CODEX-002 investigation and implementation sequence, including architecture onboarding context for new interns.
LastUpdated: 2026-03-02T16:06:00-05:00
WhatFor: Explain what broke, why it broke, how it was fixed, and how to safely continue work in codex-sessions.
WhenToUse: Read before modifying search/index/wiring behavior or reproducing CODEX-002 outcomes.
---


# Comprehensive Postmortem and Intern Onboarding Guide

## 1. Executive summary

This postmortem documents the full lifecycle of ticket `CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO`, which started as a `main.go` wiring investigation and expanded into a full reliability pass over `codex-sessions` search/index behavior.

The key outcomes were:

1. Main command wiring was refactored to reduce duplication and make registration testable.
2. Indexed search parser failures for punctuation-heavy queries were fixed.
3. Search contracts were made explicit: safe literal default plus raw FTS opt-in.
4. Stale-index behavior became policy-driven (`ignore|warn|fallback|error`) instead of silent.
5. Scope coverage and command-tree wiring tests were added.
6. Real-corpus comparison tooling and artifacts were added for repeatable parity checks.

All ticket tasks were completed and validated with tests and `docmgr doctor`.

## 2. What this system is (intern orientation)

`codex-session` is a Go CLI for querying and analyzing Codex JSONL session logs under `~/.codex/sessions`.

Core user-facing commands:

1. `projects` and `list`: discover and filter sessions.
2. `show` and `export`: normalize and inspect timeline/facets.
3. `index build` and `index stats`: build SQLite/FTS acceleration layer.
4. `search`: query either indexed backend or fallback scanner.
5. `reflect`: generate cached reflection outputs through codex CLI.
6. `cleanup` and `traces`: maintenance and reporting utilities.

## 3. High-level architecture

### 3.1 Dataflow diagram

```text
JSONL session files (~/.codex/sessions)
        |
        v
internal/sessions
  - discover paths
  - parse session meta
  - extract messages
  - derive facets (tools/paths/errors)
        |
        +------------------------+
        |                        |
        v                        v
fallback search path       index build path
(cmd/codex-session/search) (cmd/codex-session/index build)
        |                        |
        v                        v
text scan + contains        SQLite tables + FTS5 virtual tables
        |                        |
        +-----------+------------+
                    |
                    v
             indexed search path
             (cmd/codex-session/search + internal/indexdb/search)
```

### 3.2 Command wiring diagram

```text
buildRootCommand()
  -> addGlazedCommand(root, projects/list/show/search/export/reflect)
  -> create group cmd: index
     -> addGlazedCommand(index, build/stats)
  -> create group cmd: cleanup
     -> addGlazedCommand(cleanup, reflection-copies)
  -> create group cmd: traces
     -> addGlazedCommand(traces, md)
```

## 4. Incident/problem statement

The work uncovered three classes of defects/risks:

1. **Hard failure**: indexed search crashed on common punctuation queries (IDs, paths, dotted names).
2. **Correctness risk**: stale indexes could be used silently, returning outdated results.
3. **Contract drift risk**: indexed vs fallback backends had differing semantics and no clear query contract.

Additionally, `main.go` had high duplication and no explicit wiring test coverage prior to refactor.

## 5. Root-cause analysis

### 5.1 Why punctuation queries broke

Observed failures looked like:

1. `no such column: 001`
2. `no such column: go`
3. `fts5: syntax error near "/"`
4. `fts5: syntax error near "."`

Root cause:

- User input was passed directly into `MATCH ?`, so FTS parser treated punctuation/operators syntactically instead of as literal text.

Relevant API path:

1. `cmd/codex-session/search.go` -> `indexdb.Search(...)`
2. `internal/indexdb/search.go` -> SQL `WHERE <table> MATCH ?`

### 5.2 Why stale results were possible

Root cause:

- Search would use index when file existed; no freshness gate compared selected session files to index currency before query execution.

Effect:

- Late writes to JSONL after index build were visible in fallback but absent from indexed search.

### 5.3 Why wiring maintenance risk existed

Root cause:

- `main.go` repeated command creation/BuildCobra wiring boilerplate for each command, raising chance of copy/paste drift.

Effect:

- Higher review complexity and missing-command risk during refactors.

## 6. Implementation timeline (chronological)

This is the concise code-history sequence for the CODEX-002 execution pass:

1. `3657720` `search: add stale index policy and freshness tests`
2. `061793c` `tests: add scope coverage for tool calls and outputs`
3. `f785ecf` `search: add raw FTS opt-in and document query contract`
4. `54b543f` `analysis: add real-corpus search parity validation`
5. `9d0a02e` `main: refactor command wiring with shared builders`
6. `c99536b` `test: add root command wiring coverage`
7. `99f6b0c` `docs: define root command flag behavior`
8. `bb35264` `docs: close go.work policy task`

## 7. Detailed change explanation by subsystem

## 7.1 Search contract and execution path

### 7.1.1 Safe default + raw opt-in

New behavior:

1. Default indexed search treats query as literal phrase (escaped/quoted).
2. `--raw-fts-query` bypasses literalization and sends query directly to FTS parser.

Why this matters:

- Safe for normal users.
- Still supports expert FTS expressions when explicitly requested.

### 7.1.2 Stale-index policy

New flag:

- `--stale-index-policy=ignore|warn|fallback|error`

Default:

- `fallback`

Behavior pseudocode:

```text
if use_index and not case_sensitive and index_exists:
  stale = detect_stale_index(selected_metas)
  if stale:
    if policy == ignore: continue indexed
    if policy == warn: print warning; continue indexed
    if policy == fallback: print warning; use fallback path
    if policy == error: return error
  else:
    use indexed path
else:
  use fallback path
```

### 7.1.3 Scope correctness coverage

Added regression coverage ensures:

1. `scope=messages` matches message-only tokens.
2. `scope=tools` matches tool-call arguments and tool-output text.
3. `scope=all` includes union semantics.

## 7.2 Index layer

### 7.2.1 Query conversion API

`internal/indexdb/search.go` now uses:

```go
func toLiteralFTSQuery(query string) string
```

And only applies it when `SearchOptions.RawQuery == false`.

### 7.2.2 Freshness detection helper

At command layer (`cmd/codex-session/search.go`):

1. Build filtered session set via shared selector.
2. For each selected session:
   - verify session exists in index table
   - compare file mtime against index file mtime

This intentionally prioritizes operational safety over minimal checks.

## 7.3 Main wiring

### 7.3.1 Refactor

`main.go` now has:

1. `defaultParserConfig()`
2. `buildGlazedCommand(...)`
3. `addGlazedCommand(...)`
4. `buildRootCommand()`
5. Thin `main()`

### 7.3.2 Test seam

`main_wiring_test.go` asserts command-tree completeness for top-level and grouped subcommands.

## 7.4 Documentation and contract clarifications

### 7.4.1 Root command behavior

`README.md` now states:

1. Root command is grouping-only.
2. Glazed-style flags are supported on concrete subcommands.

### 7.4.2 Workspace policy

`go.work` now aligns to `go 1.25.7`, matching workspace module requirements; workspace mode is canonical.

## 8. Validation and experiments

## 8.1 Unit and package tests

Validation command used repeatedly:

```bash
go test ./...
```

Result at end of work:

- All package tests passed.

## 8.2 Synthetic audit script

Script:

- `scripts/search-behavior-audit.sh`

Purpose:

1. Reproduce parity and stale-index behavior in controlled fixtures.
2. Confirm punctuation-query safety.

## 8.3 Real-corpus comparison

Script:

- `scripts/search-real-corpus-compare.sh`

Stored output:

- `analysis/02-real-corpus-search-compare.txt`

Bounded run snapshot (`project=2026-02-12--hypercard-react`, `since=2026-02-01`):

1. `messages` / `codex`: indexed `8`, fallback `8`
2. `tools` / `functions.shell_command`: indexed `0`, fallback `0`
3. `all` / `/home/manuel`: indexed `8`, fallback `8`

Observed differences in this slice were ranking/snippet shape, not matching session-id sets.

## 9. What remains and what to watch

The original task list is fully complete, but interns should watch these medium-term risks:

1. Comparator currently reports diffs but does not fail CI on set divergence.
2. Freshness check uses file mtime heuristic; if future pipelines modify metadata without content, policy tuning may be needed.
3. Raw query mode is intentionally sharp; user-facing docs/examples should stay explicit.

## 10. API reference map (intern quick lookup)

### 10.1 Command layer APIs

1. `buildRootCommand()` in `cmd/codex-session/main.go`
2. `SearchCommand.RunIntoGlazeProcessor(...)` in `cmd/codex-session/search.go`
3. `IndexBuildCommand.RunIntoGlazeProcessor(...)` in `cmd/codex-session/index_build.go`

### 10.2 Search/index APIs

1. `indexdb.Search(ctx, db, opts)` in `internal/indexdb/search.go`
2. `toLiteralFTSQuery(query)` in `internal/indexdb/search.go`
3. `BuildSessionIndex(...)` in `internal/indexdb/build.go`

### 10.3 Session extraction APIs

1. `DiscoverRolloutFilesWithOptions(...)` in `internal/sessions/discover.go`
2. `ReadSessionMeta(path)` in `internal/sessions/parser.go`
3. `ExtractMessages(path)` in `internal/sessions/messages.go`
4. `ExtractFacets(path, opts)` in `internal/sessions/facets.go`

## 11. Reproduction runbook for new interns

1. Run full tests.

```bash
go test ./...
```

2. Run synthetic search audit.

```bash
./ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-behavior-audit.sh
```

3. Run real-corpus comparison on a bounded slice first.

```bash
./ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-real-corpus-compare.sh \
  --sessions-root ~/.codex/sessions \
  --project 2026-02-12--hypercard-react \
  --query codex \
  --scope messages \
  --since 2026-02-01 \
  --include-most-recent
```

4. Inspect stored artifacts in ticket `analysis/` and compare with design-doc conclusions.

## 12. Postmortem conclusions

The largest reliability gains came from converting implicit behavior into explicit contracts:

1. explicit stale-index policy
2. explicit raw-query opt-in
3. explicit root-command flag semantics
4. explicit command-tree wiring tests

The system is now substantially easier to reason about for both operators and contributors, and the ticket includes scripts and artifacts needed to continue this work safely.

## 13. References

1. `cmd/codex-session/main.go`
2. `cmd/codex-session/main_wiring_test.go`
3. `cmd/codex-session/search.go`
4. `cmd/codex-session/search_stale_test.go`
5. `internal/indexdb/search.go`
6. `internal/indexdb/indexdb_test.go`
7. `internal/indexdb/build.go`
8. `internal/indexdb/schema.go`
9. `internal/sessions/discover.go`
10. `internal/sessions/messages.go`
11. `README.md`
12. `ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-behavior-audit.sh`
13. `ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-real-corpus-compare.sh`
14. `ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/analysis/02-real-corpus-search-compare.txt`
