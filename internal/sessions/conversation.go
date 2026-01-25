package sessions

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrNoTimestamps = errors.New("no timestamps found in session history")
var errStopScan = errors.New("stop scan")

// ConversationUpdatedAt returns the maximum timestamp observed in the session JSONL.
//
// This mirrors the Python behavior which scans all JSONL lines for a `timestamp` field.
func ConversationUpdatedAt(path string) (time.Time, error) {
	var latest time.Time
	found := false
	err := WalkJSONLLines(path, func(line JSONLLine) error {
		if line.Timestamp == "" {
			return nil
		}
		parsed, err := parseTimestamp(line.Timestamp)
		if err != nil {
			// Ignore unparseable timestamps; treat as absent.
			return nil
		}
		if !found || parsed.After(latest) {
			latest = parsed
			found = true
		}
		return nil
	})
	if err != nil {
		return time.Time{}, err
	}
	if !found {
		return time.Time{}, ErrNoTimestamps
	}
	return latest, nil
}

func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return s[:maxLen-1] + "…"
}

func extractRequestTitle(text string, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.TrimSpace(strings.ToLower(line)) != "## my request for codex:" {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if next == "" {
				continue
			}
			next = strings.TrimPrefix(next, prefix)
			return next
		}
		return ""
	}
	return ""
}

func firstEventUserText(raw json.RawMessage) (string, bool) {
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", false
	}
	if data["type"] != "event_msg" {
		return "", false
	}
	payload, ok := data["payload"].(map[string]any)
	if !ok || payload["type"] != "user_message" {
		return "", false
	}
	msg, ok := payload["message"].(string)
	return msg, ok
}

func firstResponseUserText(raw json.RawMessage) (string, bool) {
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", false
	}
	if data["type"] != "response_item" {
		return "", false
	}
	payload, ok := data["payload"].(map[string]any)
	if !ok || payload["type"] != "message" || payload["role"] != "user" {
		return "", false
	}
	content, ok := payload["content"].([]any)
	if !ok {
		return "", false
	}
	for _, itemAny := range content {
		item, ok := itemAny.(map[string]any)
		if !ok || item["type"] != "input_text" {
			continue
		}
		text, ok := item["text"].(string)
		if ok && text != "" {
			return text, true
		}
	}
	return "", false
}

// ConversationTitle derives a display title for the session.
//
// Behavior mirrors the Python tool:
// - Prefer the first event_msg user_message.
// - Otherwise, use the first response_item user input_text.
// - If the text contains "## my request for codex:", use the first non-empty line after it.
// - Strip the self-reflection prefix and truncate.
func ConversationTitle(path string, prefix string, maxLen int) (string, error) {
	var firstResponse string
	err := WalkJSONLLines(path, func(line JSONLLine) error {
		if msg, ok := firstEventUserText(line.Raw); ok && msg != "" {
			firstResponse = msg
			return errStopScan
		}
		if firstResponse == "" {
			if msg, ok := firstResponseUserText(line.Raw); ok && msg != "" {
				firstResponse = msg
			}
		}
		return nil
	})
	if err != nil {
		if !errors.Is(err, errStopScan) {
			return "", err
		}
	}
	raw := strings.TrimSpace(StripSelfReflectionPrefix(firstResponse))
	req := extractRequestTitle(raw, prefix)
	source := req
	if source == "" {
		source = raw
	}
	source = normalizeSpaces(source)
	if source == "" {
		source = "Untitled conversation"
	}
	return truncate(source, maxLen), nil
}
