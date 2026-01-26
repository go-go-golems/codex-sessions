package indexdb

import "database/sql"

const schemaVersion = 2

func EnsureSchema(db *sql.DB) error {
	// user_version is a simple integer we can bump if we introduce breaking schema changes.
	var userVersion int
	if err := db.QueryRow("PRAGMA user_version;").Scan(&userVersion); err != nil {
		return err
	}
	if userVersion > schemaVersion {
		// Newer schema than this binary understands.
		return nil
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			session_id  TEXT PRIMARY KEY,
			project     TEXT,
			started_at  TEXT,
			updated_at  TEXT,
			title       TEXT,
			source_path TEXT NOT NULL,
			indexed_at  TEXT
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
			arguments  TEXT
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

	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	if userVersion < 2 {
		if err := migrateToV2(db); err != nil {
			return err
		}
		if _, err := db.Exec("PRAGMA user_version = 2;"); err != nil {
			return err
		}
	}
	return nil
}

func migrateToV2(db *sql.DB) error {
	columns, err := existingColumns(db, "sessions")
	if err != nil {
		return err
	}

	adds := []struct {
		name string
		spec string
	}{
		{name: "meta_json", spec: "TEXT"},
		{name: "cwd", spec: "TEXT"},
		{name: "host", spec: "TEXT"},
		{name: "model", spec: "TEXT"},
		{name: "client", spec: "TEXT"},
		{name: "session_version", spec: "TEXT"},
		{name: "source_mtime", spec: "INTEGER"},
		{name: "source_size", spec: "INTEGER"},
		{name: "source_hash", spec: "TEXT"},
		{name: "is_reflection_copy", spec: "INTEGER DEFAULT 0"},
	}

	for _, add := range adds {
		if columns[add.name] {
			continue
		}
		if _, err := db.Exec("ALTER TABLE sessions ADD COLUMN " + add.name + " " + add.spec + ";"); err != nil {
			return err
		}
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS session_meta_kv (
		session_id TEXT NOT NULL,
		key        TEXT NOT NULL,
		value      TEXT NOT NULL,
		value_type TEXT,
		PRIMARY KEY (session_id, key)
	);`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_session_meta_kv_key_value ON session_meta_kv(key, value);`); err != nil {
		return err
	}

	return nil
}

func existingColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ");")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	columns := map[string]bool{}
	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			defaultVal any
			pk         int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &pk); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}
