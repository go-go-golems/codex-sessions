package main

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"codex-reflect-skill/internal/sessions"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type ExportSettings struct {
	SessionsRoot string `glazed.parameter:"sessions-root"`
	SessionID    string `glazed.parameter:"session-id"`
	Path         string `glazed.parameter:"path"`

	Shape      string `glazed.parameter:"shape"`   // document|rows
	Extract    string `glazed.parameter:"extract"` // minimal|timeline|facets|all
	MaxChars   int    `glazed.parameter:"max-chars"`
	SingleLine bool   `glazed.parameter:"single-line"`
}

type ExportCommand struct {
	*cmds.CommandDescription
}

func NewExportCommand() (*ExportCommand, error) {
	desc := cmds.NewCommandDescription(
		"export",
		cmds.WithShort("Export a session in a normalized shape"),
		cmds.WithLong(`Export a session for downstream processing.

Two shapes are supported:
- document: emit a single row with nested arrays/maps (best with --output json/yaml)
- rows: emit one row per entity (messages, tool calls, paths, errors)

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
				"shape",
				fields.TypeChoice,
				fields.WithDefault("document"),
				fields.WithChoices("document", "rows"),
				fields.WithHelp("Output shape: document (one row) or rows (one row per entity)"),
			),
			fields.New(
				"extract",
				fields.TypeChoice,
				fields.WithDefault("all"),
				fields.WithChoices("minimal", "timeline", "facets", "all"),
				fields.WithHelp("What to include: minimal metadata, timeline messages, facets, or all"),
			),
			fields.New(
				"max-chars",
				fields.TypeInteger,
				fields.WithDefault(5000),
				fields.WithHelp("Truncate large text values to at most this many characters (0 = no truncation)"),
			),
			fields.New(
				"single-line",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Replace newlines in exported string fields with literal \\\\n"),
			),
		),
	)
	return &ExportCommand{CommandDescription: desc}, nil
}

var _ cmds.GlazeCommand = &ExportCommand{}

func (c *ExportCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &ExportSettings{}
	if err := values.DecodeSectionInto(vals, schema.DefaultSlug, settings); err != nil {
		return errors.Wrap(err, "failed to decode settings")
	}

	if settings.SessionID != "" && settings.Path != "" {
		return errors.New("use only one of --session-id or --path")
	}
	if settings.SessionID == "" && settings.Path == "" {
		return errors.New("must set --session-id or --path")
	}

	render := func(s string) string {
		if settings.SingleLine {
			s = strings.ReplaceAll(s, "\r\n", "\n")
			s = strings.ReplaceAll(s, "\n", `\n`)
		}
		if settings.MaxChars > 0 && len(s) > settings.MaxChars {
			s = s[:settings.MaxChars-1] + "…"
		}
		return s
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

	updatedAt, err := sessions.ConversationUpdatedAt(meta.Path)
	if err != nil {
		return err
	}
	title, err := sessions.ConversationTitle(meta.Path, sessions.DefaultSelfReflectionPrefix, 80)
	if err != nil {
		title = "Untitled conversation"
	}

	includeTimeline := settings.Extract == "timeline" || settings.Extract == "all"
	includeFacets := settings.Extract == "facets" || settings.Extract == "all"

	switch settings.Shape {
	case "document":
		doc := map[string]any{
			"session_id":               meta.ID,
			"project":                  meta.ProjectName(),
			"conversation_started_at":  meta.Timestamp.UTC().Format(time.RFC3339),
			"conversation_updated_at":  updatedAt.UTC().Format(time.RFC3339),
			"conversation_title":       title,
			"source_path":              filepath.Clean(meta.Path),
			"exported_at":              time.Now().UTC().Format(time.RFC3339),
			"export_shape":             "document",
			"export_extract_selection": settings.Extract,
		}

		if includeTimeline {
			msgs, err := sessions.ExtractMessages(meta.Path)
			if err != nil {
				return err
			}
			mOut := make([]map[string]any, 0, len(msgs))
			for _, m := range msgs {
				mOut = append(mOut, map[string]any{
					"timestamp": m.Timestamp.UTC().Format(time.RFC3339),
					"role":      m.Role,
					"text":      render(m.Text),
					"source":    m.Source,
				})
			}
			doc["messages"] = mOut
		}

		if includeFacets {
			f, err := sessions.ExtractFacets(meta.Path, sessions.FacetOptions{MaxValueChars: settings.MaxChars})
			if err != nil {
				return err
			}
			fm := map[string]any{}
			tc := make([]map[string]any, 0, len(f.ToolCalls))
			for _, c := range f.ToolCalls {
				tc = append(tc, map[string]any{
					"timestamp":  c.Timestamp.UTC().Format(time.RFC3339),
					"tool":       c.Name,
					"arguments":  render(c.Arguments),
					"facet_kind": "call",
				})
			}
			to := make([]map[string]any, 0, len(f.ToolOutputs))
			for _, o := range f.ToolOutputs {
				to = append(to, map[string]any{
					"timestamp":  o.Timestamp.UTC().Format(time.RFC3339),
					"tool":       o.Name,
					"output":     render(o.Output),
					"facet_kind": "output",
				})
			}
			paths := make([]map[string]any, 0, len(f.Paths))
			for _, p := range f.Paths {
				paths = append(paths, map[string]any{
					"timestamp": p.Timestamp.UTC().Format(time.RFC3339),
					"path":      render(p.Path),
					"source":    p.Source,
					"role":      p.Role,
				})
			}
			errs := make([]map[string]any, 0, len(f.Errors))
			for _, e := range f.Errors {
				errs = append(errs, map[string]any{
					"timestamp": e.Timestamp.UTC().Format(time.RFC3339),
					"kind":      e.Kind,
					"source":    e.Source,
					"snippet":   render(e.Snippet),
				})
			}
			texts := make([]map[string]any, 0, len(f.Texts))
			for _, t := range f.Texts {
				texts = append(texts, map[string]any{
					"timestamp": t.Timestamp.UTC().Format(time.RFC3339),
					"source":    t.Source,
					"text":      render(t.Text),
				})
			}
			fm["tool_calls"] = tc
			fm["tool_outputs"] = to
			fm["paths"] = paths
			fm["errors"] = errs
			fm["texts"] = texts
			doc["facets"] = fm
		}

		row := types.NewRow(types.MRP("document", doc))
		return gp.AddRow(ctx, row)

	case "rows":
		// Minimal metadata row always
		row := types.NewRow(
			types.MRP("kind", "session"),
			types.MRP("session_id", meta.ID),
			types.MRP("project", meta.ProjectName()),
			types.MRP("conversation_started_at", meta.Timestamp.UTC().Format(time.RFC3339)),
			types.MRP("conversation_updated_at", updatedAt.UTC().Format(time.RFC3339)),
			types.MRP("conversation_title", title),
			types.MRP("source_path", filepath.Clean(meta.Path)),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}

		if includeTimeline {
			msgs, err := sessions.ExtractMessages(meta.Path)
			if err != nil {
				return err
			}
			for _, m := range msgs {
				r := types.NewRow(
					types.MRP("kind", "message"),
					types.MRP("session_id", meta.ID),
					types.MRP("timestamp", m.Timestamp.UTC().Format(time.RFC3339)),
					types.MRP("role", m.Role),
					types.MRP("text", render(m.Text)),
					types.MRP("source", m.Source),
				)
				if err := gp.AddRow(ctx, r); err != nil {
					return err
				}
			}
		}

		if includeFacets {
			f, err := sessions.ExtractFacets(meta.Path, sessions.FacetOptions{MaxValueChars: settings.MaxChars})
			if err != nil {
				return err
			}
			for _, t := range f.Texts {
				r := types.NewRow(
					types.MRP("kind", "text"),
					types.MRP("session_id", meta.ID),
					types.MRP("timestamp", t.Timestamp.UTC().Format(time.RFC3339)),
					types.MRP("source", t.Source),
					types.MRP("text", render(t.Text)),
				)
				if err := gp.AddRow(ctx, r); err != nil {
					return err
				}
			}
			for _, c := range f.ToolCalls {
				r := types.NewRow(
					types.MRP("kind", "tool_call"),
					types.MRP("session_id", meta.ID),
					types.MRP("timestamp", c.Timestamp.UTC().Format(time.RFC3339)),
					types.MRP("tool", c.Name),
					types.MRP("arguments", render(c.Arguments)),
				)
				if err := gp.AddRow(ctx, r); err != nil {
					return err
				}
			}
			for _, o := range f.ToolOutputs {
				r := types.NewRow(
					types.MRP("kind", "tool_output"),
					types.MRP("session_id", meta.ID),
					types.MRP("timestamp", o.Timestamp.UTC().Format(time.RFC3339)),
					types.MRP("tool", o.Name),
					types.MRP("output", render(o.Output)),
				)
				if err := gp.AddRow(ctx, r); err != nil {
					return err
				}
			}
			for _, p := range f.Paths {
				r := types.NewRow(
					types.MRP("kind", "path"),
					types.MRP("session_id", meta.ID),
					types.MRP("timestamp", p.Timestamp.UTC().Format(time.RFC3339)),
					types.MRP("path", render(p.Path)),
					types.MRP("source", p.Source),
					types.MRP("role", p.Role),
				)
				if err := gp.AddRow(ctx, r); err != nil {
					return err
				}
			}
			for _, e := range f.Errors {
				r := types.NewRow(
					types.MRP("kind", "error"),
					types.MRP("session_id", meta.ID),
					types.MRP("timestamp", e.Timestamp.UTC().Format(time.RFC3339)),
					types.MRP("error_kind", e.Kind),
					types.MRP("source", e.Source),
					types.MRP("snippet", render(e.Snippet)),
				)
				if err := gp.AddRow(ctx, r); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return errors.Errorf("unknown --shape %q", settings.Shape)
	}
}
