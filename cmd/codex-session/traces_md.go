package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/codex-session/internal/sessions"
	"github.com/go-go-golems/codex-session/internal/tracesmd"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type TracesMDSettings struct {
	SessionsRoot      string `glazed:"sessions-root"`
	Project           string `glazed:"project"`
	Since             string `glazed:"since"`
	Until             string `glazed:"until"`
	SessionID         string `glazed:"session-id"`
	SessionIDs        string `glazed:"session-ids"`
	Limit             int    `glazed:"limit"`
	IncludeMostRecent bool   `glazed:"include-most-recent"`

	MDOutput       string `glazed:"md-output"`
	EntriesPerFile int    `glazed:"entries-per-file"`
	MaxStrLen      int    `glazed:"max-str-len"`
	MaxListLen     int    `glazed:"max-list-len"`
	IncludeMeta    bool   `glazed:"include-entry-metadata"`
	PayloadTypes   string `glazed:"payload-types"`
	IncludeRaw     bool   `glazed:"include-raw-payload"`
}

type TracesMDCommand struct {
	*cmds.CommandDescription
}

func NewTracesMDCommand() (*TracesMDCommand, error) {
	desc := cmds.NewCommandDescription(
		"md",
		cmds.WithShort("Export trace examples to Markdown"),
		cmds.WithLong(`Export curated trace examples (response_item excerpts) as a Markdown report.

This is a Go port of scripts/parse_traces.py and is intended for schema debugging and human review.
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
				"session-id",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Specific session id to include (overrides filters/limit)"),
			),
			fields.New(
				"session-ids",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Comma-separated session ids to include (overrides filters/limit)"),
			),
			fields.New(
				"limit",
				fields.TypeInteger,
				fields.WithDefault(3),
				fields.WithHelp("Limit to the most recent N sessions after filtering"),
			),
			fields.New(
				"include-most-recent",
				fields.TypeBool,
				fields.WithDefault(true),
				fields.WithHelp("Include the most recent session (enabled by default for traces)"),
			),
			fields.New(
				"md-output",
				fields.TypeString,
				fields.WithDefault("trace_examples.md"),
				fields.WithHelp("Markdown output path for the generated report (or '-' for stdout)"),
			),
			fields.New(
				"entries-per-file",
				fields.TypeInteger,
				fields.WithDefault(20),
				fields.WithHelp("Number of response_item entries to include per session file"),
			),
			fields.New(
				"include-entry-metadata",
				fields.TypeBool,
				fields.WithDefault(true),
				fields.WithHelp("Include entry metadata (line_no, timestamp, tool_name when present)"),
			),
			fields.New(
				"payload-types",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Comma-separated response_item payload.type values to include (empty = all)"),
			),
			fields.New(
				"include-raw-payload",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Include a truncated rendering of the raw payload object"),
			),
			fields.New(
				"max-str-len",
				fields.TypeInteger,
				fields.WithDefault(2000),
				fields.WithHelp("Truncate strings in extracted payloads to at most this many characters (0 = no truncation)"),
			),
			fields.New(
				"max-list-len",
				fields.TypeInteger,
				fields.WithDefault(10),
				fields.WithHelp("Truncate lists in extracted payloads to at most this many items (0 = no truncation)"),
			),
		),
	)
	return &TracesMDCommand{CommandDescription: desc}, nil
}

var _ cmds.GlazeCommand = &TracesMDCommand{}

func (c *TracesMDCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &TracesMDSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, settings); err != nil {
		return errors.Wrap(err, "failed to decode settings")
	}

	// Resolve selection set
	var metas []sessions.SessionMeta
	explicitIDs := []string{}
	if strings.TrimSpace(settings.SessionID) != "" {
		explicitIDs = append(explicitIDs, strings.TrimSpace(settings.SessionID))
	}
	explicitIDs = append(explicitIDs, parseCSVIDs(settings.SessionIDs)...)

	if len(explicitIDs) > 0 {
		seen := map[string]bool{}
		for _, id := range explicitIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			meta, err := sessions.FindSessionByID(settings.SessionsRoot, id)
			if err != nil {
				return err
			}
			metas = append(metas, meta)
		}
	} else {
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

		paths, err := sessions.DiscoverRolloutFilesWithOptions(settings.SessionsRoot, sessions.DiscoverOptions{
			IncludeFilenameCopies:   false,
			IncludeReflectionCopies: false,
			ReflectionCopyPrefix:    sessions.DefaultSelfReflectionPrefix,
		})
		if err != nil {
			return err
		}
		metas = make([]sessions.SessionMeta, 0, len(paths))
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
	}

	paths := make([]string, 0, len(metas))
	for _, m := range metas {
		paths = append(paths, m.Path)
	}

	mdLines, err := tracesmd.BuildMarkdown(paths, tracesmd.Options{
		EntriesPerFile:       settings.EntriesPerFile,
		MaxStrLen:            settings.MaxStrLen,
		MaxListLen:           settings.MaxListLen,
		IncludeEntryMetadata: settings.IncludeMeta,
		PayloadTypes:         parseCSVIDs(settings.PayloadTypes),
		IncludeRawPayload:    settings.IncludeRaw,
	})
	if err != nil {
		return err
	}

	content := strings.Join(mdLines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if settings.MDOutput == "-" {
		fmt.Print(content)
		return nil
	}

	outPath := filepath.Clean(settings.MDOutput)
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return err
	}

	row := types.NewRow(
		types.MRP("output_path", outPath),
		types.MRP("session_count", len(paths)),
		types.MRP("entries_per_file", settings.EntriesPerFile),
		types.MRP("max_str_len", settings.MaxStrLen),
		types.MRP("max_list_len", settings.MaxListLen),
	)
	return gp.AddRow(ctx, row)
}
