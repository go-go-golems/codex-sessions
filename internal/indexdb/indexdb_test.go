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
