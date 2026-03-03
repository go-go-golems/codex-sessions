---
Title: Command Reference and Examples
Slug: codex-session-reference-examples
Short: Practical command reference with runnable examples for discovery, indexing, search, export, reflection, cleanup, and traces.
Topics:
- codex-session
- reference
- examples
- cli
Commands:
- projects
- list
- show
- export
- index build
- index stats
- search
- reflect
- cleanup reflection-copies
- traces md
Flags:
- project
- since
- until
- limit
- include-most-recent
- include-reflection-copies
- use-index
- stale-index-policy
- raw-fts-query
- scope
- case-sensitive
- show-matches
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

This reference covers concrete command patterns with realistic options and output modes. It is organized for fast copy/paste use during incident response, analysis sessions, and scripted workflows.

## Selection and filtering primitives

This section covers shared selection flags and why they are central to reproducible results across commands.

- `--project`: derived project label match.
- `--since`, `--until`: inclusive date/datetime window.
- `--limit`: most-recent N after filtering.
- `--include-most-recent`: include latest session (otherwise many commands skip by default).
- `--include-reflection-copies`: include reflection-prefixed archives.

Example pattern reused across commands:

```bash
--project /home/manuel/workspaces/2026-03-02/fix-codex-sessions \
--since 2026-03-01 \
--until 2026-03-03 \
--limit 100 \
--include-most-recent
```

## Projects and list

This section covers inventory commands and why they are the first checkpoint before any expensive action.

List known projects:

```bash
codex-session projects --output table
```

List only reflection copies:

```bash
codex-session list --include-reflection-copies --limit 50 --output table
```

Machine-readable list:

```bash
codex-session list --limit 25 --output json | jq '.[] | {id, project, ts}'
```

## Show and export

This section covers detailed inspection and normalized export shapes. Use `show` for interactive diagnosis and `export` for downstream tooling.

Load by explicit file path:

```bash
codex-session show --path ~/.codex/sessions/2026/03/02/rollout-...jsonl --view timeline --output table
```

Inspect text snippets with single-line rendering:

```bash
codex-session show --session-id <SESSION_ID> --view texts --single-line --limit 20 --output table
```

Export minimal document:

```bash
codex-session export --session-id <SESSION_ID> --shape document --extract minimal --output yaml
```

Export full rows for analytics:

```bash
codex-session export --session-id <SESSION_ID> --shape rows --extract all --single-line --output json
```

## Index build and stats

This section covers index maintenance patterns and why explicit options matter for privacy and performance.

Baseline build:

```bash
codex-session index build --output table
```

Large-scope build with controlled text size:

```bash
codex-session index build \
  --project /home/manuel/workspaces \
  --limit 1000 \
  --max-chars 8000 \
  --include-tool-calls \
  --include-tool-outputs \
  --output table
```

Force refresh of selected scope:

```bash
codex-session index build --force --output table
```

Check stats:

```bash
codex-session index stats --output table
```

## Search reference

This section covers indexed vs fallback behavior and how to select the right mode for a query.

Literal text search (default):

```bash
codex-session search --query "go-go-os" --output table
```

Scope to tool calls and outputs:

```bash
codex-session search --query "functions.exec_command" --scope tools --output table
```

Show matching snippets per message:

```bash
codex-session search --query "stale-index-policy" --show-matches --max-snippet-chars 300 --output table
```

Case-sensitive run (fallback path):

```bash
codex-session search --query "RawQuery" --case-sensitive=true --use-index=false --output table
```

Safe stale handling:

```bash
codex-session search --query "CODEX-002" --stale-index-policy=fallback --output table
```

Strict stale handling:

```bash
codex-session search --query "CODEX-002" --stale-index-policy=error --output table
```

Raw FTS advanced query:

```bash
codex-session search --query 'session NEAR("index", 5)' --raw-fts-query --output table
```

## Reflect command examples

This section covers practical reflection patterns and why cache/parallel controls are important.

Dry-run candidate selection:

```bash
codex-session reflect --project /home/manuel/workspaces --limit 10 --dry-run --output table
```

Parallel reflection run:

```bash
codex-session reflect --limit 8 --max-workers 4 --prompt-preset summary --output table
```

Sequential deterministic run:

```bash
codex-session reflect --limit 5 --sequential --prompt-preset decisions --output table
```

Use custom prompt file:

```bash
codex-session reflect --limit 3 --prompt-file ./scripts/prompts/reflection.md --output table
```

## Traces and cleanup examples

This section covers reporting and hygiene operations.

Generate focused traces with metadata:

```bash
codex-session traces md \
  --project /home/manuel/workspaces/2026-03-02/fix-codex-sessions \
  --limit 3 \
  --entries-per-file 20 \
  --include-entry-metadata \
  --md-output codex_traces.md
```

Truncate oversized payload renders:

```bash
codex-session traces md --limit 2 --max-str-len 500 --max-list-len 20 --md-output compact_traces.md
```

Cleanup reflection copies with safety limit:

```bash
codex-session cleanup reflection-copies --dry-run --limit 50 --output table
```

Move copies to trash:

```bash
codex-session cleanup reflection-copies --dry-run=false --mode trash --output table
```

## Output formats and scripting patterns

This section covers output mode selection and why it matters for automation.

- Use `--output table` for interactive scans.
- Use `--output json` for scripts and CI checks.
- Use `--output yaml` for compact human-readable artifacts.

Example: assert at least one hit in CI.

```bash
count=$(codex-session search --query "CODEX-002" --output json | jq 'length')
if [ "$count" -lt 1 ]; then
  echo "expected at least one hit" >&2
  exit 1
fi
```

## Troubleshooting

| Problem | Cause | Solution |
| --- | --- | --- |
| `search --scope tools` returns no hits for tool output terms | Tool outputs were not indexed | Rebuild index with `--include-tool-outputs` |
| `reflect` fails with codex invocation errors | Wrong codex binary or approval/sandbox mismatch | Set `--codex-path`, verify `--codex-sandbox` and `--codex-approval` |
| `show --session-id` cannot find session | Session id is outside selected root or typo | Confirm session root and discover ID via `list --output table` |
| Results differ between indexed and fallback search | Index freshness or backend semantics mismatch | Use `--stale-index-policy=fallback` and compare with `--use-index=false` |
| Cleanup removes too many files | Broad prefix or filters | Start with `--dry-run` and set `--limit` before deletion |

## See Also

- `codex-session help codex-session-getting-started`
- `codex-session help codex-session-architecture`
