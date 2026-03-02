package indexdb

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/codex-session/internal/sessions"
)

func TestBuildSessionIndexAndSearch(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "2026", "01", "01")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "rollout-2026-01-01T00-00-00-test.jsonl")
	contents := "" +
		`{"type":"session_meta","payload":{"id":"sid","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/proj"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-01-01T00:00:10Z","payload":{"type":"user_message","message":"hello world /tmp/test.txt"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-01T00:00:11Z","payload":{"type":"custom_tool_call","status":"completed","call_id":"call_1","name":"functions.shell_command","input":"{\"command\":\"echo hi\"}"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-01T00:00:12Z","payload":{"type":"custom_tool_call_output","call_id":"call_1","output":"ok"}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	meta, err := sessions.ReadSessionMeta(path)
	if err != nil {
		t.Fatalf("ReadSessionMeta: %v", err)
	}

	db, err := Open(DefaultIndexPath(tmp))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	r1 := BuildSessionIndex(ctx, db, meta, BuildOptions{
		MaxChars:           20000,
		IncludeToolCalls:   true,
		IncludeToolOutputs: true,
	})
	if r1.Status != SessionIndexed {
		t.Fatalf("expected indexed, got %q (err=%q)", r1.Status, r1.Error)
	}

	stats, err := GetStats(ctx, db)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.Sessions != 1 || stats.Messages == 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	msgHits, err := Search(ctx, db, SearchOptions{Query: "hello", Scope: ScopeMessages, MaxResults: 10})
	if err != nil {
		t.Fatalf("Search messages: %v", err)
	}
	if len(msgHits) == 0 || msgHits[0].Kind != "message" {
		t.Fatalf("expected message hits, got %#v", msgHits)
	}

	toolHits, err := Search(ctx, db, SearchOptions{Query: "echo", Scope: ScopeTools, MaxResults: 10})
	if err != nil {
		t.Fatalf("Search tools: %v", err)
	}
	if len(toolHits) == 0 {
		t.Fatalf("expected tool hits")
	}

	r2 := BuildSessionIndex(ctx, db, meta, BuildOptions{
		MaxChars:           20000,
		IncludeToolCalls:   true,
		IncludeToolOutputs: true,
	})
	if r2.Status != SessionSkipped {
		t.Fatalf("expected skipped on second run, got %q (err=%q)", r2.Status, r2.Error)
	}

	// Force rebuild should index again.
	r3 := BuildSessionIndex(ctx, db, meta, BuildOptions{
		Force:              true,
		MaxChars:           20000,
		IncludeToolCalls:   true,
		IncludeToolOutputs: true,
	})
	if r3.Status != SessionIndexed {
		t.Fatalf("expected indexed with force, got %q (err=%q)", r3.Status, r3.Error)
	}

	// Ensure started/updated timestamps are RFC3339 parseable.
	if _, err := time.Parse(time.RFC3339, r3.StartedAt); err != nil {
		t.Fatalf("started_at not RFC3339: %v", err)
	}
	if _, err := time.Parse(time.RFC3339, r3.UpdatedAt); err != nil {
		t.Fatalf("updated_at not RFC3339: %v", err)
	}
}

func TestSearchLiteralQueryWithPunctuation(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "2026", "01", "02")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "rollout-2026-01-02T00-00-00-test.jsonl")
	contents := "" +
		`{"type":"session_meta","payload":{"id":"sid2","timestamp":"2026-01-02T00:00:00Z","cwd":"/tmp/proj"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-01-02T00:00:10Z","payload":{"type":"user_message","message":"Investigate CODEX-001 in go-go-os at /tmp/test.txt and call functions.shell_command with foo/bar"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-02T00:00:11Z","payload":{"type":"custom_tool_call","status":"completed","call_id":"call_1","name":"functions.shell_command","input":"{\"command\":\"echo hi\"}"}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	meta, err := sessions.ReadSessionMeta(path)
	if err != nil {
		t.Fatalf("ReadSessionMeta: %v", err)
	}

	db, err := Open(DefaultIndexPath(tmp))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	r := BuildSessionIndex(ctx, db, meta, BuildOptions{
		MaxChars:           20000,
		IncludeToolCalls:   true,
		IncludeToolOutputs: true,
	})
	if r.Status != SessionIndexed {
		t.Fatalf("expected indexed, got %q (err=%q)", r.Status, r.Error)
	}

	queries := []string{
		"CODEX-001",
		"go-go-os",
		"/tmp/test.txt",
		"functions.shell_command",
		"foo/bar",
	}
	for _, query := range queries {
		hits, err := Search(ctx, db, SearchOptions{
			Query:      query,
			Scope:      ScopeMessages,
			MaxResults: 10,
		})
		if err != nil {
			t.Fatalf("Search(%q): %v", query, err)
		}
		if len(hits) == 0 {
			t.Fatalf("Search(%q): expected hits", query)
		}
	}
}

func TestSearchScopesWithPunctuationAndToolOutputs(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "2026", "01", "03")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "rollout-2026-01-03T00-00-00-test.jsonl")
	contents := "" +
		`{"type":"session_meta","payload":{"id":"sid3","timestamp":"2026-01-03T00:00:00Z","cwd":"/tmp/proj"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-01-03T00:00:10Z","payload":{"type":"user_message","message":"message-only-token"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-03T00:00:11Z","payload":{"type":"custom_tool_call","status":"completed","call_id":"call_1","name":"functions.shell_command","input":"{\"command\":\"run /tmp/tool-call.txt with functions.shell_command\"}"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-03T00:00:12Z","payload":{"type":"custom_tool_call_output","call_id":"call_1","output":"tool-output-token from foo/bar"}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	meta, err := sessions.ReadSessionMeta(path)
	if err != nil {
		t.Fatalf("ReadSessionMeta: %v", err)
	}

	db, err := Open(DefaultIndexPath(tmp))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	r := BuildSessionIndex(ctx, db, meta, BuildOptions{
		MaxChars:           20000,
		IncludeToolCalls:   true,
		IncludeToolOutputs: true,
	})
	if r.Status != SessionIndexed {
		t.Fatalf("expected indexed, got %q (err=%q)", r.Status, r.Error)
	}

	msgHits, err := Search(ctx, db, SearchOptions{
		Query:      "message-only-token",
		Scope:      ScopeMessages,
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("Search messages: %v", err)
	}
	if len(msgHits) == 0 || msgHits[0].Kind != "message" {
		t.Fatalf("expected message hit, got %#v", msgHits)
	}

	toolCallHits, err := Search(ctx, db, SearchOptions{
		Query:      "/tmp/tool-call.txt",
		Scope:      ScopeTools,
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("Search tool calls: %v", err)
	}
	if len(toolCallHits) == 0 {
		t.Fatalf("expected tool-call hits")
	}
	if toolCallHits[0].Kind != "tool_call" {
		t.Fatalf("expected tool_call kind, got %#v", toolCallHits[0])
	}

	toolOutputHits, err := Search(ctx, db, SearchOptions{
		Query:      "foo/bar",
		Scope:      ScopeTools,
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("Search tool outputs: %v", err)
	}
	if len(toolOutputHits) == 0 {
		t.Fatalf("expected tool-output hits")
	}
	foundToolOutput := false
	for _, h := range toolOutputHits {
		if h.Kind == "tool_output" {
			foundToolOutput = true
			break
		}
	}
	if !foundToolOutput {
		t.Fatalf("expected at least one tool_output hit, got %#v", toolOutputHits)
	}

	allHits, err := Search(ctx, db, SearchOptions{
		Query:      "tool-output-token",
		Scope:      ScopeAll,
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("Search all scope: %v", err)
	}
	if len(allHits) == 0 {
		t.Fatalf("expected hits for scope=all")
	}
}
