# Tasks

## TODO

- [ ] Add tasks here

- [x] Implement schema migration for metadata fields + session_meta_kv table
- [x] Populate new metadata columns + is_reflection_copy + file signature during indexing
- [x] Add metadata K/V extraction + upsert into session_meta_kv
- [x] Implement SQLite-first list with staleness reindex (opt-out flag)
- [x] Add tool call arg parsing + columns for structured querying
- [x] Add search flags for tool + args (ParameterTypeKeyValue) and wire into indexdb.Search
- [x] Backfill list when index incomplete + verify reindex behavior
- [x] Allow tool-only search (no query/arg)
- [ ] Initialize TUI help system and update glazed tutorial
