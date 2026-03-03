---
Title: Codex Sessions Comprehensive Reliability Assessment
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
    - Path: cmd/codex-session/search.go
      Note: Primary command-level backend split and parity logic
    - Path: codex-sessions/cmd/codex-session/main.go
      Note: Root command wiring duplication and testability concerns
    - Path: codex-sessions/cmd/codex-session/search.go
      Note: Backend split logic and flag behavior differences
    - Path: codex-sessions/internal/indexdb/build.go
      Note: Incremental indexing + updated_at behavior
    - Path: codex-sessions/internal/indexdb/indexdb_test.go
      Note: Search regression coverage including punctuation queries
    - Path: codex-sessions/internal/indexdb/search.go
      Note: Indexed query semantics and scope handling
    - Path: codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-behavior-audit.sh
      Note: Synthetic parity and staleness experiment harness
    - Path: codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-real-corpus-compare.sh
      Note: Real-corpus indexed-vs-fallback comparison tool
    - Path: internal/indexdb/build.go
      Note: Incremental index behavior and stale-risk context
    - Path: internal/indexdb/search.go
      Note: Indexed query normalization and scope queries
    - Path: internal/sessions/facets.go
      Note: Facet extraction heuristics and tool/path/error signals
    - Path: ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-behavior-audit.sh
      Note: Synthetic experiment harness for parity and staleness
    - Path: ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-real-corpus-compare.sh
      Note: Real-corpus indexed-vs-fallback comparator
ExternalSources: []
Summary: Full-system reliability assessment of codex-sessions with emphasis on search/index behavior, parity gaps, and maintainability risk.
LastUpdated: 2026-03-02T15:40:00-05:00
WhatFor: Orient new engineers and provide a concrete remediation/test plan for codex-sessions reliability.
WhenToUse: Use before touching search/index behavior, adding new commands, or planning cleanup/refactors.
---


# Codex Sessions Comprehensive Reliability Assessment

## 1. Executive summary

`codex-sessions` is broadly functional and testable, but it currently has a reliability split: behavior differs materially between indexed search and fallback scan search, and stale indexes can silently return outdated results. This creates correctness risk for users who assume one stable search contract.

A concrete parser-error bug in indexed search was fixed in this investigation (`CODEX-001`, paths, tool names with punctuation), and regression tests were added. The remaining work is mostly contract alignment and observability: make backend choice explicit, normalize semantics, and detect stale index state.

## 2. Scope and method

### 2.1 Scope

This assessment covers:

1. CLI wiring and command shape (`cmd/codex-session/*.go`)
2. Session parsing/facet extraction (`internal/sessions/*`)
3. SQLite index build and search (`internal/indexdb/*`)
4. Reflection workflow execution model (`cmd/codex-session/reflect.go`, `internal/reflect/*`)
5. Tests and ticket-local verification scripts

### 2.2 Method

1. Static inspection with line-anchored evidence (`nl -ba`, `rg -n`).
2. Unit suite execution (`go test ./...`).
3. Synthetic search parity and staleness experiments.
4. Ticket-local reproducible scripts for intern handoff.

## 3. System map (for new interns)

### 3.1 Runtime architecture

```text
JSONL sessions on disk (~/.codex/sessions)
        |
        v
internal/sessions (discover, meta, title, messages, facets)
        |
        +--> cmd list/projects/show/export/reflect/search(fallback)
        |
        +--> internal/indexdb build (SQLite + FTS5)
                    |
                    v
            cmd index build/stats
                    |
                    v
            cmd search (indexed path)
```

### 3.2 Command-tree map

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

Evidence: `cmd/codex-session/main.go:12-209`.

## 4. Current behavior by subsystem

### 4.1 Session discovery and normalization

- Discovery includes `rollout-*.jsonl`, excludes `-copy` filenames by default, and can exclude content-detected reflection copies (`internal/sessions/discover.go:31-63`).
- Message extraction is intentionally best-effort for two shapes only (`event_msg user_message`, `response_item message`) (`internal/sessions/messages.go:111-133`).
- JSONL line scanning caps line size to 8 MB (`internal/sessions/jsonl.go:34-37`), which is pragmatic but still a hard ceiling.

### 4.2 Index build and search pipeline

- Index build is incremental by comparing `sessions.updated_at` to latest timestamp from JSONL (`internal/indexdb/build.go:48-57`, `:82-88`).
- Search joins FTS tables to `sessions` metadata and ranks by `bm25` (`internal/indexdb/search.go:70-97`, `:144-171`, `:218-245`).
- Query strings are now normalized as literal FTS phrases to prevent parser errors on punctuation-heavy input (`internal/indexdb/search.go:46-53`, `:277-282`).

### 4.3 Search command backend split

Search has two different engines in one command (`cmd/codex-session/search.go`):

1. Indexed path if `--use-index=true`, index file exists, and `--case-sensitive=false` (`:212-310`).
2. Fallback scan path otherwise (`:312-415`).

That backend switch is practical for speed, but it currently yields observable semantic differences.

## 5. Findings (ordered by severity)

## Finding 1 (high): Indexed and fallback search do not have one stable contract

**Problem**

The same CLI flags can produce materially different results depending on backend selection.

**Where to look**

- `cmd/codex-session/search.go:212-310` (indexed path)
- `cmd/codex-session/search.go:312-415` (fallback path)
- Ticket script: `scripts/search-behavior-audit.sh`

**Example evidence**

Indexed path ignores `--limit` (fallback enforces it):

```go
// fallback only
if settings.Limit > 0 && len(metas) > settings.Limit {
    metas = metas[len(metas)-settings.Limit:]
}
```

(From `cmd/codex-session/search.go:354-356`)

Synthetic audit output:

```text
indexed count with --limit=1: 2
fallback count with --limit=1: 0
```

**Why it matters**

Users cannot trust `search` as one API. Result drift between modes increases debugging cost and can hide/mislead analyses.

**Cleanup sketch**

```text
search command
  -> select candidate sessions once (shared filtering: project/since/until/include-most-recent/include-copies/limit)
  -> if indexed backend available
       run backend query constrained to candidate session_ids
     else
       scan candidate JSONL sessions
  -> unify row schema + aggregation
```

## Finding 2 (high): Stale index usage is silent

**Problem**

Search uses existing index if file exists; it does not validate freshness against session files before query.

**Where to look**

- `cmd/codex-session/search.go:212-214`
- `internal/indexdb/build.go:82-88`
- Ticket script: `scripts/search-behavior-audit.sh`

**Example evidence**

```text
indexed new-term count after late write: 0
fallback new-term count after late write: 1
```

(After appending a new JSONL line post-index-build)

**Why it matters**

Stale data risk is correctness-critical. Results can be wrong without warning.

**Cleanup sketch**

```text
Before indexed search:
  gather newest_mtime under sessions-root (or cheap timestamp watermark)
  compare with sessions.indexed_at max
  if stale:
    warn + optional fail (strict mode)
    or auto-fallback to scan mode
```

## Finding 3 (resolved): Query semantics are now explicit with safe default + expert opt-in

**Problem**

Indexed queries needed safe default handling for punctuation-heavy user input, but advanced users may still need raw FTS operators.

**Where to look**

- `internal/indexdb/search.go:46-53`, `:277-282`
- `internal/indexdb/indexdb_test.go:106-165`

**Example snippet**

```go
escaped := strings.ReplaceAll(trimmed, `"`, `""`)
return `"` + escaped + `"`
```

**Why it matters**

Power users may expect raw FTS operators. Without documentation, behavior changes look like bugs.

**Decision implemented**

```text
Default: literal query contract (safe)
Optional: --raw-fts-query (expert mode)
Help text now documents this behavior in the search command.
```

## Finding 4 (medium): Main command wiring is still repetitive and lightly tested

**Problem**

`main.go` repeats command construction/parser config blocks, increasing drift risk.

**Where to look**

- `cmd/codex-session/main.go:18-205`

**Why it matters**

Higher maintenance cost and easier accidental omission of command registration/config.

**Cleanup sketch**

```go
func buildGlazed(cmdCtor func() (cmds.Command, error)) *cobra.Command { ... }
func buildRootCommand() (*cobra.Command, error) { ... }
```

Then test `buildRootCommand()` command tree.

## Finding 5 (medium): Command-level integration test coverage is thin

**Problem**

Most tests target internals; few cover command flag contracts/end-to-end outputs.

**Where to look**

- Current test files: `internal/*/*_test.go`, `cmd/codex-session/parallel_ordered_test.go`

**Why it matters**

Backend-switch regressions can pass unit tests while breaking CLI behavior.

**Cleanup sketch**

```text
Add cmd/codex-session/search_integration_test.go:
  - build temp sessions
  - run index build + search in both modes
  - assert parity under defined contract
```

## Finding 6 (low): Some scan loops drop malformed sessions silently

**Problem**

Several commands continue on parse/meta errors without emitting warning rows.

**Where to look**

- `cmd/codex-session/list.go:129-133`
- `cmd/codex-session/search.go:323-326`, `:359-362`

**Why it matters**

Silent drops reduce operator visibility when data hygiene problems occur.

**Cleanup sketch**

```text
Collect skipped-file counters + optional --verbose-warnings rows
Emit summary at end: scanned / skipped / reasons
```

## 6. Fixes delivered during this investigation

### 6.1 Glazed API/tag migration (compatibility)

Applied across `cmd/codex-session/*.go`:

1. `glazed.parameter:"..."` -> `glazed:"..."`
2. `values.DecodeSectionInto(...)` -> `vals.DecodeSectionInto(...)`
3. `ShortHelpLayers` -> `ShortHelpSections`

### 6.2 Indexed punctuation-query bug fix

Implemented in `internal/indexdb/search.go` with regression tests in `internal/indexdb/indexdb_test.go`.

Verified working for:

- `CODEX-001`
- `go-go-os`
- `/tmp/test.txt`
- `functions.shell_command`
- `foo/bar`

## 7. API and ownership map

### 7.1 Key APIs

1. `sessions.DiscoverRolloutFilesWithOptions` (`internal/sessions/discover.go`)
2. `sessions.ReadSessionMeta` (`internal/sessions/parser.go`)
3. `sessions.ExtractMessages` / `sessions.ExtractFacets` (`internal/sessions/messages.go`, `facets.go`)
4. `indexdb.BuildSessionIndex` / `indexdb.Search` (`internal/indexdb/build.go`, `search.go`)
5. `SearchCommand.RunIntoGlazeProcessor` (`cmd/codex-session/search.go`)

### 7.2 Suggested ownership boundaries

```text
cmd/codex-session/*         -> CLI contract + row shaping
internal/sessions/*         -> JSONL decoding + normalized model extraction
internal/indexdb/*          -> persistence/index/search backend
internal/reflect/*          -> codex subprocess + cache + prompt versioning
```

## 8. Implementation plan (phased)

### Phase 1: Contract alignment for search

1. Define explicit parity rules between indexed/fallback behavior.
2. Apply `limit/include-*` semantics consistently.
3. Add CLI integration tests for both backends.

### Phase 2: Index freshness policy

1. Add staleness check API (`indexdb.CheckFreshness` or equivalent).
2. Expose policy flags (`--stale-index=warn|fallback|error`).
3. Include staleness metadata in output rows.

### Phase 3: Maintainability cleanup

1. Refactor `main.go` into testable `buildRootCommand()`.
2. Add command-tree wiring tests.
3. Add warning counters for skipped malformed sessions.

## 9. Pseudocode and flow sketches

### 9.1 Unified search flow

```text
func RunSearch(settings):
  candidates = SelectSessions(settings.filters, settings.limit, settings.includeMostRecent, settings.includeCopies)

  backend = ChooseBackend(settings, indexExists, caseSensitive)
  if backend == INDEX:
    freshness = CheckIndexFreshness(indexPath, candidates)
    if freshness.stale:
      handle according to stale policy
    hits = IndexSearch(query=settings.query, scope=settings.scope, candidateIDs=candidates.ids)
  else:
    hits = ScanSearch(query=settings.query, candidates=candidates, caseSensitive=settings.caseSensitive)

  return NormalizeRows(hits, settings.perMessage)
```

### 9.2 Index freshness sketch

```text
func CheckIndexFreshness(indexDB, sessionsRoot):
  latestFileTS = MaxJSONLTimestamp(sessionsRoot)
  latestIndexedTS = SELECT max(indexed_at) FROM sessions
  return latestIndexedTS >= latestFileTS
```

## 10. Validation and test strategy

### 10.1 Tests run in this investigation

1. `go test ./...` -> pass
2. `go test ./internal/indexdb -v` -> pass (includes new punctuation regression)
3. `scripts/search-behavior-audit.sh` -> reproduced parity/staleness gaps

### 10.2 Tests to add next

1. `cmd/codex-session/search_integration_test.go` parity matrix:
   - `use-index=true/false`
   - `case-sensitive=true/false`
   - `scope=messages/tools/all`
2. Staleness policy tests (warn/fallback/error behavior)
3. `main_wiring_test.go` command-tree assertions

### 10.3 Real-corpus comparison snapshot (task 15)

I ran ticket script `scripts/search-real-corpus-compare.sh` against real sessions under `~/.codex/sessions` with bounded filters:

- project: `2026-02-12--hypercard-react`
- since: `2026-02-01`

Summary results:

1. `scope=messages`, `query=codex`
   - indexed_count: `8`
   - fallback_count: `8`
   - session-id set diff: none
2. `scope=tools`, `query=functions.shell_command`
   - indexed_count: `0`
   - fallback_count: `0`
   - session-id set diff: none
3. `scope=all`, `query=/home/manuel`
   - indexed_count: `8`
   - fallback_count: `8`
   - session-id set diff: none

Observed differences were in ranking/snippet rendering and match-count aggregation, not in which sessions matched for this filtered slice.

## 11. Intern runbook

1. Start with `README.md` and `cmd/codex-session/main.go` for CLI shape.
2. Read `internal/sessions/messages.go` and `facets.go` to understand extraction heuristics.
3. Read `internal/indexdb/build.go` and `search.go` for index semantics.
4. Run:

```bash
go test ./...
./ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-behavior-audit.sh
```

5. For real data comparison, run:

```bash
./ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-real-corpus-compare.sh \
  --sessions-root ~/.codex/sessions \
  --query "CODEX-001" \
  --scope messages \
  --include-most-recent
```

## 12. Open questions

1. Should indexed search ever diverge from fallback semantics, or must parity be strict?
2. Should stale index default behavior be warning, hard error, or auto-fallback?
3. Should raw FTS syntax be exposed as opt-in (`--raw-fts-query`) or kept internal only?
4. Do we want warning rows/counters for skipped malformed files by default?

## 13. References

- `cmd/codex-session/main.go`
- `cmd/codex-session/search.go`
- `cmd/codex-session/index_build.go`
- `internal/indexdb/search.go`
- `internal/indexdb/build.go`
- `internal/indexdb/schema.go`
- `internal/indexdb/indexdb_test.go`
- `internal/sessions/discover.go`
- `internal/sessions/messages.go`
- `internal/sessions/facets.go`
- `internal/sessions/jsonl.go`
- `ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-behavior-audit.sh`
- `ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-real-corpus-compare.sh`
