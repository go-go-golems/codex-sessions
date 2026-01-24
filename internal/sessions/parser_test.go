package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSessionMeta_NewFormat(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rollout-2026-01-01T00-00-00-00000000-0000-0000-0000-000000000000.jsonl")
	contents := `{"type":"session_meta","payload":{"id":"11111111-1111-1111-1111-111111111111","timestamp":"2026-01-01T00:00:00Z","cwd":"/home/me/project"}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	meta, err := ReadSessionMeta(path)
	if err != nil {
		t.Fatalf("ReadSessionMeta: %v", err)
	}
	if meta.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected id: %s", meta.ID)
	}
	if meta.Cwd != "/home/me/project" {
		t.Fatalf("unexpected cwd: %s", meta.Cwd)
	}
	if got := meta.ProjectName(); got != "project" {
		t.Fatalf("unexpected project: %s", got)
	}
	if meta.Timestamp.IsZero() {
		t.Fatalf("timestamp should not be zero")
	}
}

func TestReadSessionMeta_LegacyFormat(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rollout-2026-01-01T00-00-00-00000000-0000-0000-0000-000000000000.jsonl")
	contents := `{"id":"22222222-2222-2222-2222-222222222222","timestamp":"2026-01-01T00:00:00+00:00","cwd":"/tmp/foo"}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	meta, err := ReadSessionMeta(path)
	if err != nil {
		t.Fatalf("ReadSessionMeta: %v", err)
	}
	if meta.ID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("unexpected id: %s", meta.ID)
	}
	if got := meta.ProjectName(); got != "foo" {
		t.Fatalf("unexpected project: %s", got)
	}
}
