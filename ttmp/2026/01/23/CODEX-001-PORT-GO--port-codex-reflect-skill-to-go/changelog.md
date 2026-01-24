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

