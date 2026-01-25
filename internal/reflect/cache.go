package reflect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-go-golems/codex-session/internal/sessions"
)

type CacheEntry struct {
	SessionID          string `json:"session_id"`
	SessionTimestamp   string `json:"session_timestamp"`
	Project            string `json:"project"`
	SourcePath         string `json:"source_path"`
	Reflection         string `json:"reflection"`
	CreatedAt          string `json:"created_at"`
	CacheSchemaVersion string `json:"cache_schema_version"`
	PromptVersion      string `json:"prompt_version"`
	PromptUpdatedAt    string `json:"prompt_updated_at"`
	PromptHash         string `json:"prompt_hash"`
	Prompt             string `json:"prompt"`
}

type ConversationInfo struct {
	UpdatedAt    time.Time
	UpdatedAtISO string
	Title        string
}

type CacheDecision struct {
	UseCache bool
	Status   string // fresh|out_of_date
	Reason   string
}

func ParseTimestampBestEffort(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func BuildConversationInfo(meta sessions.SessionMeta) (ConversationInfo, error) {
	updatedAt, err := sessions.ConversationUpdatedAt(meta.Path)
	if err != nil {
		return ConversationInfo{}, err
	}
	title, err := sessions.ConversationTitle(meta.Path, DefaultPrefix, 80)
	if err != nil {
		title = "Untitled conversation"
	}
	return ConversationInfo{
		UpdatedAt:    updatedAt,
		UpdatedAtISO: updatedAt.UTC().Format(time.RFC3339),
		Title:        title,
	}, nil
}

func EnsureCacheDir(sessionsRoot string, cacheDirOverride string) (string, error) {
	cacheDir := cacheDirOverride
	if strings.TrimSpace(cacheDir) == "" {
		cacheDir = filepath.Join(sessionsRoot, DefaultCacheDirName)
	}
	cacheDir = filepath.Clean(cacheDir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	return cacheDir, nil
}

func CachePath(cacheDir string, sessionID string, promptCacheKey string) string {
	return filepath.Join(cacheDir, fmt.Sprintf("%s-%s.json", sessionID, promptCacheKey))
}

func LegacyCachePath(cacheDir string, sessionID string) string {
	return filepath.Join(cacheDir, fmt.Sprintf("%s.json", sessionID))
}

func LoadCacheEntry(path string) (*CacheEntry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entry CacheEntry
	if err := json.Unmarshal(b, &entry); err != nil {
		return nil, err
	}
	if entry.CacheSchemaVersion == "" {
		entry.CacheSchemaVersion = "legacy"
	}
	if entry.PromptVersion == "" {
		entry.PromptVersion = "legacy"
	}
	if entry.PromptUpdatedAt == "" {
		entry.PromptUpdatedAt = "unknown"
	}
	return &entry, nil
}

func WriteCacheEntry(path string, entry CacheEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func refreshDecision(staleness []string, refreshMode string) CacheDecision {
	if len(staleness) > 0 {
		return CacheDecision{
			UseCache: false,
			Status:   "fresh",
			Reason:   "refreshed:" + strings.Join(staleness, ","),
		}
	}
	return CacheDecision{UseCache: false, Status: "fresh", Reason: "refreshed:" + refreshMode}
}

func AssessCacheDecision(
	entry *CacheEntry,
	conversationUpdatedAt time.Time,
	promptUpdatedAt time.Time,
	refreshMode string,
) CacheDecision {
	if entry == nil {
		return CacheDecision{UseCache: false, Status: "fresh", Reason: "generated"}
	}

	createdAt, err := ParseTimestampBestEffort(entry.CreatedAt)
	if err != nil {
		return CacheDecision{UseCache: false, Status: "fresh", Reason: "generated:invalid_created_at"}
	}

	var reasons []string
	if conversationUpdatedAt.After(createdAt) {
		reasons = append(reasons, "conversation_updated_after_reflection")
	}
	if !promptUpdatedAt.IsZero() && promptUpdatedAt.After(createdAt) {
		reasons = append(reasons, "prompt_updated_after_reflection")
	}
	if entry.CacheSchemaVersion != CacheSchemaVersion {
		reasons = append(reasons, "cache_schema_mismatch")
	}

	if refreshMode == "always" {
		return refreshDecision(reasons, refreshMode)
	}
	if refreshMode == "auto" && len(reasons) > 0 {
		autoReasons := make([]string, 0, len(reasons))
		for _, r := range reasons {
			if autoRefreshReasons[r] {
				autoReasons = append(autoReasons, r)
			}
		}
		if len(autoReasons) > 0 {
			return refreshDecision(reasons, refreshMode)
		}
	}
	if len(reasons) > 0 {
		return CacheDecision{UseCache: true, Status: "out_of_date", Reason: strings.Join(reasons, ",")}
	}
	return CacheDecision{UseCache: true, Status: "fresh", Reason: "cache_up_to_date"}
}
