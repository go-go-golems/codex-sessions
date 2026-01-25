package main

import (
	"context"
	"path/filepath"
	"strings"

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
	View         string `glazed.parameter:"view"`
	SingleLine   bool   `glazed.parameter:"single-line"`
	Limit        int    `glazed.parameter:"limit"`
}

type ShowCommand struct {
	*cmds.CommandDescription
}

func NewShowCommand() (*ShowCommand, error) {
	desc := cmds.NewCommandDescription(
		"show",
		cmds.WithShort("Show a session timeline or extracted facets"),
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
				fields.WithHelp("Truncate text values to at most this many characters"),
			),
			fields.New(
				"view",
				fields.TypeChoice,
				fields.WithDefault("timeline"),
				fields.WithChoices("timeline", "tools", "paths", "errors", "texts"),
				fields.WithHelp("View to render: timeline, tools, paths, errors, or texts"),
			),
			fields.New(
				"single-line",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Render text fields as a single line (newlines become \\\\n)"),
			),
			fields.New(
				"limit",
				fields.TypeInteger,
				fields.WithDefault(200),
				fields.WithHelp("Limit rows for non-timeline views (0 = no limit)"),
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

	limit := settings.Limit
	if limit < 0 {
		limit = 0
	}
	renderText := func(s string) string {
		if settings.SingleLine {
			s = strings.ReplaceAll(s, "\r\n", "\n")
			s = strings.ReplaceAll(s, "\n", `\n`)
		}
		if settings.MaxChars > 0 && len(s) > settings.MaxChars {
			s = s[:settings.MaxChars-1] + "…"
		}
		return s
	}

	switch settings.View {
	case "timeline":
		msgs, err := sessions.ExtractMessages(meta.Path)
		if err != nil {
			return err
		}
		for _, m := range msgs {
			row := types.NewRow(
				types.MRP("session_id", meta.ID),
				types.MRP("project", meta.ProjectName()),
				types.MRP("timestamp", m.Timestamp.UTC().Format("2006-01-02T15:04:05Z")),
				types.MRP("role", m.Role),
				types.MRP("text", renderText(m.Text)),
				types.MRP("source", m.Source),
				types.MRP("source_path", filepath.Clean(meta.Path)),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
		}
		return nil
	case "tools", "paths", "errors", "texts":
		facets, err := sessions.ExtractFacets(meta.Path, sessions.FacetOptions{MaxValueChars: settings.MaxChars})
		if err != nil {
			return err
		}
		added := 0
		emit := func(row types.Row) error {
			if limit > 0 && added >= limit {
				return nil
			}
			added++
			return gp.AddRow(ctx, row)
		}

		switch settings.View {
		case "texts":
			for _, t := range facets.Texts {
				row := types.NewRow(
					types.MRP("session_id", meta.ID),
					types.MRP("project", meta.ProjectName()),
					types.MRP("timestamp", t.Timestamp.UTC().Format("2006-01-02T15:04:05Z")),
					types.MRP("source", t.Source),
					types.MRP("text", renderText(t.Text)),
				)
				if err := emit(row); err != nil {
					return err
				}
			}
		case "tools":
			for _, c := range facets.ToolCalls {
				row := types.NewRow(
					types.MRP("session_id", meta.ID),
					types.MRP("project", meta.ProjectName()),
					types.MRP("timestamp", c.Timestamp.UTC().Format("2006-01-02T15:04:05Z")),
					types.MRP("kind", "call"),
					types.MRP("tool", c.Name),
					types.MRP("value", renderText(c.Arguments)),
				)
				if err := emit(row); err != nil {
					return err
				}
			}
			for _, o := range facets.ToolOutputs {
				row := types.NewRow(
					types.MRP("session_id", meta.ID),
					types.MRP("project", meta.ProjectName()),
					types.MRP("timestamp", o.Timestamp.UTC().Format("2006-01-02T15:04:05Z")),
					types.MRP("kind", "output"),
					types.MRP("tool", o.Name),
					types.MRP("value", renderText(o.Output)),
				)
				if err := emit(row); err != nil {
					return err
				}
			}
		case "paths":
			for _, p := range facets.Paths {
				row := types.NewRow(
					types.MRP("session_id", meta.ID),
					types.MRP("project", meta.ProjectName()),
					types.MRP("timestamp", p.Timestamp.UTC().Format("2006-01-02T15:04:05Z")),
					types.MRP("path", renderText(p.Path)),
					types.MRP("source", p.Source),
					types.MRP("role", p.Role),
				)
				if err := emit(row); err != nil {
					return err
				}
			}
		case "errors":
			for _, e := range facets.Errors {
				row := types.NewRow(
					types.MRP("session_id", meta.ID),
					types.MRP("project", meta.ProjectName()),
					types.MRP("timestamp", e.Timestamp.UTC().Format("2006-01-02T15:04:05Z")),
					types.MRP("kind", e.Kind),
					types.MRP("source", e.Source),
					types.MRP("snippet", renderText(e.Snippet)),
				)
				if err := emit(row); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return errors.Errorf("unknown --view %q", settings.View)
	}

	return nil
}
