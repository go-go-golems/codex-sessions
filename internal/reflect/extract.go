package reflect

import (
	"encoding/json"
	"fmt"
	"strings"
)

func extractAssistantOutputText(payload map[string]any) string {
	content, ok := payload["content"].([]any)
	if !ok {
		return ""
	}
	var chunks []string
	for _, itemAny := range content {
		item, ok := itemAny.(map[string]any)
		if !ok || item["type"] != "output_text" {
			continue
		}
		if text, ok := item["text"].(string); ok && text != "" {
			chunks = append(chunks, text)
		}
	}
	return strings.TrimSpace(strings.Join(chunks, ""))
}

func ExtractLastAssistantText(lines []string) (string, error) {
	last := ""
	for _, line := range lines {
		var data map[string]any
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}
		if data["type"] != "response_item" {
			continue
		}
		payload, ok := data["payload"].(map[string]any)
		if !ok {
			continue
		}
		if payload["type"] != "message" || payload["role"] != "assistant" {
			continue
		}
		text := extractAssistantOutputText(payload)
		if text != "" {
			last = text
		}
	}
	if strings.TrimSpace(last) == "" {
		return "", fmt.Errorf("no assistant response found in session")
	}
	return last, nil
}

func ExtractLastAssistantTextFromFile(path string) (string, error) {
	lines, err := readLines(path)
	if err != nil {
		return "", err
	}
	return ExtractLastAssistantText(lines)
}
