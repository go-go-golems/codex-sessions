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

type ShowSettings struct {
	SessionsRoot string `glazed.parameter:"sessions-root"`
	SessionID    string `glazed.parameter:"session-id"`
	Path         string `glazed.parameter:"path"`
	MaxChars     int    `glazed.parameter:"max-chars"`
}

type ShowCommand struct {
	*cmds.CommandDescription
}

func NewShowCommand() (*ShowCommand, error) {
	desc := cmds.NewCommandDescription(
		"show",
		cmds.WithShort("Show a session message timeline"),
		cmds.WithLong(`Show a normalized message timeline for a session.

Use either --session-id (look up under --sessions-root) or --path (direct JSONL path).
`),
		cmds.WithFlags(
			fields.New(
				"sessions-root",
				fields.TypeString,
				fields.WithDefault(defaultSessionsRoot()),
				fields.WithHelp("Root directory containing Codex session JSONL files (used with --session-id)"),
			),
			fields.New(
				"session-id",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Session id to load (mutually exclusive with --path)"),
			),
			fields.New(
				"path",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Path to a rollout JSONL file (mutually exclusive with --session-id)"),
			),
			fields.New(
				"max-chars",
				fields.TypeInteger,
				fields.WithDefault(2000),
				fields.WithHelp("Truncate message text to at most this many characters"),
			),
		),
	)
	return &ShowCommand{CommandDescription: desc}, nil
}

var _ cmds.GlazeCommand = &ShowCommand{}

func (c *ShowCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &ShowSettings{}
	if err := values.DecodeSectionInto(vals, schema.DefaultSlug, settings); err != nil {
		return errors.Wrap(err, "failed to decode settings")
	}

	if settings.SessionID != "" && settings.Path != "" {
		return errors.New("use only one of --session-id or --path")
	}
	if settings.SessionID == "" && settings.Path == "" {
		return errors.New("must set --session-id or --path")
	}

	var meta sessions.SessionMeta
	var err error
	if settings.Path != "" {
		meta, err = sessions.ReadSessionMeta(settings.Path)
		if err != nil {
			return err
		}
	} else {
		meta, err = sessions.FindSessionByID(settings.SessionsRoot, settings.SessionID)
		if err != nil {
			return err
		}
	}

	msgs, err := sessions.ExtractMessages(meta.Path)
	if err != nil {
		return err
	}

	for _, m := range msgs {
		text := m.Text
		if settings.MaxChars > 0 && len(text) > settings.MaxChars {
			text = text[:settings.MaxChars-1] + "…"
		}
		row := types.NewRow(
			types.MRP("session_id", meta.ID),
			types.MRP("project", meta.ProjectName()),
			types.MRP("timestamp", m.Timestamp.UTC().Format("2006-01-02T15:04:05Z")),
			types.MRP("role", m.Role),
			types.MRP("text", text),
			types.MRP("source", m.Source),
			types.MRP("source_path", filepath.Clean(meta.Path)),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}
