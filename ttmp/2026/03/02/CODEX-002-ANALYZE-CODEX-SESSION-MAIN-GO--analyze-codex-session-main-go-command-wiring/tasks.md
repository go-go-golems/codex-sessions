# Tasks

## Completed

- [x] Create new CODEX-002 ticket workspace and initialize docs.
- [x] Locate and analyze `cmd/codex-session/main.go` wiring.
- [x] Run runtime checks for root and nested command help.
- [x] Reproduce workspace startup failure and isolate root cause.
- [x] Create ticket-local reproducible audit script.
- [x] Write detailed design report and chronological diary.

## Follow-up

- [x] Decide whether to align `go.work` Go version or standardize `GOWORK=off` for this workspace.
- [x] Refactor `main.go` with a shared command-registration helper to remove repetition.
- [x] Add command-tree wiring tests (e.g., `main_wiring_test.go`).
- [x] Decide and document whether root-level glazed flags should be supported.
- [x] Validate --case-sensitive parity between indexed and fallback search paths (including tools scope behavior).
- [x] Audit flag parity: document and test differences for --include-most-recent, --include-reflection-copies, and --limit between indexed and fallback modes.
- [x] Add scope correctness tests for --scope messages|tools|all including punctuation-heavy queries and tool outputs.
- [x] Design and test index freshness detection so stale SQLite index usage is visible (warning/error/policy).
- [x] Run real-corpus validation comparing indexed search vs --use-index=false across project/since/until filters and summarize diffs.
- [x] Document query semantics (literal phrase vs raw FTS) and decide whether to add a --raw-fts-query opt-in flag.
