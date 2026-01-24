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
		Use:   "codex-sessions",
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

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
