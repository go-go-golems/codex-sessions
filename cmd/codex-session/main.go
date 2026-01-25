package main

import (
	"fmt"
	"os"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "codex-session",
		Short: "Query and reflect on Codex session histories",
	}

	projectsCmd, err := NewProjectsCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating projects command: %v\n", err)
		os.Exit(1)
	}
	cobraProjectsCmd, err := cli.BuildCobraCommand(projectsCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			ShortHelpLayers: []string{schema.DefaultSlug},
			MiddlewaresFunc: cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building cobra command: %v\n", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(cobraProjectsCmd)

	listCmd, err := NewListCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating list command: %v\n", err)
		os.Exit(1)
	}
	cobraListCmd, err := cli.BuildCobraCommand(listCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			ShortHelpLayers: []string{schema.DefaultSlug},
			MiddlewaresFunc: cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building cobra command: %v\n", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(cobraListCmd)

	showCmd, err := NewShowCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating show command: %v\n", err)
		os.Exit(1)
	}
	cobraShowCmd, err := cli.BuildCobraCommand(showCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			ShortHelpLayers: []string{schema.DefaultSlug},
			MiddlewaresFunc: cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building cobra command: %v\n", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(cobraShowCmd)

	searchCmd, err := NewSearchCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating search command: %v\n", err)
		os.Exit(1)
	}
	cobraSearchCmd, err := cli.BuildCobraCommand(searchCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			ShortHelpLayers: []string{schema.DefaultSlug},
			MiddlewaresFunc: cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building cobra command: %v\n", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(cobraSearchCmd)

	exportCmd, err := NewExportCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating export command: %v\n", err)
		os.Exit(1)
	}
	cobraExportCmd, err := cli.BuildCobraCommand(exportCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			ShortHelpLayers: []string{schema.DefaultSlug},
			MiddlewaresFunc: cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building cobra command: %v\n", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(cobraExportCmd)

	reflectCmd, err := NewReflectCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating reflect command: %v\n", err)
		os.Exit(1)
	}
	cobraReflectCmd, err := cli.BuildCobraCommand(reflectCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			ShortHelpLayers: []string{schema.DefaultSlug},
			MiddlewaresFunc: cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building cobra reflect command: %v\n", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(cobraReflectCmd)

	indexCmd := &cobra.Command{
		Use:   "index",
		Short: "Build and inspect the local SQLite/FTS index",
	}
	rootCmd.AddCommand(indexCmd)

	indexBuildCmd, err := NewIndexBuildCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating index build command: %v\n", err)
		os.Exit(1)
	}
	cobraIndexBuildCmd, err := cli.BuildCobraCommand(indexBuildCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			ShortHelpLayers: []string{schema.DefaultSlug},
			MiddlewaresFunc: cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building cobra index build command: %v\n", err)
		os.Exit(1)
	}
	indexCmd.AddCommand(cobraIndexBuildCmd)

	indexStatsCmd, err := NewIndexStatsCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating index stats command: %v\n", err)
		os.Exit(1)
	}
	cobraIndexStatsCmd, err := cli.BuildCobraCommand(indexStatsCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			ShortHelpLayers: []string{schema.DefaultSlug},
			MiddlewaresFunc: cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building cobra index stats command: %v\n", err)
		os.Exit(1)
	}
	indexCmd.AddCommand(cobraIndexStatsCmd)

	cleanupCmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up session artifacts (e.g., reflection copies)",
	}
	rootCmd.AddCommand(cleanupCmd)

	cleanupReflectionCopiesCmd, err := NewCleanupReflectionCopiesCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating cleanup reflection-copies command: %v\n", err)
		os.Exit(1)
	}
	cobraCleanupReflectionCopiesCmd, err := cli.BuildCobraCommand(cleanupReflectionCopiesCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			ShortHelpLayers: []string{schema.DefaultSlug},
			MiddlewaresFunc: cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building cobra cleanup reflection-copies command: %v\n", err)
		os.Exit(1)
	}
	cleanupCmd.AddCommand(cobraCleanupReflectionCopiesCmd)

	tracesCmd := &cobra.Command{
		Use:   "traces",
		Short: "Export curated trace reports",
	}
	rootCmd.AddCommand(tracesCmd)

	tracesMDCmd, err := NewTracesMDCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating traces md command: %v\n", err)
		os.Exit(1)
	}
	cobraTracesMDCmd, err := cli.BuildCobraCommand(tracesMDCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			ShortHelpLayers: []string{schema.DefaultSlug},
			MiddlewaresFunc: cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building cobra traces md command: %v\n", err)
		os.Exit(1)
	}
	tracesCmd.AddCommand(cobraTracesMDCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
