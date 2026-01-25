package indexdb

import (
	"context"
	"database/sql"
	"sort"
	"time"
)

type SearchScope string

const (
	ScopeMessages SearchScope = "messages"
	ScopeTools    SearchScope = "tools"
	ScopeAll      SearchScope = "all"
)

type SearchOptions struct {
	Query      string
	MaxResults int
	Scope      SearchScope

	Project string
	Since   *time.Time
	Until   *time.Time
}

type SearchHit struct {
	SessionID  string
	Project    string
	StartedAt  string
	UpdatedAt  string
	Title      string
	SourcePath string

	Kind      string // message|tool_call|tool_output
	Timestamp string
	Role      string
	Tool      string
	Snippet   string
	Score     float64
}

func searchMessages(ctx context.Context, db *sql.DB, opts SearchOptions) ([]SearchHit, error) {
	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}

	var since string
	if opts.Since != nil {
		since = opts.Since.UTC().Format(time.RFC3339)
	}
	var until string
	if opts.Until != nil {
		until = opts.Until.UTC().Format(time.RFC3339)
	}

	rows, err := db.QueryContext(ctx, `
SELECT
  s.session_id,
  s.project,
  s.started_at,
  s.updated_at,
  s.title,
  s.source_path,
  m.ts,
  m.role,
  '' AS tool,
  snippet(messages_fts, 0, '', '', ' … ', 10) AS snippet,
  bm25(messages_fts) AS score
FROM messages_fts
JOIN messages m ON m.id = messages_fts.rowid
JOIN sessions s ON s.session_id = messages_fts.session_id
WHERE messages_fts MATCH ?
  AND (? = '' OR s.project = ?)
  AND (? = '' OR s.started_at >= ?)
  AND (? = '' OR s.started_at <= ?)
ORDER BY score
LIMIT ?;`,
		opts.Query,
		opts.Project, opts.Project,
		since, since,
		until, until,
		maxResults,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SearchHit
	for rows.Next() {
		h := SearchHit{Kind: "message"}
		if err := rows.Scan(
			&h.SessionID,
			&h.Project,
			&h.StartedAt,
			&h.UpdatedAt,
			&h.Title,
			&h.SourcePath,
			&h.Timestamp,
			&h.Role,
			&h.Tool,
			&h.Snippet,
			&h.Score,
		); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func searchToolCalls(ctx context.Context, db *sql.DB, opts SearchOptions) ([]SearchHit, error) {
	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}

	var since string
	if opts.Since != nil {
		since = opts.Since.UTC().Format(time.RFC3339)
	}
	var until string
	if opts.Until != nil {
		until = opts.Until.UTC().Format(time.RFC3339)
	}

	rows, err := db.QueryContext(ctx, `
SELECT
  s.session_id,
  s.project,
  s.started_at,
  s.updated_at,
  s.title,
  s.source_path,
  tc.ts,
  '' AS role,
  tc.tool,
  snippet(tool_calls_fts, 0, '', '', ' … ', 10) AS snippet,
  bm25(tool_calls_fts) AS score
FROM tool_calls_fts
JOIN tool_calls tc ON tc.id = tool_calls_fts.rowid
JOIN sessions s ON s.session_id = tool_calls_fts.session_id
WHERE tool_calls_fts MATCH ?
  AND (? = '' OR s.project = ?)
  AND (? = '' OR s.started_at >= ?)
  AND (? = '' OR s.started_at <= ?)
ORDER BY score
LIMIT ?;`,
		opts.Query,
		opts.Project, opts.Project,
		since, since,
		until, until,
		maxResults,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SearchHit
	for rows.Next() {
		h := SearchHit{Kind: "tool_call"}
		if err := rows.Scan(
			&h.SessionID,
			&h.Project,
			&h.StartedAt,
			&h.UpdatedAt,
			&h.Title,
			&h.SourcePath,
			&h.Timestamp,
			&h.Role,
			&h.Tool,
			&h.Snippet,
			&h.Score,
		); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func searchToolOutputs(ctx context.Context, db *sql.DB, opts SearchOptions) ([]SearchHit, error) {
	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}

	var since string
	if opts.Since != nil {
		since = opts.Since.UTC().Format(time.RFC3339)
	}
	var until string
	if opts.Until != nil {
		until = opts.Until.UTC().Format(time.RFC3339)
	}

	rows, err := db.QueryContext(ctx, `
SELECT
  s.session_id,
  s.project,
  s.started_at,
  s.updated_at,
  s.title,
  s.source_path,
  to1.ts,
  '' AS role,
  to1.tool,
  snippet(tool_outputs_fts, 0, '', '', ' … ', 10) AS snippet,
  bm25(tool_outputs_fts) AS score
FROM tool_outputs_fts
JOIN tool_outputs to1 ON to1.id = tool_outputs_fts.rowid
JOIN sessions s ON s.session_id = tool_outputs_fts.session_id
WHERE tool_outputs_fts MATCH ?
  AND (? = '' OR s.project = ?)
  AND (? = '' OR s.started_at >= ?)
  AND (? = '' OR s.started_at <= ?)
ORDER BY score
LIMIT ?;`,
		opts.Query,
		opts.Project, opts.Project,
		since, since,
		until, until,
		maxResults,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SearchHit
	for rows.Next() {
		h := SearchHit{Kind: "tool_output"}
		if err := rows.Scan(
			&h.SessionID,
			&h.Project,
			&h.StartedAt,
			&h.UpdatedAt,
			&h.Title,
			&h.SourcePath,
			&h.Timestamp,
			&h.Role,
			&h.Tool,
			&h.Snippet,
			&h.Score,
		); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func Search(ctx context.Context, db *sql.DB, opts SearchOptions) ([]SearchHit, error) {
	var hits []SearchHit
	switch opts.Scope {
	case ScopeTools:
		calls, err := searchToolCalls(ctx, db, opts)
		if err != nil {
			return nil, err
		}
		outs, err := searchToolOutputs(ctx, db, opts)
		if err != nil {
			return nil, err
		}
		hits = append(hits, calls...)
		hits = append(hits, outs...)
	case ScopeAll:
		msgs, err := searchMessages(ctx, db, opts)
		if err != nil {
			return nil, err
		}
		calls, err := searchToolCalls(ctx, db, opts)
		if err != nil {
			return nil, err
		}
		outs, err := searchToolOutputs(ctx, db, opts)
		if err != nil {
			return nil, err
		}
		hits = append(hits, msgs...)
		hits = append(hits, calls...)
		hits = append(hits, outs...)
	default:
		msgs, err := searchMessages(ctx, db, opts)
		if err != nil {
			return nil, err
		}
		hits = append(hits, msgs...)
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			if hits[i].SessionID == hits[j].SessionID {
				return hits[i].Timestamp < hits[j].Timestamp
			}
			return hits[i].SessionID < hits[j].SessionID
		}
		return hits[i].Score < hits[j].Score
	})
	if opts.MaxResults > 0 && len(hits) > opts.MaxResults {
		hits = hits[:opts.MaxResults]
	}
	return hits, nil
}
