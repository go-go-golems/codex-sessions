package sessions

import (
	"encoding/json"
	"time"
)

// Message is a normalized conversation message extracted from a session log.
type Message struct {
	Timestamp time.Time
	Role      string // user|assistant|system (best-effort)
	Text      string
	Source    string // event_msg|response_item
}

func extractEventMsgMessage(raw json.RawMessage) (*Message, bool) {
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, false
	}
	if data["type"] != "event_msg" {
		return nil, false
	}
	tsStr, _ := data["timestamp"].(string)
	if tsStr == "" {
		return nil, false
	}
	payload, ok := data["payload"].(map[string]any)
	if !ok || payload["type"] != "user_message" {
		return nil, false
	}
	msgStr, _ := payload["message"].(string)
	if msgStr == "" {
		return nil, false
	}
	ts, err := parseTimestamp(tsStr)
	if err != nil {
		return nil, false
	}
	return &Message{
		Timestamp: ts,
		Role:      "user",
		Text:      msgStr,
		Source:    "event_msg",
	}, true
}

func extractResponseItemMessage(raw json.RawMessage) ([]Message, bool) {
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, false
	}
	if data["type"] != "response_item" {
		return nil, false
	}
	tsStr, _ := data["timestamp"].(string)
	if tsStr == "" {
		return nil, false
	}
	payload, ok := data["payload"].(map[string]any)
	if !ok || payload["type"] != "message" {
		return nil, false
	}
	role, _ := payload["role"].(string)
	if role == "" {
		return nil, false
	}
	content, ok := payload["content"].([]any)
	if !ok {
		return nil, false
	}
	ts, err := parseTimestamp(tsStr)
	if err != nil {
		return nil, false
	}

	var out []Message
	for _, itemAny := range content {
		item, ok := itemAny.(map[string]any)
		if !ok {
			continue
		}
		t, _ := item["type"].(string)
		switch t {
		case "input_text":
			if role != "user" {
				continue
			}
			text, _ := item["text"].(string)
			if text == "" {
				continue
			}
			out = append(out, Message{Timestamp: ts, Role: "user", Text: text, Source: "response_item"})
		case "output_text":
			if role != "assistant" {
				continue
			}
			text, _ := item["text"].(string)
			if text == "" {
				continue
			}
			out = append(out, Message{Timestamp: ts, Role: "assistant", Text: text, Source: "response_item"})
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// ExtractMessages streams the session JSONL and returns a normalized message timeline.
//
// It is intentionally best-effort and only supports the message formats used by the existing Python tool:
// - event_msg user_message
// - response_item message with input_text/output_text segments
func ExtractMessages(path string) ([]Message, error) {
	var msgs []Message
	err := WalkJSONLLines(path, func(line JSONLLine) error {
		if msg, ok := extractEventMsgMessage(line.Raw); ok {
			msgs = append(msgs, *msg)
			return nil
		}
		if out, ok := extractResponseItemMessage(line.Raw); ok {
			msgs = append(msgs, out...)
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return msgs, nil
}
