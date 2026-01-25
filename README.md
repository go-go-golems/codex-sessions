# codex-session

Query, index, export, and reflect on Codex session histories stored in `~/.codex/sessions`.

## What it does

- **Discover + list** sessions by project/date (`codex-session list`, `codex-session projects`)
- **Search** across sessions:
  - fast with an optional **SQLite/FTS** index (`codex-session index build`, `codex-session search`)
  - fallback streaming scan when an index isn’t present
- **Export** normalized shapes for downstream tooling (`codex-session export`)
- **Generate reflections** via the `codex` CLI with caching (`codex-session reflect`)
- **Clean up reflection copies** (dry-run by default; delete or trash) (`codex-session cleanup reflection-copies`)
- **Generate trace reports** as readable Markdown (`codex-session traces md`)

## Install

### Homebrew

This repo is intended to be released via GoReleaser and published to the go-go-golems Homebrew tap.

```bash
brew tap go-go-golems/go-go-go
brew install codex-session
```

### Go install (from source)

```bash
go install github.com/go-go-golems/codex-session/cmd/codex-session@latest
```

## Quick start

```bash
codex-session projects --output table
codex-session list --limit 10 --output table
codex-session index build --output table
codex-session search --query "TODO" --output table
```

Trace report:

```bash
codex-session traces md --limit 2 --entries-per-file 10 --md-output trace_examples.md
```

Reflection (uses `codex exec resume ...` under the hood):

```bash
codex-session reflect --limit 5 --output table
```

## Development

```bash
make lint
make test
make build
```

Pre-commit hooks are managed via `lefthook.yml`:

```bash
lefthook install
```

Snapshot release:

```bash
make goreleaser
```

## Security notes

This tool reads local session histories (often containing sensitive data). Be careful when enabling indexing of tool outputs and when sharing exported data.

## License

MIT. See `LICENSE`.

