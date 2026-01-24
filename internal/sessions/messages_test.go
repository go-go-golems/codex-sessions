package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMessages_EventAndResponse(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rollout.jsonl")
	contents := "" +
		`{"type":"session_meta","payload":{"id":"x","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/p"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-01-01T00:00:10Z","payload":{"type":"user_message","message":"hello"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-01T00:00:11Z","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-01-01T00:00:12Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"next"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	msgs, err := ExtractMessages(path)
	if err != nil {
		t.Fatalf("ExtractMessages: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d: %#v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Text != "hello" || msgs[0].Source != "event_msg" {
		t.Fatalf("unexpected first message: %#v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Text != "hi" {
		t.Fatalf("unexpected second message: %#v", msgs[1])
	}
	if msgs[2].Role != "user" || msgs[2].Text != "next" {
		t.Fatalf("unexpected third message: %#v", msgs[2])
	}
}
