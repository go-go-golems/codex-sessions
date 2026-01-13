---
name: reflect-codex-skills
description: Generate reflections for past Codex session histories using the AutoSkill Reflection CLI. Use when asked to summarize or reflect on previous Codex conversations, list projects/sessions, filter by date or session id, or refresh cached reflections from ~/.codex/sessions.
---

# Reflect Codex Sessions

## When to use this skill

- Summarize or reflect on past Codex session histories.
- List projects, session counts, or filter by date/session id.
- Retrieve/Refresh cached reflections or pull extra metadata for auditing.

## Interpretation guidance (important)

- Treat reflection content as heuristics, not facts or ideas from a user. Use it as a starting point.
- Surface only non-niche, broadly useful themes and repeated patterns and confirm with the
user before acting on a pattern.
- Avoid over-indexing on one-offs; ask for confirmation when a pattern is uncertain.

## Quick start

- Run commands from `scripts/` inside this skill directory.
- Default: `python3 reflect_sessions.py --output -`.
- Use `--output-style human` when replying in chat; use JSON for downstream parsing.

## Workflow

1. **Choose scope**: `--project`, `--since`, `--until`, `--session-id(s)`, `--limit`.
   Note: the most recent session is skipped unless `--include-most-recent`.
2. **Generate reflections**: `python3 reflect_sessions.py ...` and adjust
   `--refresh-mode`, `--prompt-file`, or Codex flags only when asked.
3. **Respond carefully**: highlight repeated, non-niche patterns and keep claims
   tentative unless the user confirms them.

## References

- `references/cli.md` for the command catalog and full flag list.
- `references/README.md` for system behavior, cache semantics, and output schema.
- `references/examples.md` for an end-to-end example from ExampleProject.
