package main

import (
	"context"
	"path/filepath"

	"codex-reflect-skill/internal/sessions"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type CleanupReflectionCopiesSettings struct {
	SessionsRoot string `glazed.parameter:"sessions-root"`
	Prefix       string `glazed.parameter:"prefix"`
	DryRun       bool   `glazed.parameter:"dry-run"`
	Limit        int    `glazed.parameter:"limit"`
}

type CleanupReflectionCopiesCommand struct {
	*cmds.CommandDescription
}

func NewCleanupReflectionCopiesCommand() (*CleanupReflectionCopiesCommand, error) {
	desc := cmds.NewCommandDescription(
		"reflection-copies",
		cmds.WithShort("List and remove leftover self-reflection session copies"),
		cmds.WithLong(`Scan the sessions archive for reflection copies (content-based, via a prefix) and optionally delete them.

This is intended to clean up orphaned copies left behind when "reflect" was interrupted.
By default this command is safe and will not delete anything unless --dry-run=false.
`),
		cmds.WithFlags(
			fields.New(
				"sessions-root",
				fields.TypeString,
				fields.WithDefault(defaultSessionsRoot()),
				fields.WithHelp("Root directory containing Codex session JSONL files"),
			),
			fields.New(
				"prefix",
				fields.TypeString,
				fields.WithDefault(sessions.DefaultSelfReflectionPrefix),
				fields.WithHelp("Prefix used to mark reflection copies"),
			),
			fields.New(
				"dry-run",
				fields.TypeBool,
				fields.WithDefault(true),
				fields.WithHelp("Do not delete; only list files that would be deleted"),
			),
			fields.New(
				"limit",
				fields.TypeInteger,
				fields.WithDefault(0),
				fields.WithHelp("Safety limit: stop after matching this many files (0 = no limit)"),
			),
		),
	)
	return &CleanupReflectionCopiesCommand{CommandDescription: desc}, nil
}

var _ cmds.GlazeCommand = &CleanupReflectionCopiesCommand{}

func (c *CleanupReflectionCopiesCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &CleanupReflectionCopiesSettings{}
	if err := values.DecodeSectionInto(vals, schema.DefaultSlug, settings); err != nil {
		return errors.Wrap(err, "failed to decode settings")
	}

	results, err := sessions.CleanupReflectionCopies(settings.SessionsRoot, settings.Prefix, sessions.CleanupReflectionCopiesOptions{
		DryRun: settings.DryRun,
		Limit:  settings.Limit,
	})
	if err != nil {
		return err
	}

	for _, r := range results {
		row := types.NewRow(
			types.MRP("status", r.Status),
			types.MRP("session_id", r.SessionID),
			types.MRP("project", r.Project),
			types.MRP("path", filepath.Clean(r.Path)),
			types.MRP("error", r.Error),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}
