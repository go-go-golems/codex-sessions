package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func childNames(cmd *cobra.Command) map[string]bool {
	out := map[string]bool{}
	for _, c := range cmd.Commands() {
		out[c.Name()] = true
	}
	return out
}

func requireCommand(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("expected command %q under %q", name, parent.Name())
	return nil
}

func TestBuildRootCommandWiring(t *testing.T) {
	root, err := buildRootCommand()
	if err != nil {
		t.Fatalf("buildRootCommand: %v", err)
	}

	top := childNames(root)
	for _, expected := range []string{
		"projects",
		"list",
		"show",
		"search",
		"export",
		"reflect",
		"index",
		"cleanup",
		"traces",
	} {
		if !top[expected] {
			t.Fatalf("missing top-level command %q", expected)
		}
	}

	indexCmd := requireCommand(t, root, "index")
	indexChildren := childNames(indexCmd)
	for _, expected := range []string{"build", "stats"} {
		if !indexChildren[expected] {
			t.Fatalf("missing index subcommand %q", expected)
		}
	}

	cleanupCmd := requireCommand(t, root, "cleanup")
	cleanupChildren := childNames(cleanupCmd)
	if !cleanupChildren["reflection-copies"] {
		t.Fatalf("missing cleanup subcommand %q", "reflection-copies")
	}

	tracesCmd := requireCommand(t, root, "traces")
	tracesChildren := childNames(tracesCmd)
	if !tracesChildren["md"] {
		t.Fatalf("missing traces subcommand %q", "md")
	}
}
