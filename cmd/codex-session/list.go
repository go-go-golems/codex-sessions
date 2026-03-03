package main

import (
	"context"
	"path/filepath"
	"sort"
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

type ListSettings struct {
	SessionsRoot      string `glazed:"sessions-root"`
	IndexPath         string `glazed:"index-path"`
	Project           string `glazed:"project"`
	Since             string `glazed:"since"`
	Until             string `glazed:"until"`
	Limit             int    `glazed:"limit"`
	IncludeMostRecent bool   `glazed:"include-most-recent"`
	IncludeCopies     bool   `glazed:"include-reflection-copies"`
	NoIndex           bool   `glazed:"no-index"`
	NoReindex         bool   `glazed:"no-reindex"`
}

type ListCommand struct {
	*cmds.CommandDescription
}

func NewListCommand() (*ListCommand, error) {
	desc := cmds.NewCommandDescription(
		"list",
		cmds.WithShort("List sessions from the Codex archive"),
		cmds.WithLong(`List sessions using a lightweight filesystem scan.

This is the Go analogue of the Python tool's selection behavior (project/time filters, skip most recent by default, limit).
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
				fields.WithHelp("Limit to the most recent N sessions after filtering"),
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
				"no-index",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Disable SQLite index usage and force filesystem scan"),
			),
			fields.New(
				"no-reindex",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Disable automatic reindexing when sessions appear stale"),
			),
		),
	)
	return &ListCommand{CommandDescription: desc}, nil
}

var _ cmds.GlazeCommand = &ListCommand{}

func (c *ListCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &ListSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, settings); err != nil {
		return errors.Wrap(err, "failed to decode settings")
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

	if !settings.NoIndex {
		indexPath := settings.IndexPath
		if indexPath == "" {
			indexPath = indexdb.DefaultIndexPath(settings.SessionsRoot)
		}
		db, err := indexdb.Open(filepath.Clean(indexPath))
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		if err := indexdb.EnsureSchema(db); err != nil {
			return err
		}

		rows, err := indexdb.ListSessions(ctx, db, indexdb.ListFilters{
			Project:                 settings.Project,
			Since:                   since,
			Until:                   until,
			IncludeReflectionCopies: settings.IncludeCopies,
		})
		if err != nil {
			return err
		}

		if !settings.NoReindex {
			shouldBackfill := settings.Limit > 0 && len(rows) < settings.Limit
			if shouldBackfill {
				metas, err := collectMetasFromFS(settings, since, until)
				if err != nil {
					return err
				}
				buildOpts := indexdb.DefaultBuildOptions()
				buildOpts.Force = true
				for _, meta := range metas {
					_ = indexdb.BuildSessionIndex(ctx, db, meta, buildOpts)
				}
				rows, err = indexdb.ListSessions(ctx, db, indexdb.ListFilters{
					Project:                 settings.Project,
					Since:                   since,
					Until:                   until,
					IncludeReflectionCopies: settings.IncludeCopies,
				})
				if err != nil {
					return err
				}
			}

			rowsForReindex := filterRows(rows, settings.IncludeMostRecent, settings.Limit)
			stale := indexdb.FindStaleRows(rowsForReindex)
			if len(stale) > 0 {
				buildOpts := indexdb.DefaultBuildOptions()
				buildOpts.Force = true
				for _, row := range stale {
					meta := indexdb.RowToMeta(row)
					_ = indexdb.BuildSessionIndex(ctx, db, meta, buildOpts)
				}
				rows, err = indexdb.ListSessions(ctx, db, indexdb.ListFilters{
					Project:                 settings.Project,
					Since:                   since,
					Until:                   until,
					IncludeReflectionCopies: settings.IncludeCopies,
				})
				if err != nil {
					return err
				}
			}
		}

		rows = filterRows(rows, settings.IncludeMostRecent, settings.Limit)
		for _, row := range rows {
			updated := indexdb.ParseSessionTime(row.UpdatedAt)
			started := indexdb.ParseSessionTime(row.StartedAt)
			rowOut := types.NewRow(
				types.MRP("session_id", row.SessionID),
				types.MRP("project", row.Project),
				types.MRP("conversation_started_at", started.UTC().Format(time.RFC3339)),
				types.MRP("conversation_updated_at", updated.UTC().Format(time.RFC3339)),
				types.MRP("conversation_title", row.Title),
				types.MRP("source_path", filepath.Clean(row.SourcePath)),
			)
			if err := gp.AddRow(ctx, rowOut); err != nil {
				return err
			}
		}
		return nil
	}

	metas, err := collectMetasFromFS(settings, since, until)
	if err != nil {
		return err
	}
	if len(metas) == 0 {
		return nil
	}

	for _, m := range metas {
		updatedAt, err := sessions.ConversationUpdatedAt(m.Path)
		if err != nil {
			continue
		}
		title, err := sessions.ConversationTitle(m.Path, sessions.DefaultSelfReflectionPrefix, 80)
		if err != nil {
			title = "Untitled conversation"
		}
		row := types.NewRow(
			types.MRP("session_id", m.ID),
			types.MRP("project", m.ProjectName()),
			types.MRP("conversation_started_at", m.Timestamp.UTC().Format(time.RFC3339)),
			types.MRP("conversation_updated_at", updatedAt.UTC().Format(time.RFC3339)),
			types.MRP("conversation_title", title),
			types.MRP("source_path", filepath.Clean(m.Path)),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

func collectMetasFromFS(settings *ListSettings, since *time.Time, until *time.Time) ([]sessions.SessionMeta, error) {
	paths, err := sessions.DiscoverRolloutFilesWithOptions(settings.SessionsRoot, sessions.DiscoverOptions{
		IncludeFilenameCopies:   false,
		IncludeReflectionCopies: settings.IncludeCopies,
		ReflectionCopyPrefix:    sessions.DefaultSelfReflectionPrefix,
	})
	if err != nil {
		return nil, err
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

	sort.Slice(metas, func(i, j int) bool { return metas[i].Timestamp.Before(metas[j].Timestamp) })
	if len(metas) == 0 {
		return nil, nil
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
	return metas, nil
}

func filterRows(rows []indexdb.SessionRow, includeMostRecent bool, limit int) []indexdb.SessionRow {
	if len(rows) == 0 {
		return rows
	}

	if !includeMostRecent {
		newest := indexdb.ParseSessionTime(rows[len(rows)-1].StartedAt)
		filtered := rows[:0]
		for _, row := range rows {
			if !indexdb.ParseSessionTime(row.StartedAt).Equal(newest) {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}

	if limit > 0 && len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	return rows
}
