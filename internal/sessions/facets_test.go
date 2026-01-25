package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractFacets_TextsToolsPathsErrors(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rollout.jsonl")
	contents := "" +
		`{"type":"session_meta","payload":{"id":"x","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/proj"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-01-01T00:00:10Z","payload":{"type":"user_message","message":"Please check /home/me/project/main.go\npanic: boom"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-01T00:00:10Z","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"some output text"}]}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-01T00:00:11Z","payload":{"type":"tool_call","tool_name":"functions.shell_command","arguments":{"command":"rg foo internal/sessions","workdir":"/tmp"}}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-01T00:00:12Z","payload":{"type":"tool_result","tool_name":"functions.shell_command","output":{"exit_code":1,"stderr":"error: something broke\nTraceback ..."}}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	facets, err := ExtractFacets(path, DefaultFacetOptions())
	if err != nil {
		t.Fatalf("ExtractFacets: %v", err)
	}

	if len(facets.Texts) == 0 {
		t.Fatalf("expected some text fields")
	}
	if len(facets.ToolCalls) == 0 {
		t.Fatalf("expected tool calls")
	}
	if len(facets.ToolOutputs) == 0 {
		t.Fatalf("expected tool outputs")
	}

	// Paths
	foundHome := false
	for _, pm := range facets.Paths {
		if pm.Path == "/home/me/project/main.go" {
			foundHome = true
			break
		}
	}
	if !foundHome {
		t.Fatalf("expected to find /home/me/project/main.go in paths: %#v", facets.Paths)
	}

	// Errors
	foundExit := false
	foundPanic := false
	for _, e := range facets.Errors {
		if e.Kind == "exit_code" {
			foundExit = true
		}
		if e.Kind == "panic" {
			foundPanic = true
		}
	}
	if !foundExit {
		t.Fatalf("expected exit_code error signal")
	}
	if !foundPanic {
		t.Fatalf("expected panic error signal")
	}
}

func TestExtractFacets_CustomToolCallAndOutput(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rollout.jsonl")
	contents := "" +
		`{"type":"session_meta","payload":{"id":"x","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/proj"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-01T00:00:11Z","payload":{"type":"custom_tool_call","status":"completed","call_id":"call_1","name":"apply_patch","input":"*** Begin Patch\\n*** End Patch"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-01T00:00:12Z","payload":{"type":"custom_tool_call_output","call_id":"call_1","output":"ok"}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	facets, err := ExtractFacets(path, DefaultFacetOptions())
	if err != nil {
		t.Fatalf("ExtractFacets: %v", err)
	}

	if len(facets.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(facets.ToolCalls))
	}
	if facets.ToolCalls[0].Name != "apply_patch" {
		t.Fatalf("expected tool call name apply_patch, got %q", facets.ToolCalls[0].Name)
	}
	if len(facets.ToolOutputs) != 1 {
		t.Fatalf("expected 1 tool output, got %d", len(facets.ToolOutputs))
	}
	if facets.ToolOutputs[0].Name != "apply_patch" {
		t.Fatalf("expected tool output name apply_patch, got %q", facets.ToolOutputs[0].Name)
	}
}
