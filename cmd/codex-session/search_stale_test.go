package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/codex-session/internal/indexdb"
	"github.com/go-go-golems/codex-session/internal/sessions"
)

func writeTestSession(t *testing.T, root, relPath, sessionID, cwd, ts, message string) string {
	t.Helper()
	fullPath := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	contents := "" +
		`{"type":"session_meta","payload":{"id":"` + sessionID + `","timestamp":"` + ts + `","cwd":"` + cwd + `"}}` + "\n" +
		`{"type":"event_msg","timestamp":"` + ts + `","payload":{"type":"user_message","message":"` + message + `"}}` + "\n"
	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	return fullPath
}

func TestDetectStaleIndex(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	sessionsRoot := filepath.Join(tmp, "sessions")

	path := writeTestSession(
		t,
		sessionsRoot,
		filepath.Join("2026", "03", "02", "rollout-2026-03-02T10-00-00-test.jsonl"),
		"sid-stale-a",
		"/tmp/proj",
		"2026-03-02T10:00:00Z",
		"hello",
	)

	meta, err := sessions.ReadSessionMeta(path)
	if err != nil {
		t.Fatalf("ReadSessionMeta: %v", err)
	}

	indexPath := indexdb.DefaultIndexPath(sessionsRoot)
	db, err := indexdb.Open(indexPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := indexdb.EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	r := indexdb.BuildSessionIndex(ctx, db, meta, indexdb.BuildOptions{
		MaxChars:           20000,
		IncludeToolCalls:   true,
		IncludeToolOutputs: true,
	})
	if r.Status != indexdb.SessionIndexed {
		t.Fatalf("expected indexed, got %q (err=%q)", r.Status, r.Error)
	}

	stale, reason, err := detectStaleIndex(ctx, db, []sessions.SessionMeta{meta})
	if err != nil {
		t.Fatalf("detectStaleIndex (fresh): %v", err)
	}
	if stale {
		t.Fatalf("expected fresh index, got stale: %s", reason)
	}

	// Bump mtime to force a stale signal from filesystem freshness check.
	newMTime := time.Now().Add(2 * time.Minute)
	if err := os.Chtimes(path, newMTime, newMTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	stale, reason, err = detectStaleIndex(ctx, db, []sessions.SessionMeta{meta})
	if err != nil {
		t.Fatalf("detectStaleIndex (mtime stale): %v", err)
	}
	if !stale {
		t.Fatalf("expected stale index after file mtime change")
	}
	if !strings.Contains(reason, "changed after it was indexed") {
		t.Fatalf("unexpected stale reason: %q", reason)
	}

	// A session that is not present in index should also mark stale.
	missingPath := writeTestSession(
		t,
		sessionsRoot,
		filepath.Join("2026", "03", "02", "rollout-2026-03-02T10-10-00-missing.jsonl"),
		"sid-stale-missing",
		"/tmp/proj",
		"2026-03-02T10:10:00Z",
		"new session",
	)
	missingMeta, err := sessions.ReadSessionMeta(missingPath)
	if err != nil {
		t.Fatalf("ReadSessionMeta missing: %v", err)
	}
	stale, reason, err = detectStaleIndex(ctx, db, []sessions.SessionMeta{missingMeta})
	if err != nil {
		t.Fatalf("detectStaleIndex (missing): %v", err)
	}
	if !stale {
		t.Fatalf("expected stale index for missing session")
	}
	if !strings.Contains(reason, "missing from index") {
		t.Fatalf("unexpected stale reason for missing session: %q", reason)
	}
}
