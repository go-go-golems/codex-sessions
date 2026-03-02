# Changelog

## 2026-03-02

- Created CODEX-002 investigation workspace for `cmd/codex-session/main.go`.
- Completed wiring and runtime investigation with findings:
  - `main.go` command registration is currently complete (10 constructors, 10 registrations).
  - Default workspace startup fails due `go.work` vs module Go-version mismatch.
  - `main.go` has high duplication and no dedicated wiring tests.
  - Root-level glazed-style flags are not available at root command.
- Added reproducible audit script:
  - `scripts/main-go-wiring-audit.sh`
- Authored detailed design analysis and investigation diary.

## 2026-03-02

Completed main.go wiring investigation with reproducible audit script and evidence-backed findings.

### Related Files

- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/cmd/codex-session/main.go — Investigated command registration and parser wiring
- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/main-go-wiring-audit.sh — Reproducible evidence collection


## 2026-03-02

Uploaded CODEX-002 investigation bundle to reMarkable: /ai/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO

### Related Files

- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/design-doc/01-codex-session-main-go-failure-analysis.md — Included in uploaded bundle
- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/reference/01-investigation-diary.md — Included in uploaded bundle


## 2026-03-02

Fixed indexed search punctuation-query failures by literalizing FTS input and added regression tests.

### Related Files

- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/internal/indexdb/indexdb_test.go — Regression tests for CODEX IDs
- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/internal/indexdb/search.go — Literal FTS query normalization for safe user input


## 2026-03-02

Expanded ticket to full codex-sessions reliability assessment, added parity tasks, and created synthetic/real-corpus search audit scripts.

### Related Files

- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/design-doc/02-codex-sessions-comprehensive-reliability-assessment.md — Full assessment with architecture
- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-behavior-audit.sh — Synthetic parity and stale-index reproducibility
- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/scripts/search-real-corpus-compare.sh — Real-corpus indexed-vs-fallback comparator


## 2026-03-02

Uploaded expanded CODEX-002 reliability bundle to reMarkable at /ai/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO.

### Related Files

- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/design-doc/02-codex-sessions-comprehensive-reliability-assessment.md — Included in uploaded bundle
- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/ttmp/2026/03/02/CODEX-002-ANALYZE-CODEX-SESSION-MAIN-GO--analyze-codex-session-main-go-command-wiring/reference/01-investigation-diary.md — Included in uploaded bundle


## 2026-03-02

Implemented stale-index policy in search with modes (ignore/warn/fallback/error) and added stale detection tests.

### Related Files

- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/cmd/codex-session/search.go — Added stale-index-policy flag and stale-index fallback/error behavior
- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/cmd/codex-session/search_stale_test.go — Unit coverage for stale index detection (fresh


## 2026-03-02

Added scope correctness tests for messages/tools/all including punctuation-heavy tool-call and tool-output queries.

### Related Files

- /home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions/internal/indexdb/indexdb_test.go — New scope-focused regression test for punctuation and tool outputs

