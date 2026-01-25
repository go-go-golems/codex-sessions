package main

import (
	"context"
	"path/filepath"
	"sort"
	"time"

	"codex-reflect-skill/internal/indexdb"
	"codex-reflect-skill/internal/sessions"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type IndexBuildSettings struct {
	SessionsRoot       string `glazed.parameter:"sessions-root"`
	IndexPath          string `glazed.parameter:"index-path"`
	Project            string `glazed.parameter:"project"`
	Since              string `glazed.parameter:"since"`
	Until              string `glazed.parameter:"until"`
	Limit              int    `glazed.parameter:"limit"`
	IncludeMostRecent  bool   `glazed.parameter:"include-most-recent"`
	Force              bool   `glazed.parameter:"force"`
	MaxChars           int    `glazed.parameter:"max-chars"`
	IncludeToolCalls   bool   `glazed.parameter:"include-tool-calls"`
	IncludeToolOutputs bool   `glazed.parameter:"include-tool-outputs"`
}

type IndexBuildCommand struct {
	*cmds.CommandDescription
}

func NewIndexBuildCommand() (*IndexBuildCommand, error) {
	desc := cmds.NewCommandDescription(
		"build",
		cmds.WithShort("Build or refresh the SQLite/FTS index"),
		cmds.WithLong(`Build a local SQLite/FTS index for fast search across many sessions.

The index is incremental by default (rebuild sessions whose conversation_updated_at increased).
Use --force to rebuild everything selected.
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
				fields.WithDefault(0),
				fields.WithHelp("Limit to the most recent N sessions after filtering (0 = no limit)"),
			),
			fields.New(
				"include-most-recent",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Include the most recent session (skipped by default)"),
			),
			fields.New(
				"force",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Rebuild selected sessions even if they appear up-to-date"),
			),
			fields.New(
				"max-chars",
				fields.TypeInteger,
				fields.WithDefault(20000),
				fields.WithHelp("Truncate indexed text values to at most this many characters (0 = no truncation)"),
			),
			fields.New(
				"include-tool-calls",
				fields.TypeBool,
				fields.WithDefault(true),
				fields.WithHelp("Index tool call arguments (recommended)"),
			),
			fields.New(
				"include-tool-outputs",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Index tool outputs (may contain secrets; increases index size)"),
			),
		),
	)
	return &IndexBuildCommand{CommandDescription: desc}, nil
}

var _ cmds.GlazeCommand = &IndexBuildCommand{}

func (c *IndexBuildCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &IndexBuildSettings{}
	if err := values.DecodeSectionInto(vals, schema.DefaultSlug, settings); err != nil {
		return errors.Wrap(err, "failed to decode settings")
	}

	indexPath := settings.IndexPath
	if indexPath == "" {
		indexPath = indexdb.DefaultIndexPath(settings.SessionsRoot)
	}
	indexPath = filepath.Clean(indexPath)

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

	db, err := indexdb.Open(indexPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := indexdb.EnsureSchema(db); err != nil {
		return err
	}

	paths, err := sessions.DiscoverRolloutFiles(settings.SessionsRoot)
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

	opts := indexdb.BuildOptions{
		Force:              settings.Force,
		MaxChars:           settings.MaxChars,
		IncludeToolCalls:   settings.IncludeToolCalls,
		IncludeToolOutputs: settings.IncludeToolOutputs,
	}

	for _, meta := range metas {
		r := indexdb.BuildSessionIndex(ctx, db, meta, opts)
		row := types.NewRow(
			types.MRP("index_path", indexPath),
			types.MRP("session_id", r.SessionID),
			types.MRP("project", r.Project),
			types.MRP("conversation_started_at", r.StartedAt),
			types.MRP("conversation_updated_at", r.UpdatedAt),
			types.MRP("conversation_title", r.Title),
			types.MRP("source_path", filepath.Clean(r.SourcePath)),
			types.MRP("status", string(r.Status)),
			types.MRP("duration_ms", r.Duration.Milliseconds()),
			types.MRP("error", r.Error),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}
	return nil
}
