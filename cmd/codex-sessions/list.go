package main

import (
	"context"
	"path/filepath"
	"sort"
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

type ListSettings struct {
	SessionsRoot      string `glazed.parameter:"sessions-root"`
	Project           string `glazed.parameter:"project"`
	Since             string `glazed.parameter:"since"`
	Until             string `glazed.parameter:"until"`
	Limit             int    `glazed.parameter:"limit"`
	IncludeMostRecent bool   `glazed.parameter:"include-most-recent"`
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
	if err := values.DecodeSectionInto(vals, schema.DefaultSlug, settings); err != nil {
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

	for _, m := range metas {
		updatedAt, err := sessions.ConversationUpdatedAt(m.Path)
		if err != nil {
			continue
		}
		title, err := sessions.ConversationTitle(m.Path, "[SELF-REFLECTION] ", 80)
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
