package indexdb

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/go-go-golems/codex-session/internal/sessions"
)

type ListFilters struct {
	Project                 string
	Since                   *time.Time
	Until                   *time.Time
	IncludeReflectionCopies bool
}

type SessionRow struct {
	SessionID        string
	Project          string
	StartedAt        string
	UpdatedAt        string
	Title            string
	SourcePath       string
	Cwd              string
	IsReflectionCopy bool
	SourceMtime      int64
	SourceSize       int64
}

func ListSessions(ctx context.Context, db *sql.DB, filters ListFilters) ([]SessionRow, error) {
	var since string
	if filters.Since != nil {
		since = filters.Since.UTC().Format(time.RFC3339)
	}
	var until string
	if filters.Until != nil {
		until = filters.Until.UTC().Format(time.RFC3339)
	}
	includeCopies := 0
	if filters.IncludeReflectionCopies {
		includeCopies = 1
	}

	rows, err := db.QueryContext(ctx, `
SELECT
  session_id,
  project,
  started_at,
  updated_at,
  title,
  source_path,
  cwd,
  is_reflection_copy,
  source_mtime,
  source_size
FROM sessions
WHERE (? = '' OR project = ?)
  AND (? = '' OR started_at >= ?)
  AND (? = '' OR started_at <= ?)
  AND (? = 1 OR is_reflection_copy = 0)
ORDER BY started_at ASC;`,
		filters.Project, filters.Project,
		since, since,
		until, until,
		includeCopies,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []SessionRow
	for rows.Next() {
		var row SessionRow
		var isCopy int
		var cwd sql.NullString
		var sourceMtime sql.NullInt64
		var sourceSize sql.NullInt64
		if err := rows.Scan(
			&row.SessionID,
			&row.Project,
			&row.StartedAt,
			&row.UpdatedAt,
			&row.Title,
			&row.SourcePath,
			&cwd,
			&isCopy,
			&sourceMtime,
			&sourceSize,
		); err != nil {
			return nil, err
		}
		if cwd.Valid {
			row.Cwd = cwd.String
		}
		if sourceMtime.Valid {
			row.SourceMtime = sourceMtime.Int64
		}
		if sourceSize.Valid {
			row.SourceSize = sourceSize.Int64
		}
		row.IsReflectionCopy = isCopy == 1
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ParseSessionTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func RowToMeta(row SessionRow) sessions.SessionMeta {
	ts := ParseSessionTime(row.StartedAt)
	return sessions.SessionMeta{
		ID:        row.SessionID,
		Timestamp: ts,
		Cwd:       row.Cwd,
		Path:      row.SourcePath,
	}
}

func IsRowStale(row SessionRow) bool {
	if row.SourcePath == "" {
		return false
	}
	info, err := os.Stat(row.SourcePath)
	if err != nil {
		return false
	}
	if row.SourceMtime == 0 && row.SourceSize == 0 {
		return true
	}
	if row.SourceMtime != info.ModTime().Unix() {
		return true
	}
	if row.SourceSize != info.Size() {
		return true
	}
	return false
}

func FindStaleRows(rows []SessionRow) []SessionRow {
	var out []SessionRow
	for _, row := range rows {
		if IsRowStale(row) {
			out = append(out, row)
		}
	}
	return out
}
