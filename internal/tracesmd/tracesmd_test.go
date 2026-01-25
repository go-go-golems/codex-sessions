package tracesmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildMarkdown_RendersMultilineSafely(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "rollout-2026-01-01T00-00-00-a.jsonl")
	content := `{"type":"session_meta","payload":{"id":"a","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/proj"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"tool_result","output":"line1\nline2"}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	lines, err := BuildMarkdown([]string{path}, Options{EntriesPerFile: 1, MaxStrLen: 0, MaxListLen: 0})
	if err != nil {
		t.Fatalf("BuildMarkdown: %v", err)
	}
	rendered := strings.Join(lines, "\n")
	if !strings.Contains(rendered, "**output**") {
		t.Fatalf("expected output section, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "```") && !strings.Contains(rendered, "````") {
		t.Fatalf("expected fenced code block, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, `"""`) {
		t.Fatalf("expected triple-quoted multiline rendering, got:\n%s", rendered)
	}
}

func TestTruncation_TruncatesStringsAndLists(t *testing.T) {
	v := map[string]any{
		"s": strings.Repeat("a", 10),
		"l": []any{1, 2, 3, 4, 5},
	}
	v2 := truncateStrings(v, 5).(map[string]any)
	if s := v2["s"].(string); s != "aaaa…" {
		t.Fatalf("expected truncated string, got %q", s)
	}
	v3 := truncateLists(v, 3).(map[string]any)
	if l := v3["l"].([]any); len(l) != 3 {
		t.Fatalf("expected truncated list length 3, got %d", len(l))
	}
}
