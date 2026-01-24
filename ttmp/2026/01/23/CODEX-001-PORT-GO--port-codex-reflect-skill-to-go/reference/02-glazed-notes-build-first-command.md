---
Title: Glazed Notes (build-first-command)
Ticket: CODEX-001-PORT-GO
Status: active
Topics:
    - backend
    - chat
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Key patterns from `glaze help build-first-command` to use Glazed for the new Go CLI."
LastUpdated: 2026-01-23T23:49:16.095061427-05:00
WhatFor: ""
WhenToUse: ""
---

# Glazed Notes (build-first-command)

## Goal

Capture the Glazed “build first command” patterns we’ll apply to the Go port, so the CLI is naturally multi-format (table/JSON/CSV/YAML) without duplicating output code.

## Context

User request explicitly asked to consult `glaze help build-first-command` by redirecting output to a file and reading it. I captured it at:

```text
/tmp/glaze_build_first_command.txt
```

This doc distills the parts that matter for the planned `codex-sessions` CLI suite.

## Quick Reference

### Core mental model

- Each command implements Glazed’s `GlazeCommand` interface (via `RunIntoGlazeProcessor(...)`).
- The command yields structured rows (`types.Row`) instead of printing directly.
- Glazed handles output formatting via standard flags (e.g. output format, field selection, sorting).

### Non-negotiable pattern: decode resolved values into a settings struct

The help doc’s strongest guidance is: always decode resolved values into a settings struct (don’t read Cobra flags directly), e.g.:

```go
settings := &MySettings{}
if err := values.DecodeSectionInto(vals, schema.DefaultSlug, settings); err != nil {
    return err
}
```

This is what keeps defaults, validation, and help text consistent with your schema.

### Output pattern: emit `types.Row`

```go
row := types.NewRow(
    types.MRP("session_id", sessionID),
    types.MRP("project", project),
    types.MRP("updated_at", updatedAt),
)
return gp.AddRow(ctx, row)
```

### Parameter layers / standard output flags

Use `schema.NewGlazedSchema()` (and optionally `cli.NewCommandSettingsLayer()`) so you get standard output-related flags automatically.

## Usage Examples

Examples (planned CLI names):

```bash
codex-sessions list --output json
codex-sessions search --query "panic" --output table
codex-sessions export --session-id <uuid> --output yaml
```

## Appendix: excerpted notes from the help output

The following excerpt is copied from the captured help output (wrapped for readability):

```text
**Important — Decode values into a struct:** Always decode resolved values
into your settings struct using values.DecodeSectionInto(vals,
schema.DefaultSlug, &YourSettings{}) ...

The schema.NewGlazedSchema() helper ... adds standard flags like --output,
--fields, and --sort-columns ...
```

## Related

- `ttmp/2026/01/23/CODEX-001-PORT-GO--port-codex-reflect-skill-to-go/design-doc/02-design-go-conversation-parsing-indexing-and-glazed-cli.md`
