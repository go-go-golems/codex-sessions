package main

import (
	"context"
	"fmt"
	"os"
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

type ProjectsSettings struct {
	SessionsRoot string `glazed.parameter:"sessions-root"`
}

type ProjectsCommand struct {
	*cmds.CommandDescription
}

func defaultSessionsRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.codex/sessions"
	}
	return filepath.Join(home, ".codex", "sessions")
}

func NewProjectsCommand() (*ProjectsCommand, error) {
	desc := cmds.NewCommandDescription(
		"projects",
		cmds.WithShort("List projects found in the Codex sessions archive"),
		cmds.WithLong(`Scan Codex session logs under a sessions root and emit a project count table.

Project is derived from the session meta cwd basename (matching the Python tool).
`),
		cmds.WithFlags(
			fields.New(
				"sessions-root",
				fields.TypeString,
				fields.WithDefault(defaultSessionsRoot()),
				fields.WithHelp("Root directory containing Codex session JSONL files"),
			),
		),
	)

	return &ProjectsCommand{CommandDescription: desc}, nil
}

var _ cmds.GlazeCommand = &ProjectsCommand{}

func (c *ProjectsCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &ProjectsSettings{}
	if err := values.DecodeSectionInto(vals, schema.DefaultSlug, settings); err != nil {
		return errors.Wrap(err, "failed to decode settings")
	}

	root := settings.SessionsRoot
	paths, err := sessions.DiscoverRolloutFiles(root)
	if err != nil {
		return err
	}

	counts := map[string]int{}
	for _, p := range paths {
		meta, err := sessions.ReadSessionMeta(p)
		if err != nil {
			// Best-effort behavior: skip files we can't parse yet.
			continue
		}
		counts[meta.ProjectName()]++
	}

	currentProject := filepath.Base(mustGetwd())
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	now := time.Now().UTC().Format(time.RFC3339)
	for _, name := range names {
		row := types.NewRow(
			types.MRP("project", name),
			types.MRP("count", counts[name]),
			types.MRP("current", name == currentProject),
			types.MRP("generated_at", now),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("getwd: %v", err))
	}
	return wd
}
