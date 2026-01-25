package indexdb

import (
	"context"
	"database/sql"
)

type Stats struct {
	UserVersion   int
	Sessions      int
	Messages      int
	ToolCalls     int
	ToolOutputs   int
	Paths         int
	Errors        int
	LastIndexedAt string
}

func GetStats(ctx context.Context, db *sql.DB) (*Stats, error) {
	out := &Stats{}
	if err := db.QueryRowContext(ctx, "PRAGMA user_version;").Scan(&out.UserVersion); err != nil {
		return nil, err
	}

	type countStmt struct {
		field *int
		sql   string
	}
	for _, cs := range []countStmt{
		{&out.Sessions, "SELECT COUNT(*) FROM sessions"},
		{&out.Messages, "SELECT COUNT(*) FROM messages"},
		{&out.ToolCalls, "SELECT COUNT(*) FROM tool_calls"},
		{&out.ToolOutputs, "SELECT COUNT(*) FROM tool_outputs"},
		{&out.Paths, "SELECT COUNT(*) FROM paths"},
		{&out.Errors, "SELECT COUNT(*) FROM errors"},
	} {
		if err := db.QueryRowContext(ctx, cs.sql).Scan(cs.field); err != nil {
			return nil, err
		}
	}

	_ = db.QueryRowContext(ctx, "SELECT COALESCE(MAX(indexed_at), '') FROM sessions").Scan(&out.LastIndexedAt)
	return out, nil
}
