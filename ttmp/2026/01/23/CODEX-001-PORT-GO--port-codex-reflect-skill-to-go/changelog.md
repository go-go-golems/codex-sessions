# Changelog

## 2026-01-23

- Initial workspace created


## 2026-01-24

Added detailed Go-port design docs, Glazed notes, diary entries, and expanded implementation task list.

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/design-doc/01-analysis-current-python-session-reflection-cli.md — Baseline behavior analysis
- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/design-doc/02-design-go-conversation-parsing-indexing-and-glazed-cli.md — Go architecture + CLI design
- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/02-glazed-notes-build-first-command.md — Glazed patterns reference
- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/tasks.md — Detailed phased checklist


## 2026-01-24

Bootstrapped Go module deps, added internal session discovery/meta parsing, and wired first Glazed command: codex-sessions projects (commit d4dcafc).

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/cmd/codex-sessions/projects.go — First Glazed command emitting project counts
- /home/manuel/code/others/llms/Codex-Reflect-Skill/internal/sessions/discover.go — Rollout JSONL discovery
- /home/manuel/code/others/llms/Codex-Reflect-Skill/internal/sessions/parser.go — Minimal session_meta parser (new+legacy)

## 2026-01-24

Checked off initial build/run + gitignore tasks and corrected changelog entry formatting to avoid shell backticks.

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/changelog.md — Fix backtick-substitution damage
- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/tasks.md — Mark build/run and gitignore tasks complete


## 2026-01-24

Added streaming JSONL scan (line metadata + raw retention), derived conversation updated_at/title, and implemented codex-sessions list (commit 15e3b6a).

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/cmd/codex-sessions/list.go — Glazed list command
- /home/manuel/code/others/llms/Codex-Reflect-Skill/internal/sessions/conversation.go — updated_at + title derivation
- /home/manuel/code/others/llms/Codex-Reflect-Skill/internal/sessions/jsonl.go — Streaming JSONL walker

## 2026-01-24

Implemented normalized message extraction and added codex-sessions show (commit f52ca3e).

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/cmd/codex-sessions/show.go — Show command (timeline rows)
- /home/manuel/code/others/llms/Codex-Reflect-Skill/internal/sessions/messages.go — Normalized message extraction


## 2026-01-24

Added codex-sessions search (non-indexed streaming scan) to find sessions/messages by substring (commit 9615a87).

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/cmd/codex-sessions/search.go — Streaming scan search command


## 2026-01-24

Ran Go CLI smoke tests against real ~/.codex/sessions and added a test report doc to the ticket.

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/03-test-report-go-cli-smoke-test-codex-sessions.md — Smoke test results


## 2026-01-24

Uploaded Go CLI smoke test report to reMarkable under /ai/2026/01/25/CODEX-001-PORT-GO as CODEX-001-Go-CLI-Smoke-Test.pdf.

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/03-test-report-go-cli-smoke-test-codex-sessions.md — Report uploaded to reMarkable


## 2026-01-24

Implemented Phase 3 facet extraction (texts/tools/paths/errors) and wired it into codex-sessions show --view; improved search snippet display (commit 99a6340).

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/cmd/codex-sessions/search.go — Single-line snippets
- /home/manuel/code/others/llms/Codex-Reflect-Skill/cmd/codex-sessions/show.go — Facet views
- /home/manuel/code/others/llms/Codex-Reflect-Skill/internal/sessions/facets.go — Facet extraction
- /home/manuel/code/others/llms/Codex-Reflect-Skill/internal/sessions/patterns.go — Path/error heuristics

## 2026-01-24

Add export command and robust tool facet extraction (commit 39e2894)

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/cmd/codex-sessions/export.go — New export command for normalized document/rows output
- /home/manuel/code/others/llms/Codex-Reflect-Skill/internal/sessions/facets.go — Extract tool calls/outputs from custom_tool_call payloads and correlate outputs by call_id


## 2026-01-24

Update smoke test report for export/tool facets and upload follow-up PDF to reMarkable

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/03-test-report-go-cli-smoke-test-codex-sessions.md — Added follow-up run results for commit 39e2894 and exported artifacts


## 2026-01-25

Add SQLite/FTS indexing: index build/stats and index-backed search (commit e9d44ff)

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/cmd/codex-sessions/index_build.go — Glazed index build command
- /home/manuel/code/others/llms/Codex-Reflect-Skill/cmd/codex-sessions/search.go — Search now uses index when available
- /home/manuel/code/others/llms/Codex-Reflect-Skill/internal/indexdb/schema.go — SQLite schema (sessions/messages/tools/paths/errors + FTS)


## 2026-01-25

Update smoke test report for SQLite/FTS indexing and upload PDF to reMarkable

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/03-test-report-go-cli-smoke-test-codex-sessions.md — Added index follow-up run section and captured artifacts


## 2026-01-25

Add edge cases + limitations reference doc (task 69)

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/04-known-edge-cases-and-limitations.md — Documented index-vs-scan semantics and heuristic extraction limitations


## 2026-01-25

Implement reflect command with prompt selection/versioning, cache semantics, and codex resume execution (commit 80e630b)

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/cmd/codex-sessions/reflect.go — New reflect command and cache output fields
- /home/manuel/code/others/llms/Codex-Reflect-Skill/internal/reflect — Prompt/cache/codex/copy implementation


## 2026-01-25

Update smoke test report for reflect and upload PDF to reMarkable

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/03-test-report-go-cli-smoke-test-codex-sessions.md — Added reflect follow-up run section and uploaded PDF


## 2026-01-25

Check off remaining umbrella task for CLI command suite (task 59); all tasks complete

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/tasks.md — All tasks checked


## 2026-01-25

Run full-archive index build and record perf snapshot in test report

### Related Files

- /home/manuel/code/others/llms/Codex-Reflect-Skill/ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/reference/03-test-report-go-cli-smoke-test-codex-sessions.md — Added full index build (406 sessions) + perf snapshot


## 2026-01-25

Ticket closed

