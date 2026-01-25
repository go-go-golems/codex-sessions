package sessions

import (
	"encoding/json"
	"strings"
)

const requestTitleMarker = "## my request for codex:"

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

func hasPrefixedRequestTitle(text string, prefix string) bool {
	if strings.HasPrefix(text, prefix) {
		return true
	}
	lines := strings.Split(text, "\n")
	titleIdx, ok := findRequestTitleIndex(lines)
	if !ok {
		return false
	}
	return strings.HasPrefix(lines[titleIdx], prefix)
}

type eventMsgLine struct {
	Payload struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"payload"`
}

type responseItemLine struct {
	Payload struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"payload"`
}

// IsReflectionCopy checks whether a session file appears to be a reflection copy.
//
// This is a cheap, streaming scan: it looks for the self-reflection prefix on the
// first user message, in either the event_msg or response_item representation.
func IsReflectionCopy(path string, prefix string) (bool, error) {
	if strings.TrimSpace(prefix) == "" {
		return false, nil
	}

	const maxScanLines = 2000
	linesScanned := 0
	seenUserMessage := false

	err := WalkJSONLLines(path, func(line JSONLLine) error {
		linesScanned++
		if linesScanned > maxScanLines && seenUserMessage {
			return errStopWalk
		}

		switch line.Type {
		case "event_msg":
			var msg eventMsgLine
			if err := json.Unmarshal(line.Raw, &msg); err != nil {
				return nil
			}
			if msg.Payload.Type != "user_message" {
				return nil
			}
			if msg.Payload.Message == "" {
				return nil
			}
			seenUserMessage = true
			if hasPrefixedRequestTitle(msg.Payload.Message, prefix) {
				return errFoundReflectionCopy
			}
		case "response_item":
			var item responseItemLine
			if err := json.Unmarshal(line.Raw, &item); err != nil {
				return nil
			}
			if item.Payload.Type != "message" || item.Payload.Role != "user" {
				return nil
			}
			seenUserMessage = true
			for _, c := range item.Payload.Content {
				if c.Type != "input_text" || c.Text == "" {
					continue
				}
				if hasPrefixedRequestTitle(c.Text, prefix) {
					return errFoundReflectionCopy
				}
				break
			}
		}
		return nil
	})
	if err == errFoundReflectionCopy {
		return true, nil
	}
	if err == errStopWalk {
		return false, nil
	}
	return false, err
}

var (
	errStopWalk            = &sentinelError{"stop walk"}
	errFoundReflectionCopy = &sentinelError{"found reflection copy"}
)

type sentinelError struct {
	msg string
}

func (e *sentinelError) Error() string { return e.msg }
