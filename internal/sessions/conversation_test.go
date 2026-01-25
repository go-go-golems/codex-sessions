package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConversationUpdatedAt(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rollout.jsonl")
	contents := "" +
		`{"type":"session_meta","payload":{"id":"x","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/p"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-01-01T00:00:10Z","payload":{"type":"user_message","message":"hi"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-01-01T00:00:20Z","payload":{"type":"user_message","message":"bye"}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ts, err := ConversationUpdatedAt(path)
	if err != nil {
		t.Fatalf("ConversationUpdatedAt: %v", err)
	}
	if got := ts.Format("2006-01-02T15:04:05Z"); got != "2026-01-01T00:00:20Z" {
		t.Fatalf("unexpected updated_at: %s", got)
	}
}

func TestConversationTitle_EventMsgPreferred(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rollout.jsonl")
	contents := "" +
		`{"type":"session_meta","payload":{"id":"x","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/p"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-01-01T00:00:10Z","payload":{"type":"user_message","message":"Hello world"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-01T00:00:11Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Other"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	title, err := ConversationTitle(path, DefaultSelfReflectionPrefix, 80)
	if err != nil {
		t.Fatalf("ConversationTitle: %v", err)
	}
	if title != "Hello world" {
		t.Fatalf("unexpected title: %q", title)
	}
}

func TestConversationTitle_RequestMarker(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rollout.jsonl")
	contents := "" +
		`{"type":"session_meta","payload":{"id":"x","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/p"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-01-01T00:00:10Z","payload":{"type":"user_message","message":"## My request for Codex:\n\nDo the thing\n\nMore context"}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	title, err := ConversationTitle(path, DefaultSelfReflectionPrefix, 80)
	if err != nil {
		t.Fatalf("ConversationTitle: %v", err)
	}
	if title != "Do the thing" {
		t.Fatalf("unexpected title: %q", title)
	}
}
