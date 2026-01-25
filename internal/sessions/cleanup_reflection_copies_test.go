package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupReflectionCopies_DryRunAndDelete(t *testing.T) {
	root := t.TempDir()
	copyPath := filepath.Join(root, "rollout-2026-01-01T00-00-00-copy.jsonl")
	normalPath := filepath.Join(root, "rollout-2026-01-01T00-00-00-normal.jsonl")

	copyContent := `{"type":"session_meta","payload":{"id":"copy","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/proj"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"` + DefaultSelfReflectionPrefix + `hello"}}` + "\n"
	if err := os.WriteFile(copyPath, []byte(copyContent), 0o644); err != nil {
		t.Fatalf("write copy: %v", err)
	}
	if err := os.WriteFile(normalPath, []byte(`{"type":"session_meta","payload":{"id":"normal","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/proj"}}`+"\n"), 0o644); err != nil {
		t.Fatalf("write normal: %v", err)
	}

	results, err := CleanupReflectionCopies(root, DefaultSelfReflectionPrefix, CleanupReflectionCopiesOptions{DryRun: true})
	if err != nil {
		t.Fatalf("CleanupReflectionCopies dry-run: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 match, got %d: %#v", len(results), results)
	}
	if results[0].Status != "would_delete" {
		t.Fatalf("expected would_delete, got %q", results[0].Status)
	}
	if _, err := os.Stat(copyPath); err != nil {
		t.Fatalf("expected file to still exist in dry-run: %v", err)
	}

	results, err = CleanupReflectionCopies(root, DefaultSelfReflectionPrefix, CleanupReflectionCopiesOptions{DryRun: false})
	if err != nil {
		t.Fatalf("CleanupReflectionCopies delete: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 match, got %d: %#v", len(results), results)
	}
	if results[0].Status != "deleted" {
		t.Fatalf("expected deleted, got %q (%s)", results[0].Status, results[0].Error)
	}
	if _, err := os.Stat(copyPath); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, got stat err=%v", err)
	}
	if _, err := os.Stat(normalPath); err != nil {
		t.Fatalf("expected normal file to remain: %v", err)
	}
}

func TestCleanupReflectionCopies_RespectsLimit(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "rollout-2026-01-01T00-00-00-a.jsonl"),
		filepath.Join(root, "rollout-2026-01-01T00-00-00-b.jsonl"),
	}
	for _, p := range paths {
		content := `{"type":"session_meta","payload":{"id":"x","timestamp":"2026-01-01T00:00:00Z","cwd":"/tmp/proj"}}` + "\n" +
			`{"type":"event_msg","payload":{"type":"user_message","message":"` + DefaultSelfReflectionPrefix + `hello"}}` + "\n"
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	results, err := CleanupReflectionCopies(root, DefaultSelfReflectionPrefix, CleanupReflectionCopiesOptions{DryRun: false, Limit: 1})
	if err != nil {
		t.Fatalf("CleanupReflectionCopies: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %#v", len(results), results)
	}

	remaining := 0
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			remaining++
		}
	}
	if remaining != 1 {
		t.Fatalf("expected 1 remaining file, got %d", remaining)
	}
}
