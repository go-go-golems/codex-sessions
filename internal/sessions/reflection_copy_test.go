package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsReflectionCopy_EventMsg_RequestTitlePrefixed(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "rollout-2026-01-01T00-00-00-a.jsonl")
	payload := `{"type":"session_meta","payload":{"id":"a","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/proj"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"## my request for codex:\n` + DefaultSelfReflectionPrefix + `do the thing\n\nmore"}}` + "\n"
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	isCopy, err := IsReflectionCopy(path, DefaultSelfReflectionPrefix)
	if err != nil {
		t.Fatalf("IsReflectionCopy: %v", err)
	}
	if !isCopy {
		t.Fatalf("expected reflection copy")
	}
}

func TestIsReflectionCopy_ResponseItem_PrefixedAtStart(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "rollout-2026-01-01T00-00-00-a.jsonl")
	payload := `{"type":"session_meta","payload":{"id":"a","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/proj"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"` + DefaultSelfReflectionPrefix + `hello"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	isCopy, err := IsReflectionCopy(path, DefaultSelfReflectionPrefix)
	if err != nil {
		t.Fatalf("IsReflectionCopy: %v", err)
	}
	if !isCopy {
		t.Fatalf("expected reflection copy")
	}
}

func TestIsReflectionCopy_NotCopy(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "rollout-2026-01-01T00-00-00-a.jsonl")
	payload := `{"type":"session_meta","payload":{"id":"a","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/proj"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"## my request for codex:\ndo the thing"}}` + "\n"
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	isCopy, err := IsReflectionCopy(path, DefaultSelfReflectionPrefix)
	if err != nil {
		t.Fatalf("IsReflectionCopy: %v", err)
	}
	if isCopy {
		t.Fatalf("expected not a reflection copy")
	}
}
