package main

import (
	"context"
	"path/filepath"

	"codex-reflect-skill/internal/indexdb"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type IndexStatsSettings struct {
	SessionsRoot string `glazed.parameter:"sessions-root"`
	IndexPath    string `glazed.parameter:"index-path"`
}

type IndexStatsCommand struct {
	*cmds.CommandDescription
}

func NewIndexStatsCommand() (*IndexStatsCommand, error) {
	desc := cmds.NewCommandDescription(
		"stats",
		cmds.WithShort("Show index statistics"),
		cmds.WithLong(`Show basic statistics for the SQLite/FTS index (row counts, last indexed timestamp).`),
		cmds.WithFlags(
			fields.New(
				"sessions-root",
				fields.TypeString,
				fields.WithDefault(defaultSessionsRoot()),
				fields.WithHelp("Root directory containing Codex session JSONL files (used to derive default index path)"),
			),
			fields.New(
				"index-path",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Path to SQLite index file (default: <sessions-root>/session_index.sqlite)"),
			),
		),
	)
	return &IndexStatsCommand{CommandDescription: desc}, nil
}

var _ cmds.GlazeCommand = &IndexStatsCommand{}

func (c *IndexStatsCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &IndexStatsSettings{}
	if err := values.DecodeSectionInto(vals, schema.DefaultSlug, settings); err != nil {
		return errors.Wrap(err, "failed to decode settings")
	}

	indexPath := settings.IndexPath
	if indexPath == "" {
		indexPath = indexdb.DefaultIndexPath(settings.SessionsRoot)
	}
	indexPath = filepath.Clean(indexPath)

	db, err := indexdb.Open(indexPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := indexdb.EnsureSchema(db); err != nil {
		return err
	}

	stats, err := indexdb.GetStats(ctx, db)
	if err != nil {
		return err
	}

	row := types.NewRow(
		types.MRP("index_path", indexPath),
		types.MRP("schema_user_version", stats.UserVersion),
		types.MRP("sessions", stats.Sessions),
		types.MRP("messages", stats.Messages),
		types.MRP("tool_calls", stats.ToolCalls),
		types.MRP("tool_outputs", stats.ToolOutputs),
		types.MRP("paths", stats.Paths),
		types.MRP("errors", stats.Errors),
		types.MRP("last_indexed_at", stats.LastIndexedAt),
	)
	return gp.AddRow(ctx, row)
}
