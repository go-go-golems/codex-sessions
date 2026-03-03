package main

import (
	"fmt"
	"os"

	"github.com/go-go-golems/codex-session/pkg/doc"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/spf13/cobra"
)

func defaultParserConfig() cli.CobraParserConfig {
	return cli.CobraParserConfig{
		ShortHelpSections: []string{schema.DefaultSlug},
		MiddlewaresFunc:   cli.CobraCommandDefaultMiddlewares,
	}
}

func buildGlazedCommand[T cmds.Command](label string, ctor func() (T, error)) (*cobra.Command, error) {
	command, err := ctor()
	if err != nil {
		return nil, fmt.Errorf("error creating %s command: %w", label, err)
	}
	cobraCommand, err := cli.BuildCobraCommand(command, cli.WithParserConfig(defaultParserConfig()))
	if err != nil {
		return nil, fmt.Errorf("error building cobra %s command: %w", label, err)
	}
	return cobraCommand, nil
}

func addGlazedCommand[T cmds.Command](parent *cobra.Command, label string, ctor func() (T, error)) error {
	cobraCommand, err := buildGlazedCommand(label, ctor)
	if err != nil {
		return err
	}
	parent.AddCommand(cobraCommand)
	return nil
}

func buildRootCommand() (*cobra.Command, error) {
	rootCmd := &cobra.Command{
		Use:   "codex-session",
		Short: "Query and reflect on Codex session histories",
	}

	if err := addGlazedCommand(rootCmd, "projects", NewProjectsCommand); err != nil {
		return nil, err
	}
	if err := addGlazedCommand(rootCmd, "list", NewListCommand); err != nil {
		return nil, err
	}
	if err := addGlazedCommand(rootCmd, "show", NewShowCommand); err != nil {
		return nil, err
	}
	if err := addGlazedCommand(rootCmd, "search", NewSearchCommand); err != nil {
		return nil, err
	}
	if err := addGlazedCommand(rootCmd, "export", NewExportCommand); err != nil {
		return nil, err
	}
	if err := addGlazedCommand(rootCmd, "reflect", NewReflectCommand); err != nil {
		return nil, err
	}

	indexCmd := &cobra.Command{
		Use:   "index",
		Short: "Build and inspect the local SQLite/FTS index",
	}
	rootCmd.AddCommand(indexCmd)
	if err := addGlazedCommand(indexCmd, "index build", NewIndexBuildCommand); err != nil {
		return nil, err
	}
	if err := addGlazedCommand(indexCmd, "index stats", NewIndexStatsCommand); err != nil {
		return nil, err
	}

	cleanupCmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up session artifacts (e.g., reflection copies)",
	}
	rootCmd.AddCommand(cleanupCmd)
	if err := addGlazedCommand(cleanupCmd, "cleanup reflection-copies", NewCleanupReflectionCopiesCommand); err != nil {
		return nil, err
	}

	tracesCmd := &cobra.Command{
		Use:   "traces",
		Short: "Export curated trace reports",
	}
	rootCmd.AddCommand(tracesCmd)
	if err := addGlazedCommand(tracesCmd, "traces md", NewTracesMDCommand); err != nil {
		return nil, err
	}

	return rootCmd, nil
}

func main() {
	rootCmd, err := buildRootCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	helpSystem := help.NewHelpSystem()
	if err := doc.AddDocToHelpSystem(helpSystem); err != nil {
		fmt.Fprintf(os.Stderr, "failed to load embedded help docs: %v\n", err)
		os.Exit(1)
	}
	help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
