# Codex Reflect Skill

Generate reflections for past Codex session histories using the AutoSkill Reflection CLI.
This skill helps surface repeated patterns, friction, and potential skill ideas from
prior Codex conversations.

## What’s included

- `SKILL.md`: guidance on when and how to use the skill
- `scripts/`: the CLI tools (`reflect_sessions.py` and helpers)
- `references/`: command catalog, usage notes, and examples

## Requirements

- Python 3.10+
- A local Codex session archive in `~/.codex/sessions`
- The `codex` binary available on your PATH (or pass `--codex-path`)

## Quick start

Run commands from `scripts/`:

```bash
python3 reflect_sessions.py --output -
```

Human-readable output:

```bash
python3 reflect_sessions.py --output-style human --output -
```

Project filter example:

```bash
python3 reflect_sessions.py --project ExampleProject --output -
```

## Notes on privacy

The CLI reads local session histories from `~/.codex/sessions`. Reflections may contain
sensitive content from those sessions. Review outputs before sharing them publicly.

## License

MIT. See `LICENSE`.
