package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverRolloutFiles_SortsAndFilters(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "2026", "01", "01", "rollout-2026-01-01T00-00-00-a.jsonl"),
		filepath.Join(root, "2026", "01", "02", "rollout-2026-01-02T00-00-00-b-copy.jsonl"),
		filepath.Join(root, "2026", "01", "03", "rollout-2026-01-03T00-00-00-c.jsonl"),
		filepath.Join(root, "2026", "01", "03", "not-a-rollout.jsonl"),
	}
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	found, err := DiscoverRolloutFiles(root)
	if err != nil {
		t.Fatalf("DiscoverRolloutFiles: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("expected 2 files, got %d: %#v", len(found), found)
	}
	if found[0] > found[1] {
		t.Fatalf("expected sorted output, got: %#v", found)
	}
	for _, p := range found {
		if filepath.Base(p) == "rollout-2026-01-02T00-00-00-b-copy.jsonl" {
			t.Fatalf("should have excluded -copy file: %s", p)
		}
	}
}
