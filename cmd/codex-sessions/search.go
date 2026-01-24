package main

import (
	"context"
	"sort"
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

type SearchSettings struct {
	SessionsRoot      string `glazed.parameter:"sessions-root"`
	Query             string `glazed.parameter:"query"`
	Project           string `glazed.parameter:"project"`
	Since             string `glazed.parameter:"since"`
	Until             string `glazed.parameter:"until"`
	Limit             int    `glazed.parameter:"limit"`
	IncludeMostRecent bool   `glazed.parameter:"include-most-recent"`
	CaseSensitive     bool   `glazed.parameter:"case-sensitive"`
	PerMessage        bool   `glazed.parameter:"per-message"`
	MaxSnippetChars   int    `glazed.parameter:"max-snippet-chars"`
}

type SearchCommand struct {
	*cmds.CommandDescription
}

func NewSearchCommand() (*SearchCommand, error) {
	desc := cmds.NewCommandDescription(
		"search",
		cmds.WithShort("Search session messages (streaming scan)"),
		cmds.WithLong(`Search through session message text by streaming session logs.

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
				fields.WithHelp("Limit to the most recent N sessions after filtering"),
			),
			fields.New(
				"include-most-recent",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Include the most recent session (skipped by default)"),
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
			snippet := text
			if settings.MaxSnippetChars > 0 && len(snippet) > settings.MaxSnippetChars {
				snippet = snippet[:settings.MaxSnippetChars-1] + "…"
			}
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
			title, _ := sessions.ConversationTitle(meta.Path, "[SELF-REFLECTION] ", 80)
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
