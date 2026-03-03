package indexdb

import "database/sql"

const schemaVersion = 5

func EnsureSchema(db *sql.DB) error {
	// user_version is a simple integer we can bump if we introduce breaking schema changes.
	var userVersion int
	if err := db.QueryRow("PRAGMA user_version;").Scan(&userVersion); err != nil {
		return err
	}
	if userVersion != 0 && userVersion != schemaVersion {
		if err := resetSchema(db); err != nil {
			return err
		}
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			session_id  TEXT PRIMARY KEY,
			project     TEXT,
			started_at  TEXT,
			updated_at  TEXT,
			title       TEXT,
			source_path TEXT NOT NULL,
			indexed_at  TEXT,
			meta_json   TEXT,
			cwd         TEXT,
			host        TEXT,
			model       TEXT,
			client      TEXT,
			session_version TEXT,
			source_mtime INTEGER,
			source_size  INTEGER,
			source_hash  TEXT,
			is_reflection_copy INTEGER DEFAULT 0
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_source_path ON sessions(source_path);`,

		`CREATE TABLE IF NOT EXISTS messages (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			ts         TEXT,
			role       TEXT,
			text       TEXT,
			source     TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id);`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
			text,
			session_id UNINDEXED,
			message_id UNINDEXED,
			ts UNINDEXED,
			role UNINDEXED,
			tokenize='unicode61'
		);`,

		`CREATE TABLE IF NOT EXISTS tool_calls (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			ts         TEXT,
			tool       TEXT,
			call_id    TEXT,
			arguments  TEXT,
			arguments_json TEXT,
			arguments_flat TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_tool_calls_session_id ON tool_calls(session_id);`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS tool_calls_fts USING fts5(
			arguments,
			session_id UNINDEXED,
			tool_call_id UNINDEXED,
			ts UNINDEXED,
			tool UNINDEXED,
			tokenize='unicode61'
		);`,

		`CREATE TABLE IF NOT EXISTS tool_outputs (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			ts         TEXT,
			tool       TEXT,
			call_id    TEXT,
			output     TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_tool_outputs_session_id ON tool_outputs(session_id);`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS tool_outputs_fts USING fts5(
			output,
			session_id UNINDEXED,
			tool_output_id UNINDEXED,
			ts UNINDEXED,
			tool UNINDEXED,
			tokenize='unicode61'
		);`,

		`CREATE TABLE IF NOT EXISTS paths (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			ts         TEXT,
			path       TEXT,
			source     TEXT,
			role       TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_paths_session_id ON paths(session_id);`,

		`CREATE TABLE IF NOT EXISTS errors (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			ts         TEXT,
			kind       TEXT,
			source     TEXT,
			snippet    TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_errors_session_id ON errors(session_id);`,
		`CREATE TABLE IF NOT EXISTS session_meta_kv (
			session_id TEXT NOT NULL,
			key        TEXT NOT NULL,
			value      TEXT NOT NULL,
			value_type TEXT,
			PRIMARY KEY (session_id, key)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_session_meta_kv_key_value ON session_meta_kv(key, value);`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	if _, err := db.Exec("PRAGMA user_version = 5;"); err != nil {
		return err
	}
	return nil
}

func resetSchema(db *sql.DB) error {
	stmts := []string{
		"DROP TABLE IF EXISTS tool_outputs_fts;",
		"DROP TABLE IF EXISTS tool_calls_fts;",
		"DROP TABLE IF EXISTS messages_fts;",
		"DROP TABLE IF EXISTS tool_outputs;",
		"DROP TABLE IF EXISTS tool_calls;",
		"DROP TABLE IF EXISTS messages;",
		"DROP TABLE IF EXISTS paths;",
		"DROP TABLE IF EXISTS errors;",
		"DROP TABLE IF EXISTS sessions;",
		"DROP TABLE IF EXISTS session_meta_kv;",
		"PRAGMA user_version = 0;",
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
