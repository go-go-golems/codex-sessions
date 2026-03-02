---
Title: Codex Session main.go Failure Analysis
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
    - Path: codex-sessions/cmd/codex-session/main.go
      Note: Primary command wiring under investigation
    - Path: codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/main-go-wiring-audit.sh
      Note: Repro script for wiring and startup checks
    - Path: glazed/go.mod
      Note: Evidence for higher Go version requirement
    - Path: go-go-goja/go.mod
      Note: Evidence for higher Go version requirement
    - Path: go.work
      Note: Workspace go version setting that blocks default startup
ExternalSources: []
Summary: Investigation of cmd/codex-session/main.go wiring behavior, runtime failure modes, and maintainability risks.
LastUpdated: 2026-03-02T13:35:00-05:00
WhatFor: Explain what is broken (or fragile) in codex-session main.go and provide remediation guidance.
WhenToUse: Debug CLI startup/wiring behavior or refactor command registration.
---


# Codex Session main.go Failure Analysis

## 1. Executive summary

`cmd/codex-session/main.go` is functionally wired today, but there are two concrete problems around this area:

1. **In this workspace, normal CLI execution fails before main wiring runs** because `go.work` declares `go 1.25` while workspace modules require `go >= 1.25.5` and `1.25.7`.
2. **`main.go` has high wiring fragility**: command registration and parser config are copy-pasted in ten places, and there are no tests that validate command-tree wiring.

No direct runtime bug was found in command registration itself (constructor count equals registrations, and subcommand help works under `GOWORK=off`), but this part is a high-risk hotspot for future regressions.

## 2. Problem statement and scope

### Scope

- Analyze:
  - `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/cmd/codex-session/main.go`
- Correlate with:
  - command constructors under `cmd/codex-session/*.go`
  - workspace execution behavior (`go.work`, module versions)

### Out of scope

- Rewriting command business logic (`search`, `reflect`, etc.)
- Packaging/release behavior beyond startup impacts

## 3. Current-state architecture

### 3.1 Wiring flow

```text
main.go
  -> create root cobra.Command("codex-session")
  -> New*Command() for each feature command
  -> cli.BuildCobraCommand(cmd, parser-config)
  -> attach to root or group (index/cleanup/traces)
  -> rootCmd.Execute()
```

### 3.2 Command tree as currently wired

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

### 3.3 Evidence anchors

- Root definition and repeated build pattern:
  - `cmd/codex-session/main.go:13-33`
- Group command wiring:
  - `cmd/codex-session/main.go:120-205`
- Execute:
  - `cmd/codex-session/main.go:206-208`

## 4. Findings

## Finding A (high): Workspace startup fails by default

### Symptom

`go run ./cmd/codex-session --help` fails in this checkout unless `GOWORK=off` is used.

### Evidence

Observed error:

```text
go: module ../go-go-goja listed in go.work file requires go >= 1.25.7, but go.work lists go 1.25
```

Config mismatch evidence:

- `go.work` declares `go 1.25` (`/home/manuel/workspaces/2026-03-02/fix-codex-sessions/go.work:1`)
- `go-go-goja/go.mod` declares `go 1.25.7` (`.../go-go-goja/go.mod:3`)
- `glazed/go.mod` declares `go 1.25.7` (`.../glazed/go.mod:3`)
- `codex-sessions/go.mod` declares `go 1.25.5` (`.../codex-sessions/go.mod:3`)

### Impact

- Developers may perceive CLI/main wiring as broken because the binary cannot be started in normal workspace mode.
- Investigation/debug loops become misleading unless `GOWORK=off` is known and applied.

### Root cause

Environment/toolchain workspace configuration mismatch, not business logic in `main.go`.

## Finding B (medium): main.go wiring is correct now but highly fragile

### Symptom

`main.go` duplicates the same registration ceremony and parser config repeatedly.

### Evidence

- 10 command constructors found in package.
- 10 `BuildCobraCommand(...)` registrations found in `main.go`.
- 10 repeated parser config blocks.

From audit script output:

```text
constructors=10
build_cobra_registrations=10
parser_config_repetitions=10
```

### Impact

- Adding/removing commands requires manual edits in many spots.
- Future copy/paste drift can silently break one subcommand’s parser config or registration order.
- Hard to review for completeness because registration logic is spread linearly.

### Root cause

No abstraction for “register glazed command with default parser config.”

## Finding C (medium): No tests cover command-tree wiring

### Symptom

There is no test asserting root command structure/registration completeness.

### Evidence

Audit query returned:

```text
no_main_wiring_tests_detected
```

### Impact

- Regressions in CLI command tree can pass unit tests in feature packages.
- Missing/renamed commands may only be discovered manually.

## Finding D (low): Root-level glazed-style flags are unavailable

### Symptom

Flags like `--print-schema` at root fail with unknown flag.

### Evidence

```text
codex-session --print-schema
Error: unknown flag: --print-schema
```

### Impact

- Potential UX inconsistency for users expecting global glazed flags at root.
- Not necessarily a bug, but a behavior contract worth documenting explicitly.

## 5. Reproduction assets

Ticket-local script:

- `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/main-go-wiring-audit.sh`

What it does:

1. Counts constructors and main registrations.
2. Runs CLI with workspace go.work (shows failure).
3. Runs CLI with `GOWORK=off` (baseline success).
4. Demonstrates root-flag behavior.
5. Checks for wiring-focused tests.

## 6. Proposed solution

## Phase 1: Stabilize local startup expectations

1. Align `go.work` Go version with workspace modules (or document `GOWORK=off` in repo dev instructions).
2. Add a short troubleshooting section in README for workspace-mode failures.

## Phase 2: De-duplicate command wiring in main.go

Introduce helper functions:

```go
func mustAddGlazedCommand(root *cobra.Command, label string, ctor func() (cmds.Command, error))
func mustAddGlazedSubcommand(parent *cobra.Command, label string, ctor func() (cmds.Command, error))
```

Or shared builder:

```go
func buildGlazedCobraCommand(c cmds.Command) (*cobra.Command, error) {
  return cli.BuildCobraCommand(c, cli.WithParserConfig(defaultParserConfig()))
}
```

Benefits:

- Single source of truth for parser config.
- Smaller and safer `main.go`.
- Easier review diffs when adding/removing commands.

## Phase 3: Add wiring tests

Add `cmd/codex-session/main_wiring_test.go` to validate:

1. expected top-level commands exist,
2. grouped commands (`index`, `cleanup`, `traces`) include required children,
3. command constructors are all registered.

Pseudocode:

```text
build root command via exported buildRootCommand()
collect command names recursively
assert contains: projects,list,show,search,export,reflect,index,cleanup,traces
assert index has build+stats
assert cleanup has reflection-copies
assert traces has md
```

## 7. API and code-structure recommendations

To make wiring testable, refactor main entrypoint:

- Keep `func main()` minimal:

```go
func main() {
  root, err := buildRootCommand()
  if err != nil { ... }
  if err := root.Execute(); err != nil { os.Exit(1) }
}
```

- Introduce:

```go
func buildRootCommand() (*cobra.Command, error)
```

This allows tests to inspect command tree without invoking process exit.

## 8. Testing strategy

### Immediate checks (already run)

1. `go run ./cmd/codex-session --help` (workspace mode) -> fails due go.work mismatch.
2. `GOWORK=off go run ./cmd/codex-session --help` -> success.
3. `GOWORK=off go run ./cmd/codex-session search --help` -> success.
4. `GOWORK=off go test ./...` -> pass.

### Regression checks to add

1. `go test ./cmd/codex-session -run TestMainWiring`
2. CLI smoke script in CI for command tree + core helps.

## 9. Risks, alternatives, open questions

### Risks

1. Refactoring main wiring without tests could accidentally drop command registration.
2. Keeping current duplication increases long-term drift risk.

### Alternatives

1. Keep current main.go and only add docs.
   - Low effort, but fragility remains.
2. Full CLI framework overhaul.
   - Overkill for current issue.

### Open questions

1. Is `go.work` meant for this repo’s daily dev flow, or should docs standardize `GOWORK=off`?

### Decision update

Root-level glazed flags are intentionally not supported. The root command is a grouping node; glazed-style flags are expected on executable subcommands (documented in `README.md`).

## 10. References

Primary files:

- `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/cmd/codex-session/main.go`
- `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/go.work`
- `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/go.mod`
- `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/go-go-goja/go.mod`
- `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/glazed/go.mod`

Ticket scripts:

- `/home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/main-go-wiring-audit.sh`
