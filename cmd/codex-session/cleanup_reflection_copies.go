package main

import (
	"context"
	"path/filepath"
	"time"

	"github.com/go-go-golems/codex-session/internal/sessions"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type CleanupReflectionCopiesSettings struct {
	SessionsRoot string `glazed:"sessions-root"`
	Prefix       string `glazed:"prefix"`
	DryRun       bool   `glazed:"dry-run"`
	Limit        int    `glazed:"limit"`
	Mode         string `glazed:"mode"`

	Project string `glazed:"project"`
	Since   string `glazed:"since"`
	Until   string `glazed:"until"`
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
				"mode",
				fields.TypeChoice,
				fields.WithDefault("delete"),
				fields.WithChoices("delete", "trash"),
				fields.WithHelp("When --dry-run=false: delete files or move them to <sessions-root>/trash/reflection-copies/YYYY/MM/DD"),
			),
			fields.New(
				"limit",
				fields.TypeInteger,
				fields.WithDefault(0),
				fields.WithHelp("Safety limit: stop after matching this many files (0 = no limit)"),
			),
			fields.New(
				"project",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only act on reflection copies matching this derived project label"),
			),
			fields.New(
				"since",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only act on reflection copies on/after this ISO date or datetime"),
			),
			fields.New(
				"until",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only act on reflection copies on/before this ISO date or datetime"),
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

	results, err := sessions.CleanupReflectionCopies(settings.SessionsRoot, settings.Prefix, sessions.CleanupReflectionCopiesOptions{
		DryRun:  settings.DryRun,
		Limit:   settings.Limit,
		Mode:    settings.Mode,
		Project: settings.Project,
		Since:   since,
		Until:   until,
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
			types.MRP("dest_path", r.DestPath),
			types.MRP("size_bytes", r.SizeBytes),
			types.MRP("error", r.Error),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}
