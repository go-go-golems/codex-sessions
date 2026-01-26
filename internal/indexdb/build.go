package indexdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-go-golems/codex-session/internal/sessions"
)

type BuildOptions struct {
	Force              bool
	MaxChars           int
	IncludeToolCalls   bool
	IncludeToolOutputs bool
}

const DefaultMaxChars = 20000

func DefaultBuildOptions() BuildOptions {
	return BuildOptions{
		MaxChars:           DefaultMaxChars,
		IncludeToolCalls:   true,
		IncludeToolOutputs: false,
	}
}

type SessionBuildStatus string

const (
	SessionIndexed SessionBuildStatus = "indexed"
	SessionSkipped SessionBuildStatus = "skipped"
	SessionFailed  SessionBuildStatus = "failed"
)

type SessionBuildResult struct {
	SessionID  string
	Project    string
	SourcePath string
	StartedAt  string
	UpdatedAt  string
	Title      string
	Status     SessionBuildStatus
	Error      string
	Duration   time.Duration
}

func truncateForIndex(s string, maxChars int) string {
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	if maxChars <= 1 {
		return "…"
	}
	return s[:maxChars-1] + "…"
}

func shouldReindex(existingUpdatedAt string, newUpdatedAt time.Time) bool {
	if existingUpdatedAt == "" {
		return true
	}
	existing, err := time.Parse(time.RFC3339, existingUpdatedAt)
	if err != nil {
		return true
	}
	return newUpdatedAt.After(existing)
}

func BuildSessionIndex(ctx context.Context, db *sql.DB, meta sessions.SessionMeta, opts BuildOptions) SessionBuildResult {
	start := time.Now()
	res := SessionBuildResult{
		SessionID:  meta.ID,
		Project:    meta.ProjectName(),
		SourcePath: meta.Path,
		StartedAt:  meta.Timestamp.UTC().Format(time.RFC3339),
		Status:     SessionFailed,
	}

	metaPayload, _ := sessions.ReadSessionMetaPayload(meta.Path)
	metaJSON := ""
	metaCwd := meta.Cwd
	metaHost := ""
	metaModel := ""
	metaClient := ""
	metaSessionVersion := ""
	if metaPayload != nil {
		metaJSON = marshalMetaJSON(metaPayload)
		if v := stringFromAny(metaPayload["cwd"]); v != "" {
			metaCwd = v
		}
		metaHost = stringFromAny(metaPayload["host"])
		metaModel = stringFromAny(metaPayload["model"])
		metaClient = stringFromAny(metaPayload["client"])
		metaSessionVersion = stringFromAny(metaPayload["session_version"])
		if metaSessionVersion == "" {
			metaSessionVersion = stringFromAny(metaPayload["version"])
		}
	}

	var sourceMtime int64
	var sourceSize int64
	if info, err := os.Stat(meta.Path); err == nil {
		sourceMtime = info.ModTime().Unix()
		sourceSize = info.Size()
	}
	sourceHash := ""
	isReflectionCopy := 0
	if ok, err := sessions.IsReflectionCopy(meta.Path, sessions.DefaultSelfReflectionPrefix); err == nil && ok {
		isReflectionCopy = 1
	}

	updatedAt, err := sessions.ConversationUpdatedAt(meta.Path)
	if err != nil {
		res.Error = err.Error()
		res.Duration = time.Since(start)
		return res
	}
	res.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	title, err := sessions.ConversationTitle(meta.Path, sessions.DefaultSelfReflectionPrefix, 80)
	if err != nil {
		title = "Untitled conversation"
	}
	res.Title = title

	var existingUpdatedAt string
	_ = db.QueryRowContext(ctx, "SELECT updated_at FROM sessions WHERE session_id = ?", meta.ID).Scan(&existingUpdatedAt)
	if !opts.Force && !shouldReindex(existingUpdatedAt, updatedAt) {
		res.Status = SessionSkipped
		res.Duration = time.Since(start)
		return res
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		res.Error = err.Error()
		res.Duration = time.Since(start)
		return res
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO sessions(
			session_id, project, started_at, updated_at, title, source_path, indexed_at,
			meta_json, cwd, host, model, client, session_version,
			source_mtime, source_size, source_hash, is_reflection_copy
		)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(session_id) DO UPDATE SET
		   project=excluded.project,
		   started_at=excluded.started_at,
		   updated_at=excluded.updated_at,
		   title=excluded.title,
		   source_path=excluded.source_path,
		   indexed_at=excluded.indexed_at,
		   meta_json=excluded.meta_json,
		   cwd=excluded.cwd,
		   host=excluded.host,
		   model=excluded.model,
		   client=excluded.client,
		   session_version=excluded.session_version,
		   source_mtime=excluded.source_mtime,
		   source_size=excluded.source_size,
		   source_hash=excluded.source_hash,
		   is_reflection_copy=excluded.is_reflection_copy`,
		meta.ID,
		res.Project,
		res.StartedAt,
		res.UpdatedAt,
		title,
		meta.Path,
		now,
		metaJSON,
		metaCwd,
		metaHost,
		metaModel,
		metaClient,
		metaSessionVersion,
		sourceMtime,
		sourceSize,
		sourceHash,
		isReflectionCopy,
	)
	if err != nil {
		res.Error = err.Error()
		res.Duration = time.Since(start)
		return res
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM session_meta_kv WHERE session_id = ?", meta.ID); err != nil {
		res.Error = err.Error()
		res.Duration = time.Since(start)
		return res
	}
	if len(metaPayload) > 0 {
		metaKVs := flattenMetaPayload(metaPayload)
		for _, kv := range metaKVs {
			if kv.key == "" || kv.value == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO session_meta_kv(session_id, key, value, value_type) VALUES(?, ?, ?, ?)",
				meta.ID,
				kv.key,
				kv.value,
				kv.valueType,
			); err != nil {
				res.Error = err.Error()
				res.Duration = time.Since(start)
				return res
			}
		}
	}

	// Clear previous rows for this session for idempotent rebuilds.
	clearStmts := []string{
		"DELETE FROM messages WHERE session_id = ?",
		"DELETE FROM messages_fts WHERE session_id = ?",
		"DELETE FROM tool_calls WHERE session_id = ?",
		"DELETE FROM tool_calls_fts WHERE session_id = ?",
		"DELETE FROM tool_outputs WHERE session_id = ?",
		"DELETE FROM tool_outputs_fts WHERE session_id = ?",
		"DELETE FROM paths WHERE session_id = ?",
		"DELETE FROM errors WHERE session_id = ?",
	}
	for _, stmt := range clearStmts {
		if _, err := tx.ExecContext(ctx, stmt, meta.ID); err != nil {
			res.Error = err.Error()
			res.Duration = time.Since(start)
			return res
		}
	}

	// Messages + FTS
	msgs, err := sessions.ExtractMessages(meta.Path)
	if err != nil {
		res.Error = err.Error()
		res.Duration = time.Since(start)
		return res
	}
	for _, m := range msgs {
		text := truncateForIndex(m.Text, opts.MaxChars)
		r, err := tx.ExecContext(ctx,
			"INSERT INTO messages(session_id, ts, role, text, source) VALUES(?, ?, ?, ?, ?)",
			meta.ID,
			m.Timestamp.UTC().Format(time.RFC3339),
			m.Role,
			text,
			m.Source,
		)
		if err != nil {
			res.Error = err.Error()
			res.Duration = time.Since(start)
			return res
		}
		messageID, err := r.LastInsertId()
		if err != nil {
			res.Error = err.Error()
			res.Duration = time.Since(start)
			return res
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO messages_fts(rowid, text, session_id, message_id, ts, role) VALUES(?, ?, ?, ?, ?, ?)",
			messageID,
			text,
			meta.ID,
			messageID,
			m.Timestamp.UTC().Format(time.RFC3339),
			m.Role,
		); err != nil {
			res.Error = err.Error()
			res.Duration = time.Since(start)
			return res
		}
	}

	// Facets (tools/paths/errors) + FTS where applicable.
	f, err := sessions.ExtractFacets(meta.Path, sessions.FacetOptions{MaxValueChars: opts.MaxChars})
	if err != nil {
		res.Error = err.Error()
		res.Duration = time.Since(start)
		return res
	}

	for _, p := range f.Paths {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO paths(session_id, ts, path, source, role) VALUES(?, ?, ?, ?, ?)",
			meta.ID,
			p.Timestamp.UTC().Format(time.RFC3339),
			truncateForIndex(p.Path, opts.MaxChars),
			p.Source,
			p.Role,
		); err != nil {
			res.Error = err.Error()
			res.Duration = time.Since(start)
			return res
		}
	}

	for _, e := range f.Errors {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO errors(session_id, ts, kind, source, snippet) VALUES(?, ?, ?, ?, ?)",
			meta.ID,
			e.Timestamp.UTC().Format(time.RFC3339),
			e.Kind,
			e.Source,
			truncateForIndex(e.Snippet, opts.MaxChars),
		); err != nil {
			res.Error = err.Error()
			res.Duration = time.Since(start)
			return res
		}
	}

	if opts.IncludeToolCalls {
		for _, c := range f.ToolCalls {
			args := truncateForIndex(c.Arguments, opts.MaxChars)
			r, err := tx.ExecContext(ctx,
				"INSERT INTO tool_calls(session_id, ts, tool, arguments) VALUES(?, ?, ?, ?)",
				meta.ID,
				c.Timestamp.UTC().Format(time.RFC3339),
				c.Name,
				args,
			)
			if err != nil {
				res.Error = err.Error()
				res.Duration = time.Since(start)
				return res
			}
			toolCallID, err := r.LastInsertId()
			if err != nil {
				res.Error = err.Error()
				res.Duration = time.Since(start)
				return res
			}
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO tool_calls_fts(rowid, arguments, session_id, tool_call_id, ts, tool) VALUES(?, ?, ?, ?, ?, ?)",
				toolCallID,
				args,
				meta.ID,
				toolCallID,
				c.Timestamp.UTC().Format(time.RFC3339),
				c.Name,
			); err != nil {
				res.Error = err.Error()
				res.Duration = time.Since(start)
				return res
			}
		}
	}

	if opts.IncludeToolOutputs {
		for _, o := range f.ToolOutputs {
			out := truncateForIndex(o.Output, opts.MaxChars)
			r, err := tx.ExecContext(ctx,
				"INSERT INTO tool_outputs(session_id, ts, tool, output) VALUES(?, ?, ?, ?)",
				meta.ID,
				o.Timestamp.UTC().Format(time.RFC3339),
				o.Name,
				out,
			)
			if err != nil {
				res.Error = err.Error()
				res.Duration = time.Since(start)
				return res
			}
			toolOutputID, err := r.LastInsertId()
			if err != nil {
				res.Error = err.Error()
				res.Duration = time.Since(start)
				return res
			}
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO tool_outputs_fts(rowid, output, session_id, tool_output_id, ts, tool) VALUES(?, ?, ?, ?, ?, ?)",
				toolOutputID,
				out,
				meta.ID,
				toolOutputID,
				o.Timestamp.UTC().Format(time.RFC3339),
				o.Name,
			); err != nil {
				res.Error = err.Error()
				res.Duration = time.Since(start)
				return res
			}
		}
	}

	if err := tx.Commit(); err != nil {
		res.Error = err.Error()
		res.Duration = time.Since(start)
		return res
	}

	res.Status = SessionIndexed
	res.Duration = time.Since(start)
	return res
}

func marshalMetaJSON(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(b)
}

func stringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case float64:
		return fmt.Sprintf("%v", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case map[string]any:
		for _, key := range []string{"name", "id", "version"} {
			if s, ok := t[key].(string); ok && s != "" {
				return s
			}
		}
		b, err := json.Marshal(t)
		if err == nil {
			return string(b)
		}
	case []any:
		b, err := json.Marshal(t)
		if err == nil {
			return string(b)
		}
	}
	return ""
}

type metaKV struct {
	key       string
	value     string
	valueType string
}

func flattenMetaPayload(payload map[string]any) []metaKV {
	var out []metaKV
	for k, v := range payload {
		flattenMetaValue(k, v, &out)
	}
	return out
}

func flattenMetaValue(prefix string, v any, out *[]metaKV) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			childKey := prefix + "." + k
			if prefix == "" {
				childKey = k
			}
			flattenMetaValue(childKey, child, out)
		}
	case []any:
		b, err := json.Marshal(t)
		if err != nil {
			return
		}
		*out = append(*out, metaKV{key: prefix, value: string(b), valueType: "json"})
	case string:
		*out = append(*out, metaKV{key: prefix, value: t, valueType: "string"})
	case float64:
		*out = append(*out, metaKV{key: prefix, value: fmt.Sprintf("%v", t), valueType: "number"})
	case bool:
		if t {
			*out = append(*out, metaKV{key: prefix, value: "true", valueType: "bool"})
		} else {
			*out = append(*out, metaKV{key: prefix, value: "false", valueType: "bool"})
		}
	default:
		if t == nil {
			return
		}
		b, err := json.Marshal(t)
		if err != nil {
			return
		}
		*out = append(*out, metaKV{key: prefix, value: string(b), valueType: "json"})
	}
}
