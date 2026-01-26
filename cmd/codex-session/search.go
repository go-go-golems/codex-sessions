package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/codex-session/internal/indexdb"
	"github.com/go-go-golems/codex-session/internal/sessions"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type SearchSettings struct {
	SessionsRoot      string `glazed.parameter:"sessions-root"`
	IndexPath         string `glazed.parameter:"index-path"`
	UseIndex          bool   `glazed.parameter:"use-index"`
	NoReindex         bool   `glazed.parameter:"no-reindex"`
	Scope             string `glazed.parameter:"scope"`
	Query             string `glazed.parameter:"query"`
	Project           string `glazed.parameter:"project"`
	Since             string `glazed.parameter:"since"`
	Until             string `glazed.parameter:"until"`
	Limit             int    `glazed.parameter:"limit"`
	MaxResults        int    `glazed.parameter:"max-results"`
	IncludeMostRecent bool   `glazed.parameter:"include-most-recent"`
	IncludeCopies     bool   `glazed.parameter:"include-reflection-copies"`
	CaseSensitive     bool   `glazed.parameter:"case-sensitive"`
	PerMessage        bool   `glazed.parameter:"per-message"`
	MaxSnippetChars   int    `glazed.parameter:"max-snippet-chars"`
	SingleLine        bool   `glazed.parameter:"single-line"`
}

type SearchCommand struct {
	*cmds.CommandDescription
}

func NewSearchCommand() (*SearchCommand, error) {
	desc := cmds.NewCommandDescription(
		"search",
		cmds.WithShort("Search sessions (index-backed when available)"),
		cmds.WithLong(`Search through session message text and extracted facets.

If a local SQLite/FTS index exists, it is used by default for speed. Otherwise, a streaming scan is used.

This is a non-indexed fallback that scans messages extracted from event_msg/response_item entries.
`),
		cmds.WithFlags(
			fields.New(
				"sessions-root",
				fields.TypeString,
				fields.WithDefault(defaultSessionsRoot()),
				fields.WithHelp("Root directory containing Codex session JSONL files"),
			),
			fields.New(
				"index-path",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Path to SQLite index file (default: <sessions-root>/session_index.sqlite)"),
			),
			fields.New(
				"use-index",
				fields.TypeBool,
				fields.WithDefault(true),
				fields.WithHelp("Use SQLite/FTS index when present (falls back to streaming scan if missing)"),
			),
			fields.New(
				"no-reindex",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Disable automatic reindexing when sessions appear stale"),
			),
			fields.New(
				"scope",
				fields.TypeChoice,
				fields.WithDefault("messages"),
				fields.WithChoices("messages", "tools", "all"),
				fields.WithHelp("Indexed mode only: search scope (messages/tools/all)"),
			),
			fields.New(
				"query",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Query substring to search for (required)"),
			),
			fields.New(
				"project",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only include sessions matching this derived project label"),
			),
			fields.New(
				"since",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only include sessions on/after this ISO date or datetime"),
			),
			fields.New(
				"until",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only include sessions on/before this ISO date or datetime"),
			),
			fields.New(
				"limit",
				fields.TypeInteger,
				fields.WithDefault(10),
				fields.WithHelp("Streaming scan only: limit to the most recent N sessions after filtering"),
			),
			fields.New(
				"max-results",
				fields.TypeInteger,
				fields.WithDefault(50),
				fields.WithHelp("Indexed mode only: maximum number of matches to return"),
			),
			fields.New(
				"include-most-recent",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Include the most recent session (skipped by default)"),
			),
			fields.New(
				"include-reflection-copies",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Include reflection copies (sessions prefixed for self-reflection)"),
			),
			fields.New(
				"case-sensitive",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Use case-sensitive matching"),
			),
			fields.New(
				"per-message",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Emit one row per matching message instead of one row per session"),
			),
			fields.New(
				"max-snippet-chars",
				fields.TypeInteger,
				fields.WithDefault(200),
				fields.WithHelp("Maximum snippet length to include in output"),
			),
			fields.New(
				"single-line",
				fields.TypeBool,
				fields.WithDefault(true),
				fields.WithHelp("Render snippet/text as a single line (newlines become \\\\n)"),
			),
		),
	)
	return &SearchCommand{CommandDescription: desc}, nil
}

var _ cmds.GlazeCommand = &SearchCommand{}

func (c *SearchCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &SearchSettings{}
	if err := values.DecodeSectionInto(vals, schema.DefaultSlug, settings); err != nil {
		return errors.Wrap(err, "failed to decode settings")
	}
	if strings.TrimSpace(settings.Query) == "" {
		return errors.New("--query is required")
	}

	var since *time.Time
	if settings.Since != "" {
		parsed, err := sessions.ParseDateOrDateTime(settings.Since)
		if err != nil {
			return errors.Wrap(err, "invalid --since")
		}
		since = &parsed
	}
	var until *time.Time
	if settings.Until != "" {
		parsed, err := sessions.ParseDateOrDateTime(settings.Until)
		if err != nil {
			return errors.Wrap(err, "invalid --until")
		}
		until = &parsed
	}

	query := settings.Query
	if !settings.CaseSensitive {
		query = strings.ToLower(query)
	}
	render := func(s string) string {
		if settings.SingleLine {
			s = strings.ReplaceAll(s, "\r\n", "\n")
			s = strings.ReplaceAll(s, "\n", `\n`)
		}
		if settings.MaxSnippetChars > 0 && len(s) > settings.MaxSnippetChars {
			s = s[:settings.MaxSnippetChars-1] + "…"
		}
		return s
	}

	indexPath := settings.IndexPath
	if indexPath == "" {
		indexPath = indexdb.DefaultIndexPath(settings.SessionsRoot)
	}
	indexPath = filepath.Clean(indexPath)

	if settings.UseIndex && !settings.CaseSensitive {
		if _, err := os.Stat(indexPath); err == nil {
			db, err := indexdb.Open(indexPath)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			if err := indexdb.EnsureSchema(db); err != nil {
				return err
			}

			if !settings.NoReindex {
				rows, err := indexdb.ListSessions(ctx, db, indexdb.ListFilters{
					Project:                 settings.Project,
					Since:                   since,
					Until:                   until,
					IncludeReflectionCopies: settings.IncludeCopies,
				})
				if err != nil {
					return err
				}
				stale := indexdb.FindStaleRows(rows)
				if len(stale) > 0 {
					buildOpts := indexdb.DefaultBuildOptions()
					buildOpts.Force = true
					for _, row := range stale {
						meta := indexdb.RowToMeta(row)
						_ = indexdb.BuildSessionIndex(ctx, db, meta, buildOpts)
					}
				}
			}

			scope := indexdb.ScopeMessages
			switch settings.Scope {
			case "tools":
				scope = indexdb.ScopeTools
			case "all":
				scope = indexdb.ScopeAll
			}

			hits, err := indexdb.Search(ctx, db, indexdb.SearchOptions{
				Query:      settings.Query,
				MaxResults: settings.MaxResults,
				Scope:      scope,
				Project:    settings.Project,
				Since:      since,
				Until:      until,
			})
			if err != nil {
				return err
			}

			if settings.PerMessage {
				for _, h := range hits {
					row := types.NewRow(
						types.MRP("backend", "index"),
						types.MRP("scope", settings.Scope),
						types.MRP("match_kind", h.Kind),
						types.MRP("session_id", h.SessionID),
						types.MRP("project", h.Project),
						types.MRP("conversation_started_at", h.StartedAt),
						types.MRP("conversation_updated_at", h.UpdatedAt),
						types.MRP("conversation_title", h.Title),
						types.MRP("timestamp", h.Timestamp),
						types.MRP("role", h.Role),
						types.MRP("tool", h.Tool),
						types.MRP("snippet", render(h.Snippet)),
						types.MRP("score", h.Score),
						types.MRP("source_path", filepath.Clean(h.SourcePath)),
					)
					if err := gp.AddRow(ctx, row); err != nil {
						return err
					}
				}
				return nil
			}

			type agg struct {
				first    indexdb.SearchHit
				count    int
				minScore float64
			}
			bySession := map[string]*agg{}
			order := make([]string, 0, len(hits))
			for _, h := range hits {
				a := bySession[h.SessionID]
				if a == nil {
					bySession[h.SessionID] = &agg{first: h, count: 1, minScore: h.Score}
					order = append(order, h.SessionID)
					continue
				}
				a.count++
				if h.Score < a.minScore {
					a.minScore = h.Score
					a.first = h
				}
			}
			for _, sid := range order {
				a := bySession[sid]
				h := a.first
				row := types.NewRow(
					types.MRP("backend", "index"),
					types.MRP("scope", settings.Scope),
					types.MRP("session_id", h.SessionID),
					types.MRP("project", h.Project),
					types.MRP("conversation_started_at", h.StartedAt),
					types.MRP("conversation_updated_at", h.UpdatedAt),
					types.MRP("conversation_title", h.Title),
					types.MRP("match_count", a.count),
					types.MRP("snippet", render(h.Snippet)),
					types.MRP("score_min", a.minScore),
					types.MRP("source_path", filepath.Clean(h.SourcePath)),
				)
				if err := gp.AddRow(ctx, row); err != nil {
					return err
				}
			}
			return nil
		}
	}

	paths, err := sessions.DiscoverRolloutFilesWithOptions(settings.SessionsRoot, sessions.DiscoverOptions{
		IncludeFilenameCopies:   false,
		IncludeReflectionCopies: settings.IncludeCopies,
		ReflectionCopyPrefix:    sessions.DefaultSelfReflectionPrefix,
	})
	if err != nil {
		return err
	}

	metas := make([]sessions.SessionMeta, 0, len(paths))
	for _, p := range paths {
		meta, err := sessions.ReadSessionMeta(p)
		if err != nil {
			continue
		}
		if settings.Project != "" && meta.ProjectName() != settings.Project {
			continue
		}
		if since != nil && meta.Timestamp.Before(*since) {
			continue
		}
		if until != nil && meta.Timestamp.After(*until) {
			continue
		}
		metas = append(metas, meta)
	}

	// Reuse the same selection semantics as list: sort by started_at, skip newest, then apply limit.
	sort.Slice(metas, func(i, j int) bool { return metas[i].Timestamp.Before(metas[j].Timestamp) })
	if len(metas) == 0 {
		return nil
	}
	if !settings.IncludeMostRecent {
		newest := metas[len(metas)-1].Timestamp
		filtered := metas[:0]
		for _, m := range metas {
			if !m.Timestamp.Equal(newest) {
				filtered = append(filtered, m)
			}
		}
		metas = filtered
	}
	if settings.Limit > 0 && len(metas) > settings.Limit {
		metas = metas[len(metas)-settings.Limit:]
	}

	for _, meta := range metas {
		msgs, err := sessions.ExtractMessages(meta.Path)
		if err != nil {
			continue
		}
		matchCount := 0
		var firstSnippet string

		for _, m := range msgs {
			text := m.Text
			hay := text
			if !settings.CaseSensitive {
				hay = strings.ToLower(hay)
			}
			if !strings.Contains(hay, query) {
				continue
			}
			matchCount++
			snippet := render(text)
			if firstSnippet == "" {
				firstSnippet = snippet
			}
			if settings.PerMessage {
				row := types.NewRow(
					types.MRP("session_id", meta.ID),
					types.MRP("project", meta.ProjectName()),
					types.MRP("conversation_started_at", meta.Timestamp.UTC().Format(time.RFC3339)),
					types.MRP("timestamp", m.Timestamp.UTC().Format(time.RFC3339)),
					types.MRP("role", m.Role),
					types.MRP("text", snippet),
					types.MRP("source", m.Source),
				)
				if err := gp.AddRow(ctx, row); err != nil {
					return err
				}
			}
		}

		if !settings.PerMessage && matchCount > 0 {
			updatedAt, _ := sessions.ConversationUpdatedAt(meta.Path)
			title, _ := sessions.ConversationTitle(meta.Path, sessions.DefaultSelfReflectionPrefix, 80)
			row := types.NewRow(
				types.MRP("session_id", meta.ID),
				types.MRP("project", meta.ProjectName()),
				types.MRP("conversation_started_at", meta.Timestamp.UTC().Format(time.RFC3339)),
				types.MRP("conversation_updated_at", updatedAt.UTC().Format(time.RFC3339)),
				types.MRP("conversation_title", title),
				types.MRP("match_count", matchCount),
				types.MRP("snippet", firstSnippet),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
		}
	}

	return nil
}
