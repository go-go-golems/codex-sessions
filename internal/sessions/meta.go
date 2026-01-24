package sessions

import (
	"errors"
	"path/filepath"
	"strings"
	"time"
)

// SessionMeta is minimal metadata extracted from the first JSONL line.
//
// It is intentionally small and tolerant: we only require id + timestamp.
type SessionMeta struct {
	ID        string
	Timestamp time.Time
	Cwd       string
	Path      string
}

func (m SessionMeta) ProjectName() string {
	if m.Cwd == "" {
		return "unknown"
	}
	clean := filepath.Clean(m.Cwd)
	base := filepath.Base(clean)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "unknown"
	}
	return base
}

var ErrNoSessionMeta = errors.New("session_meta not found")

// StripSelfReflectionPrefix removes the reflection prefix when present.
func StripSelfReflectionPrefix(text string) string {
	const prefix = "[SELF-REFLECTION] "
	return strings.TrimPrefix(text, prefix)
}
