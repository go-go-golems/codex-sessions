package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
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

func TestStaleIndexSelectionSkipsNewestWhenIncludeMostRecentFalse(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	sessionsRoot := filepath.Join(tmp, "sessions")

	pathA := writeTestSession(
		t,
		sessionsRoot,
		filepath.Join("2026", "03", "02", "rollout-2026-03-02T10-00-00-a.jsonl"),
		"sid-a",
		"/tmp/proj",
		"2026-03-02T10:00:00Z",
		"old-term",
	)
	pathB := writeTestSession(
		t,
		sessionsRoot,
		filepath.Join("2026", "03", "02", "rollout-2026-03-02T10-01-00-b.jsonl"),
		"sid-b",
		"/tmp/proj",
		"2026-03-02T10:01:00Z",
		"new-term",
	)

	metaA, err := sessions.ReadSessionMeta(pathA)
	if err != nil {
		t.Fatalf("ReadSessionMeta A: %v", err)
	}
	metaB, err := sessions.ReadSessionMeta(pathB)
	if err != nil {
		t.Fatalf("ReadSessionMeta B: %v", err)
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

	// Only index the older session. The newer session should not force a stale signal
	// when include-most-recent=false (default selection semantics).
	r := indexdb.BuildSessionIndex(ctx, db, metaA, indexdb.BuildOptions{
		MaxChars:           20000,
		IncludeToolCalls:   true,
		IncludeToolOutputs: true,
	})
	if r.Status != indexdb.SessionIndexed {
		t.Fatalf("expected indexed, got %q (err=%q)", r.Status, r.Error)
	}

	settings := &SearchSettings{
		SessionsRoot:      sessionsRoot,
		IncludeCopies:     false,
		IncludeMostRecent: false,
	}
	metas, err := discoverFilteredMetas(settings, nil, nil)
	if err != nil {
		t.Fatalf("discoverFilteredMetas: %v", err)
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Timestamp.Before(metas[j].Timestamp) })
	if len(metas) != 2 {
		t.Fatalf("expected 2 metas, got %d", len(metas))
	}

	// Mirror the staleness selection logic in search: skip newest session by default.
	newest := metas[len(metas)-1].Timestamp
	filtered := metas[:0]
	for _, m := range metas {
		if !m.Timestamp.Equal(newest) {
			filtered = append(filtered, m)
		}
	}
	metas = filtered

	if len(metas) != 1 || metas[0].ID != metaA.ID {
		t.Fatalf("expected only metaA after filtering, got %+v (metaB=%s)", metas, metaB.ID)
	}

	stale, reason, err := detectStaleIndex(ctx, db, metas)
	if err != nil {
		t.Fatalf("detectStaleIndex: %v", err)
	}
	if stale {
		t.Fatalf("expected non-stale after skipping newest missing meta, got stale: %s", reason)
	}
}
