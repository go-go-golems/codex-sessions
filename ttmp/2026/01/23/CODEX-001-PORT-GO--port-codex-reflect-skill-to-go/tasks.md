# Tasks

## TODO (Detailed)

### Phase 0: Repo + Tooling

- [ ] Decide Go module layout + binary name (suggested: `cmd/codex-sessions`)
- [ ] Add `go.mod` + baseline `main.go`
- [ ] Add minimal build/run docs to repo `README.md` (how to run the Go CLI)
- [ ] Add `.gitignore` updates if the Go build introduces artifacts

### Phase 1: Session JSONL discovery + parsing (streaming, tolerant)

- [ ] Implement session file discovery:
  - [ ] Default root: `~/.codex/sessions`
  - [ ] Match `**/rollout-*.jsonl`, exclude reflection copies and `-copy` artifacts
- [ ] Implement streaming JSONL reader (line-by-line) with:
  - [ ] line number tracking
  - [ ] best-effort extraction of `type` and optional `timestamp`
  - [ ] raw JSON retention for unknown formats
- [ ] Implement `session_meta` decoding supporting:
  - [ ] new format: `{type:"session_meta", payload:{id,timestamp,cwd,...}}`
  - [ ] legacy format: `{id,timestamp,cwd,...}` (no wrapper)

### Phase 2: Normalization (conversation “truth table”)

- [ ] Implement `conversation_updated_at` = max timestamp across JSONL lines
- [ ] Implement `conversation_title` heuristic:
  - [ ] prefer first `event_msg.user_message.message`
  - [ ] fallback to first `response_item` where role=user and first `input_text`
  - [ ] support the IDE marker `## my request for codex:` (extract the next non-empty line)
  - [ ] strip `[SELF-REFLECTION] ` prefix when present
  - [ ] truncate to a stable limit (match Python: 80 chars)
- [ ] Normalize message timeline:
  - [ ] map `event_msg` and `response_item` into a unified `Message{role, ts, text, source}`
  - [ ] keep raw segments for export/debug

### Phase 3: Extraction facets (query building blocks)

- [ ] Extract `text` fields from nested payloads (optional)
- [ ] Extract tool call metadata when present:
  - [ ] tool name
  - [ ] arguments (raw + parsed when JSON)
  - [ ] outputs (with configurable truncation)
- [ ] Extract file/path mentions from:
  - [ ] tool args/outputs
  - [ ] message text
- [ ] Extract errors (best-effort):
  - [ ] non-zero exit codes
  - [ ] stack traces / common error lines

### Phase 4: Optional indexing (SQLite + FTS)

- [ ] Define SQLite schema for sessions/messages/tools/events
- [ ] Implement `index build`:
  - [ ] incremental refresh based on `conversation_updated_at`
  - [ ] transactions per session for performance
- [ ] Implement `search`:
  - [ ] index-backed default when index exists
  - [ ] streaming fallback when no index

### Phase 5: Reflection parity (Codex exec resume) + caching

- [ ] Port prompt selection:
  - [ ] presets (`reflection`, `summary`, `bloat`, `incomplete`, `decisions`, `next_steps`)
  - [ ] `--prompt-file`
  - [ ] `--prompt-text` inline
- [ ] Port prompt version state tracking (per preset/file + inline prompts)
- [ ] Implement cache:
  - [ ] `reflection_cache/<session_id>-<prompt_key>.json`
  - [ ] legacy cache reuse only for default prompt
  - [ ] refresh-mode: `never|auto|always`
- [ ] Implement reflection generation:
  - [ ] duplicate session JSONL with new UUID + sync session_meta id
  - [ ] prefix first user message with `[SELF-REFLECTION] ` (match Python behavior)
  - [ ] run `codex exec ... resume <copy_id> -` (stdin prompt)
  - [ ] parse reflection from last assistant message
  - [ ] delete copy

### Phase 6: Glazed CLI suite

- [ ] Use Glazed patterns (`values.DecodeSectionInto` + `types.Row`)
- [ ] Implement commands:
  - [ ] `projects` (counts per project; mark current project)
  - [ ] `list` (session listing)
  - [ ] `show` (timeline/tools/raw views as rows)
  - [ ] `export` (normalized JSON or rows)
  - [ ] `search` (index-backed or streaming)
  - [ ] `index build` / `index stats`
  - [ ] `reflect` (parity with Python reflect behavior)

### Phase 7: Validation + tests

- [ ] Add parser unit tests with small fixtures (redacted)
- [ ] Validate against a real `~/.codex/sessions` archive (local)
- [ ] Document known edge cases + limitations in the ticket
