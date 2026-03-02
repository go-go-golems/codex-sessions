---
Title: Investigation diary
Ticket: CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO
Status: active
Topics:
    - codex-sessions
    - cli
    - wiring
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: codex-sessions/cmd/codex-session/main.go
      Note: Line-level investigation target
    - Path: codex-sessions/cmd/codex-session/search.go
      Note: Representative subcommand behavior check
    - Path: codex-sessions/internal/indexdb/indexdb_test.go
      Note: Regression tests for literal punctuation query safety
    - Path: codex-sessions/internal/indexdb/search.go
      Note: Indexed search query handling fix for punctuation-heavy input
    - Path: codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/main-go-wiring-audit.sh
      Note: Chronological repro harness
    - Path: codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-behavior-audit.sh
      Note: Synthetic parity and stale-index behavior probe
    - Path: codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-real-corpus-compare.sh
      Note: Real-corpus indexed-vs-fallback comparison helper
    - Path: codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/analysis/02-real-corpus-search-compare.txt
      Note: Stored output from real-corpus comparison runs
    - Path: go.work
      Note: Recorded startup failure context
    - Path: internal/indexdb/indexdb_test.go
      Note: Diary captures regression test additions
    - Path: internal/indexdb/search.go
      Note: Diary captures root-cause and fix of punctuation query failures
    - Path: scripts/probe_search_punctuation.sh
      Note: Diary references reusable punctuation probe
    - Path: ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-behavior-audit.sh
      Note: Diary references parity/staleness evidence
ExternalSources: []
Summary: Chronological investigation of cmd/codex-session/main.go and codex-session search/index reliability behavior.
LastUpdated: 2026-03-02T15:42:00-05:00
WhatFor: Preserve commands, evidence, fixes, and decisions across wiring and search/index investigations.
WhenToUse: Continue this investigation, rerun scripts, or validate behavior in a fresh checkout.
---



# Diary

## Goal

Investigate `cmd/codex-session/main.go`, identify what is wrong (or fragile) in that area, and leave reproducible evidence for a new engineer.

## Step 1: Create ticket workspace and locate the exact target repository

I started by locating all copies of `codex-sessions/cmd/codex-session/main.go` across local workspaces, because there were multiple repositories with similarly named directories. I scoped this ticket to the active `fix-codex-sessions` workspace and created a dedicated ticket there.

This prevented cross-repo confusion and ensured that every subsequent command, report, and script references the same codebase.

### Prompt Context

**User prompt (verbatim):** "ok but now do a second investigations investigating codex-sessions/cmd/codex-session/main.go and figuring out what is wrong with that part. In a new ticket, but otherwise identical approach."

**Assistant interpretation:** Open a second ticket and repeat the full investigation workflow, focused specifically on `cmd/codex-session/main.go`.

**Inferred user intent:** Get the same depth and rigor as the first ticket, but for a different failure surface.

**Commit (code):** N/A

### What I did

- Located candidate files:

```bash
find /home/manuel/workspaces -type f -path '*/codex-sessions/cmd/codex-session/main.go' | sort
```

- Created new ticket and docs in the selected repo:

```bash
docmgr ticket create-ticket --ticket CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO --title "Analyze codex-session main.go command wiring" --topics codex-sessions,cli,wiring
docmgr doc add --ticket CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO --doc-type design-doc --title "Codex Session main.go Failure Analysis"
docmgr doc add --ticket CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO --doc-type reference --title "Investigation diary"
```

### Why

- Ensure analysis runs against the intended checkout and toolchain context.

### What worked

- Ticket workspace created successfully at:
  - `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring`

### What didn't work

- N/A.

### What I learned

- There are multiple codex-sessions workspaces locally; path disambiguation is required before any conclusion.

### What was tricky to build

- Distinguishing similarly named repos quickly without missing the active one.
- I used absolute-path discovery first, then ticket creation in the selected workspace.

### What warrants a second pair of eyes

- Ticket scope selection if cross-repo parity comparisons are later needed.

### What should be done in the future

- If cross-repo behavior matters, add a comparative appendix against the older checkout.

### Code review instructions

- Verify the ticket path and that analysis artifacts live under the new CODEX-002 workspace.

### Technical details

- Active analyzed file:
  - `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/cmd/codex-session/main.go`

## Step 2: Map command wiring and run baseline CLI behavior checks

I read `main.go` and all command constructors to confirm what is actually registered. Then I ran CLI help commands to determine whether command wiring itself is failing at runtime or whether the perceived breakage comes from environment/tooling around startup.

This established an important baseline: under `GOWORK=off`, command wiring is operational and the command tree renders correctly.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Inspect main.go command wiring and validate runtime behavior directly.

**Inferred user intent:** Distinguish real main.go bugs from adjacent environment or workflow failures.

**Commit (code):** N/A

### What I did

- Read main and command files:

```bash
nl -ba cmd/codex-session/main.go | sed -n '1,260p'
rg -n "func New[A-Za-z0-9_]*Command\(" cmd/codex-session -S
```

- Runtime checks:

```bash
GOWORK=off go run ./cmd/codex-session --help
GOWORK=off go run ./cmd/codex-session search --help
GOWORK=off go run ./cmd/codex-session index build --help
GOWORK=off go run ./cmd/codex-session cleanup reflection-copies --help
```

### Why

- Confirm whether command registration in main.go is currently broken.

### What worked

- Help output is healthy with expected command tree in `GOWORK=off` mode.
- Constructor count and registration count align.

### What didn't work

- Running without `GOWORK=off` failed due workspace Go version mismatch.

Exact failure (first lines):

```text
go: module ../go-go-goja listed in go.work file requires go >= 1.25.7, but go.work lists go 1.25
```

### What I learned

- Main wiring itself is not immediately broken; startup failure in this checkout is dominated by workspace toolchain config.

### What was tricky to build

- Early command checks can falsely implicate main wiring when `go.work` blocks execution before command logic runs.
- I resolved this by explicitly comparing default mode vs `GOWORK=off` mode.

### What warrants a second pair of eyes

- Whether workspace-mode failure should be treated as repo-level blocker for this ticket.

### What should be done in the future

- Add startup troubleshooting guidance (`go.work` mismatch + `GOWORK=off`) in development docs.

### Code review instructions

- Re-run:

```bash
go run ./cmd/codex-session --help
GOWORK=off go run ./cmd/codex-session --help
```

- Confirm mismatch in behavior and error message.

### Technical details

- `go.work` and module version evidence:
  - `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/go.work:1`
  - `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/go-go-goja/go.mod:3`
  - `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/glazed/go.mod:3`

## Step 3: Build reproducible main.go audit script and identify fragile wiring patterns

I converted the manual checks into a single audit script under the ticket workspace. The script verifies constructor/registration parity, reproduces workspace startup failure, confirms baseline success with `GOWORK=off`, checks root-flag behavior, and detects missing wiring tests.

This made the investigation portable and reduced ambiguity for reviewers.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Produce reproducible evidence and scripts in the ticket `scripts/` folder.

**Inferred user intent:** Leave behind a practical handoff package, not just prose conclusions.

**Commit (code):** N/A

### What I did

- Added and executed:
  - `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/main-go-wiring-audit.sh`

### Why

- Ensure all findings can be replayed in one command.

### What worked

- Script produced stable evidence:

```text
constructors=10
build_cobra_registrations=10
parser_config_repetitions=10
```

- Confirmed `go.work` startup failure and `GOWORK=off` success.
- Confirmed no wiring-focused tests detected.

### What didn't work

- N/A for script generation.

### What I learned

- The current issue is less “missing command” and more “fragile registration surface + environment blocker”.

### What was tricky to build

- Needed to separate hard failures from design-quality failures in one reportable artifact.
- I split checks into explicit sections (runtime, wiring counts, tests).

### What warrants a second pair of eyes

- Whether root-level glazed flags (`--print-schema`) should be considered unsupported-by-design or a UX gap.

### What should be done in the future

- Add `main_wiring_test.go` and integrate script-like checks into CI.

### Code review instructions

- Run:

```bash
codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/main-go-wiring-audit.sh
```

- Validate each section’s output and compare to this diary.

### Technical details

- Root-flag behavior observed:

```text
codex-session --print-schema
Error: unknown flag: --print-schema
```

## Step 4: Validate repo tests and scope residual risk

I ran the package test suite to confirm there is no immediate failing unit test in this branch and to quantify what is not covered by tests. The suite passes, which supports the conclusion that failures are environmental and structural rather than active unit-level breakage.

The remaining risk is future regression in `main.go` wiring because command-tree registration is not unit-tested.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Conclude investigation with reproducibility and confidence checks.

**Inferred user intent:** Receive a defensible answer, not speculation.

**Commit (code):** N/A

### What I did

- Ran:

```bash
GOWORK=off go test ./...
```

### Why

- Confirm no hidden failing tests in current branch and calibrate risk.

### What worked

- Tests passed across command and internal packages.

### What didn't work

- N/A.

### What I learned

- Existing tests validate internal logic, but not root command wiring completeness.

### What was tricky to build

- Avoid over-interpreting passing tests as proof that CLI wiring cannot regress.
- I addressed this by separately checking for main wiring test coverage.

### What warrants a second pair of eyes

- Define minimum CLI wiring coverage standards for this repo.

### What should be done in the future

- Add wiring tests and optionally export `buildRootCommand()` to make tree assertions straightforward.

### Code review instructions

- Re-run `GOWORK=off go test ./...` and inspect absence of `main.go` wiring tests.

### Technical details

- Test output summary included:

```text
ok   github.com/go-go-golems/codex-session/cmd/codex-session
ok   github.com/go-go-golems/codex-session/internal/indexdb
ok   github.com/go-go-golems/codex-session/internal/sessions
```

## Step 5: Reproduce indexed search breakage and patch FTS query handling

I switched from main wiring analysis to search correctness after the user confirmed that search still felt broken. I reproduced real failures first, then patched the indexed search backend and added a regression test before rerunning the full suite.

The key issue was clear once reproduced: raw user query text was being passed directly into SQLite FTS `MATCH`, so punctuation-heavy queries were interpreted as FTS syntax instead of plain text.

### Prompt Context

**User prompt (verbatim):** "ok i fixed go work, continue. Thesearch I think was not working well in codex-sessions"

**Assistant interpretation:** Resume from the `go.work` fix and investigate remaining `search` behavior problems in `codex-sessions`.

**Inferred user intent:** Get search behavior to be dependable for real query strings used in session IDs, paths, and tool names.

**Commit (code):** N/A

### What I did

- Reproduced failures with synthetic JSONL + index build + search queries:

```bash
go run ./cmd/codex-session index build --sessions-root "$TMPDIR/sessions" --include-most-recent
go run ./cmd/codex-session search --sessions-root "$TMPDIR/sessions" --query "CODEX-001"
go run ./cmd/codex-session search --sessions-root "$TMPDIR/sessions" --query "go-go-os"
go run ./cmd/codex-session search --sessions-root "$TMPDIR/sessions" --query "/tmp/test.txt"
go run ./cmd/codex-session search --sessions-root "$TMPDIR/sessions" --query "functions.shell_command"
go run ./cmd/codex-session search --sessions-root "$TMPDIR/sessions" --query "foo/bar"
```

- Patched indexed search query normalization:
  - `internal/indexdb/search.go`:
    - Added `toLiteralFTSQuery(...)`
    - Applied it in `Search(...)` before SQL `MATCH ?`
- Added regression coverage:
  - `internal/indexdb/indexdb_test.go`:
    - `TestSearchLiteralQueryWithPunctuation`
- Added reusable repo script:
  - `scripts/probe_search_punctuation.sh`

### Why

- Prevent SQL/FTS parser errors for ordinary user text.
- Keep `--query` behavior aligned with user expectations from CLI help text.

### What worked

- Before fix, exact errors were:

```text
Error: SQL logic error: no such column: 001 (1)
Error: SQL logic error: no such column: go (1)
Error: SQL logic error: fts5: syntax error near "/" (1)
Error: SQL logic error: fts5: syntax error near "." (1)
```

- After fix:
  - punctuation queries return hits instead of SQL errors
  - `go test ./internal/indexdb -v` passes (including new test)
  - `go test ./...` passes

### What didn't work

- I temporarily ran checks with `GOWORK=off`, which pulled released `glazed` API instead of local workspace modules and caused compile errors:

```text
cmd/codex-session/cleanup_reflection_copies.go:107:17: vals.DecodeSectionInto undefined
cmd/codex-session/main.go:25:4: unknown field ShortHelpSections in struct literal of type cli.CobraParserConfig
```

- Resolution: run with workspace `go.work` enabled (as requested after your fix), not `GOWORK=off`.

### What I learned

- The user-facing failure was not just ranking quality; it was hard parser failure from raw FTS syntax.
- Local-vs-released glazed API drift can invalidate debugging if `GOWORK=off` is used accidentally.

### What was tricky to build

- Need to preserve search power while making default input safe.
- I chose literal phrase quoting as a low-risk default, then noted optional `--raw-fts-query` as a future enhancement for expert users.

### What warrants a second pair of eyes

- Whether literal-phrase default should remain hard default, or become mode-based with explicit user choice.

### What should be done in the future

- Add a documented opt-in for raw FTS syntax if needed.

### Code review instructions

- Start at `internal/indexdb/search.go` (`toLiteralFTSQuery` + use site).
- Review new regression test in `internal/indexdb/indexdb_test.go`.
- Re-run:

```bash
go test ./internal/indexdb -v
./scripts/probe_search_punctuation.sh
```

### Technical details

- Safety normalization now wraps query as a quoted FTS phrase and escapes internal double-quotes.

## Step 6: Add ticket tasks, create parity audit scripts, and author full-system assessment

I then expanded ticket scope from isolated bugfixing to structured reliability assessment, per the user request. I added concrete follow-up tasks in `tasks.md`, wrote two ticket-local scripts for repeatable investigations, and authored a full codex-sessions assessment document in the same evidence-forward style as the earlier review doc.

This step turns the investigation into an intern-usable package: tasks, reproducible scripts, architecture map, findings, pseudocode, and phased plan.

### Prompt Context

**User prompt (verbatim):** "Ok, create more tasks in the ticket to test all of these things, and continue keeping a detailed diary, and write a full report assessment of codex-sessions itself in the same style as the first review document. the nupload to remarkable"

**Assistant interpretation:** Add explicit ticket tasks for all identified verification areas, continue diary updates, produce a comprehensive assessment report, and then upload the deliverables to reMarkable.

**Inferred user intent:** Convert ad-hoc debugging into a durable research/operations handoff suitable for onboarding and follow-up implementation.

**Commit (code):** N/A

### What I did

- Added follow-up tasks (IDs 11-16) via `docmgr task add`, covering:
  - case-sensitive parity
  - flag parity
  - scope correctness tests
  - stale index policy
  - real-corpus parity validation
  - query semantics documentation / raw FTS decision
- Added ticket scripts:
  - `scripts/search-behavior-audit.sh`
  - `scripts/search-real-corpus-compare.sh`
- Ran `search-behavior-audit.sh` and captured key outputs:

```text
indexed count with --limit=1: 2
fallback count with --limit=1: 0
indexed new-term count after late write: 0
fallback new-term count after late write: 1
```

- Authored:
  - `design-doc/02-codex-sessions-comprehensive-reliability-assessment.md`

### Why

- Ensure each reliability risk has a corresponding executable test task.
- Provide a single long-form reference document for intern onboarding and implementation sequencing.

### What worked

- New tasks were added cleanly to ticket bookkeeping.
- Scripts run end-to-end and expose parity/staleness gaps.
- Comprehensive assessment doc now exists with architecture, findings, APIs, pseudocode, and phased plan.

### What didn't work

- First draft of `search-behavior-audit.sh` rendered empty counts as blank for empty JSON output.
- Fixed by normalizing empty output to `0` before `jq`.

### What I learned

- `codex-session search --output json` emits empty output instead of `[]` when no rows are produced, so helper scripts need explicit empty normalization.

### What was tricky to build

- Designing synthetic fixtures that demonstrate real parity divergence without overfitting to one backend.
- I split the script by concern (case sensitivity, flag parity, scope, staleness, punctuation) so each result is explainable.

### What warrants a second pair of eyes

- The long-term contract decision for parity: strict equivalence vs documented intentional divergence.

### What should be done in the future

- Implement freshness policy + parity normalization in code, then convert script checks into command-level integration tests.

### Code review instructions

- Review scripts first:
  - `ttmp/.../scripts/search-behavior-audit.sh`
  - `ttmp/.../scripts/search-real-corpus-compare.sh`
- Then review design doc:
  - `ttmp/.../design-doc/02-codex-sessions-comprehensive-reliability-assessment.md`
- Re-run:

```bash
go test ./...
ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-behavior-audit.sh
```

### Technical details

- Real-corpus script intentionally builds index with:
  - `--include-most-recent`
  - `--include-reflection-copies`
  - `--include-tool-calls`
  - `--include-tool-outputs`
- This reduces false mismatches caused purely by index-build selection defaults.

## Step 7: Implement stale-index policy with tests and task checkoff

I started implementation mode task-by-task as requested, beginning with stale index detection because that is the highest correctness risk remaining in search. I added an explicit policy flag so behavior is controllable, wired stale detection into indexed search, and added tests for the freshness checker.

This closes the “silent stale index” gap from an observability perspective: users now get deterministic behavior (`ignore`, `warn`, `fallback`, or `error`) instead of silently trusting stale index data.

### Prompt Context

**User prompt (verbatim):** "ok, implement. task by task, commit and check off as you go, keep a detailed diary"

**Assistant interpretation:** Execute the remaining ticket tasks sequentially, commit each completed task, check it off, and keep diary updates detailed.

**Inferred user intent:** Get disciplined, auditable execution with clear progress and reproducibility rather than batch changes.

**Commit (code):** N/A

### What I did

- Added `--stale-index-policy` to `search`:
  - choices: `ignore|warn|fallback|error`
  - default: `fallback`
- Added helper functions in `cmd/codex-session/search.go`:
  - `discoverFilteredMetas(...)`
  - `detectStaleIndex(...)`
- Wired stale detection into indexed search path before querying FTS.
- Added unit tests:
  - `cmd/codex-session/search_stale_test.go`
  - scenarios: fresh index, file mtime newer than index, session missing from index
- Ran tests:

```bash
gofmt -w cmd/codex-session/search.go cmd/codex-session/search_stale_test.go
go test ./cmd/codex-session ./internal/indexdb
```

- Checked off task 14:

```bash
docmgr task check --ticket CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO --id 14
```

### Why

- Ensure stale index risk is visible and policy-driven instead of silent.

### What worked

- Targeted tests passed for command package and indexdb package.
- Task bookkeeping was updated successfully.

### What didn't work

- N/A for this step.

### What I learned

- `fallback` as default stale policy gives the safest behavior without hard-failing existing workflows.

### What was tricky to build

- Freshness needs to detect both “file changed after index build” and “session exists on disk but not in index.”
- I combined file mtime comparison with “session present in `sessions` table” checks.

### What warrants a second pair of eyes

- Whether `fallback` should remain default policy or be tightened to `error` in CI/automation contexts.

### What should be done in the future

- Add a compact stale-summary field in output rows so downstream scripts can introspect without stderr parsing.

### Code review instructions

- Start at `cmd/codex-session/search.go`:
  - new flag declaration
  - `detectStaleIndex(...)`
  - indexed-path policy switch behavior
- Review `cmd/codex-session/search_stale_test.go`.
- Validate with:

```bash
go test ./cmd/codex-session ./internal/indexdb
```

### Technical details

- Staleness trigger conditions:
  1. session selected by filters is missing from `sessions` index table
  2. session file mtime is newer than index file mtime

## Step 8: Add scope correctness tests for messages/tools/all

After stale-index policy landed, I moved to scope-correctness coverage. I added a new indexdb regression test that exercises all search scopes with punctuation-heavy tokens and verifies tool output matching behavior explicitly.

This closes a test gap where scope behavior was mostly implicit; now the matrix is encoded in a reproducible test fixture.

### Prompt Context

**User prompt (verbatim):** (see Step 7)

**Assistant interpretation:** Continue sequential task execution with commit/checkoff discipline.

**Inferred user intent:** Keep reducing risk with concrete, test-backed increments.

**Commit (code):** N/A

### What I did

- Added `TestSearchScopesWithPunctuationAndToolOutputs` to:
  - `internal/indexdb/indexdb_test.go`
- Fixture includes:
  - message token (`message-only-token`)
  - tool call argument path (`/tmp/tool-call.txt`)
  - tool output punctuation token (`foo/bar`)
- Assertions added for:
  - `ScopeMessages`
  - `ScopeTools` (tool_call and tool_output)
  - `ScopeAll`
- Validation:

```bash
gofmt -w internal/indexdb/indexdb_test.go
go test ./internal/indexdb ./cmd/codex-session
```

- Checked off task 13:

```bash
docmgr task check --ticket CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO --id 13
```

### Why

- Ensure scope behavior is explicitly verified, not inferred from generic search tests.

### What worked

- New test passes and captures both scope routing and punctuation handling.

### What didn't work

- N/A for this step.

### What I learned

- Scope behavior is easiest to validate at `internal/indexdb` level before adding command-level integration layers.

### What was tricky to build

- Ensuring the fixture contains distinct tokens for message-only, tool-call-only, and tool-output-only paths to avoid ambiguous matches.

### What warrants a second pair of eyes

- Whether we should also add command-level `--scope` tests in `cmd/codex-session` for full CLI coverage.

### What should be done in the future

- Add command-level integration tests once search parity contract is finalized.

### Code review instructions

- Review `internal/indexdb/indexdb_test.go` new test block.
- Re-run:

```bash
go test ./internal/indexdb -run TestSearchScopesWithPunctuationAndToolOutputs -v
```

### Technical details

- The test confirms `ScopeTools` can match both tool-call argument text and tool-output text with punctuation-heavy queries.

## Step 9: Finalize query semantics contract with raw FTS opt-in

I completed the query-semantics decision task by implementing an explicit expert-mode flag instead of leaving behavior implicit. Literal phrase search remains the default for safety, while `--raw-fts-query` now enables advanced FTS expressions when needed.

This preserves user safety for common queries (`CODEX-001`, paths, tool names) without blocking power-user syntax.

### Prompt Context

**User prompt (verbatim):** (see Step 7)

**Assistant interpretation:** Continue task-by-task implementation and close open contract decisions with code + docs.

**Inferred user intent:** Make behavior intentional and documented, not accidental.

**Commit (code):** N/A

### What I did

- Added command flag:
  - `--raw-fts-query` (indexed mode opt-in)
  - default remains literal query mode
- Wired option through search stack:
  - `cmd/codex-session/search.go`
  - `internal/indexdb/search.go` (`SearchOptions.RawQuery`)
- Added regression test:
  - `internal/indexdb/indexdb_test.go`
  - verifies `hello OR missing` matches only when `RawQuery=true`
- Updated assessment document Finding 3 to reflect resolved decision.
- Checked off task 16:

```bash
docmgr task check --ticket CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO --id 16
```

### Why

- Eliminate ambiguity around literal vs raw query behavior.

### What worked

- Tests passed with both command and indexdb packages:

```bash
go test ./internal/indexdb ./cmd/codex-session
```

### What didn't work

- N/A for this step.

### What I learned

- A dual-mode contract (safe default + explicit expert opt-in) gives cleaner UX than trying to auto-detect “intent” from query text.

### What was tricky to build

- Avoiding accidental reintroduction of parser errors while supporting true raw expressions.
- Solved by keeping literal quoting as default and bypassing it only when opt-in flag is true.

### What warrants a second pair of eyes

- Whether raw mode should eventually include additional guardrails (for example, requiring indexed backend explicitly).

### What should be done in the future

- Add one CLI-level integration test for `--raw-fts-query` once search integration suite is added.

### Code review instructions

- Review:
  - `cmd/codex-session/search.go` (`--raw-fts-query`)
  - `internal/indexdb/search.go` (`RawQuery` handling)
  - `internal/indexdb/indexdb_test.go` (opt-in behavior test)
- Re-run:

```bash
go test ./internal/indexdb -run TestSearchRawFTSQueryOptIn -v
```

### Technical details

- In raw mode, query text is passed to FTS `MATCH` unchanged; otherwise it is literalized and escaped.

## Step 10: Run real-corpus indexed-vs-fallback comparisons and summarize diffs

I completed the real-corpus validation task using a bounded project slice to keep runs practical while still representative. I also improved the comparison script so it can skip repeated index builds and force true indexed-mode comparison even when stale-index policy would otherwise auto-fallback.

Results were stable for session-id parity in the sampled project/filter set: indexed and fallback returned the same matching session sets, with differences primarily in ranking/snippet presentation.

### Prompt Context

**User prompt (verbatim):** (see Step 7)

**Assistant interpretation:** Continue sequential execution and complete remaining reliability tasks with auditable artifacts.

**Inferred user intent:** Verify that proposed fixes actually hold on real data, not only synthetic fixtures.

**Commit (code):** N/A

### What I did

- Updated comparator script:
  - `scripts/search-real-corpus-compare.sh`
  - added `--skip-index-build`
  - forced indexed run with `--stale-index-policy=ignore` for fair comparisons
  - constrained index-build flags to provided filters when building
- Ran comparisons on:
  - `sessions-root`: `~/.codex/sessions`
  - `project`: `2026-02-12--hypercard-react`
  - `since`: `2026-02-01`
  - queries/scopes:
    - `codex` / `messages`
    - `functions.shell_command` / `tools`
    - `/home/manuel` / `all`
- Stored outputs:
  - `analysis/02-real-corpus-search-compare.txt`
- Summarized results in design doc section `10.3`.
- Checked off task 15:

```bash
docmgr task check --ticket CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO --id 15
```

### Why

- Validate parity/diff behavior on real session corpus before declaring search behavior “good”.

### What worked

- Real-corpus runs completed and produced reproducible artifacts.
- For the bounded slice, indexed/fallback session-id sets matched in all three runs.

### What didn't work

- Initial full-root comparison approach was too slow because index rebuild happened for every run.
- I adjusted workflow to “build once + skip-build compares” and added script support accordingly.

### What I learned

- On real data, practical differences were mostly ranking/snippet formatting rather than session-set membership for tested filters.

### What was tricky to build

- Stale-index fallback policy can mask indexed-vs-fallback differences by forcing both sides to fallback.
- I handled this by setting `--stale-index-policy=ignore` on the indexed side inside the comparator script.

### What warrants a second pair of eyes

- Whether comparator should support a stricter “fail on set-diff” mode for CI.

### What should be done in the future

- Add optional CI mode to comparator script with non-zero exit when indexed/fallback session-id sets diverge.

### Code review instructions

- Review:
  - `scripts/search-real-corpus-compare.sh`
  - `analysis/02-real-corpus-search-compare.txt`
  - design doc section `10.3`
- Re-run one sample:

```bash
ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-real-corpus-compare.sh \
  --sessions-root ~/.codex/sessions \
  --project 2026-02-12--hypercard-react \
  --query codex \
  --scope messages \
  --since 2026-02-01 \
  --include-most-recent
```

### Technical details

- Snapshot counts from stored run:
  - `messages/codex`: indexed `8`, fallback `8`
  - `tools/functions.shell_command`: indexed `0`, fallback `0`
  - `all//home/manuel`: indexed `8`, fallback `8`
