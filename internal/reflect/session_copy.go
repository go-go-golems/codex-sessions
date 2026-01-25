package reflect

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const requestTitleMarker = "## my request for codex:"

func formatRolloutTimestamp(t time.Time) string {
	return t.UTC().Format("2006-01-02T15-04-05")
}

func buildRolloutFilename(timestamp string, sessionID string) string {
	return fmt.Sprintf("rollout-%s-%s.jsonl", timestamp, sessionID)
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024*32)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func writeLines(path string, lines []string) error {
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func updateSessionID(lines []string, newID string) ([]string, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("session file is empty")
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		return nil, err
	}
	if typ, _ := first["type"].(string); typ == "session_meta" {
		if payload, ok := first["payload"].(map[string]any); ok {
			payload["id"] = newID
			first["payload"] = payload
		} else {
			return nil, fmt.Errorf("session_meta payload is not an object")
		}
	} else if _, ok := first["id"]; ok {
		first["id"] = newID
	} else {
		return nil, fmt.Errorf("first line is not session_meta and has no id field")
	}
	b, err := json.Marshal(first)
	if err != nil {
		return nil, err
	}
	lines[0] = string(b)
	return lines, nil
}

func findRequestTitleIndex(lines []string) (int, bool) {
	for i, line := range lines {
		if strings.TrimSpace(strings.ToLower(line)) != requestTitleMarker {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) != "" {
				return j, true
			}
		}
		return 0, false
	}
	return 0, false
}

func normalizeUserText(text string, prefix string) string {
	normalized := text
	if strings.HasPrefix(normalized, prefix) {
		normalized = strings.TrimPrefix(normalized, prefix)
	}
	lines := strings.Split(normalized, "\n")
	titleIdx, ok := findRequestTitleIndex(lines)
	if !ok {
		return normalized
	}
	titleLine := lines[titleIdx]
	if !strings.HasPrefix(titleLine, prefix) {
		return normalized
	}
	lines[titleIdx] = strings.TrimPrefix(titleLine, prefix)
	return strings.Join(lines, "\n")
}

func prefixRequestTitle(text string, prefix string) (string, bool) {
	lines := strings.Split(text, "\n")
	titleIdx, ok := findRequestTitleIndex(lines)
	if !ok {
		if strings.HasPrefix(text, prefix) {
			return text, false
		}
		return prefix + text, true
	}
	titleLine := lines[titleIdx]
	if strings.HasPrefix(titleLine, prefix) {
		return text, false
	}
	lines[titleIdx] = prefix + titleLine
	return strings.Join(lines, "\n"), true
}

func firstEventUserText(lines []string) (string, bool) {
	for _, line := range lines {
		var data map[string]any
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}
		if data["type"] != "event_msg" {
			continue
		}
		payload, ok := data["payload"].(map[string]any)
		if !ok || payload["type"] != "user_message" {
			continue
		}
		if msg, ok := payload["message"].(string); ok && msg != "" {
			return msg, true
		}
	}
	return "", false
}

func firstResponseUserText(lines []string) (string, bool) {
	for _, line := range lines {
		var data map[string]any
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}
		if data["type"] != "response_item" {
			continue
		}
		payload, ok := data["payload"].(map[string]any)
		if !ok || payload["type"] != "message" || payload["role"] != "user" {
			continue
		}
		content, ok := payload["content"].([]any)
		if !ok {
			continue
		}
		for _, itemAny := range content {
			item, ok := itemAny.(map[string]any)
			if !ok || item["type"] != "input_text" {
				continue
			}
			if text, ok := item["text"].(string); ok && text != "" {
				return text, true
			}
		}
	}
	return "", false
}

func prefixFirstUserMessage(lines []string, prefix string) ([]string, error) {
	target, ok := firstEventUserText(lines)
	targetSource := "event_msg"
	if !ok {
		target, ok = firstResponseUserText(lines)
		targetSource = "response_item"
	}
	if !ok || strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("no user message found to prefix")
	}
	targetNorm := normalizeUserText(target, prefix)

	updatedEvent := false
	updatedResponse := false
	foundResponse := false

	updatedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		var data map[string]any
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			updatedLines = append(updatedLines, line)
			continue
		}

		if data["type"] == "event_msg" {
			payload, ok := data["payload"].(map[string]any)
			if ok && payload["type"] == "user_message" {
				if msg, ok := payload["message"].(string); ok {
					if normalizeUserText(msg, prefix) == targetNorm {
						updated, changed := prefixRequestTitle(msg, prefix)
						if changed {
							payload["message"] = updated
							data["payload"] = payload
							updatedEvent = true
							if b, err := json.Marshal(data); err == nil {
								line = string(b)
							}
						}
					}
				}
			}
		}

		if data["type"] == "response_item" {
			payload, ok := data["payload"].(map[string]any)
			if ok && payload["type"] == "message" && payload["role"] == "user" {
				content, ok := payload["content"].([]any)
				if ok {
					foundResponse = true
					changedAny := false
					for i, itemAny := range content {
						item, ok := itemAny.(map[string]any)
						if !ok || item["type"] != "input_text" {
							continue
						}
						text, ok := item["text"].(string)
						if !ok {
							continue
						}
						if normalizeUserText(text, prefix) != targetNorm {
							continue
						}
						updated, changed := prefixRequestTitle(text, prefix)
						if changed {
							item["text"] = updated
							content[i] = item
							updatedResponse = true
							changedAny = true
						}
					}
					if changedAny {
						payload["content"] = content
						data["payload"] = payload
						if b, err := json.Marshal(data); err == nil {
							line = string(b)
						}
					}
				}
			}
		}

		updatedLines = append(updatedLines, line)
	}

	if targetSource == "event_msg" && !updatedEvent {
		return nil, fmt.Errorf("no matching event_msg user_message found to prefix")
	}
	if foundResponse && !updatedResponse {
		return nil, fmt.Errorf("no matching response_item user message found to prefix")
	}
	return updatedLines, nil
}

func copyFile(source string, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("destination already exists: %s", dest)
	}
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Close()
}

func CreateCopyWithNewID(sourcePath string, prefix string) (copyPath string, copyID string, err error) {
	ts := formatRolloutTimestamp(time.Now().UTC())
	newID := uuid.NewString()
	destDir := filepath.Dir(sourcePath)
	destPath := filepath.Join(destDir, buildRolloutFilename(ts, newID))

	if err := copyFile(sourcePath, destPath); err != nil {
		return "", "", err
	}

	lines, err := readLines(destPath)
	if err != nil {
		return "", "", err
	}
	lines, err = updateSessionID(lines, newID)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(prefix) != "" {
		lines, err = prefixFirstUserMessage(lines, prefix)
		if err != nil {
			return "", "", err
		}
	}
	if err := writeLines(destPath, lines); err != nil {
		return "", "", err
	}

	return destPath, newID, nil
}
