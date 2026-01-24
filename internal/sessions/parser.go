package sessions

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"
)

type rawLine struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
	// Legacy session_meta fields (when the first line is not wrapped)
	ID  string `json:"id"`
	Cwd string `json:"cwd"`
}

type sessionMetaPayload struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
}

func parseTimestamp(value string) (time.Time, error) {
	// Python uses datetime.fromisoformat after replacing Z with +00:00.
	normalized := value
	if strings.HasSuffix(normalized, "Z") {
		normalized = strings.TrimSuffix(normalized, "Z") + "+00:00"
	}
	parsed, err := time.Parse(time.RFC3339Nano, normalized)
	if err != nil {
		// Some logs might omit nanos.
		parsed2, err2 := time.Parse(time.RFC3339, normalized)
		if err2 != nil {
			return time.Time{}, err
		}
		return parsed2.UTC(), nil
	}
	return parsed.UTC(), nil
}

// ReadSessionMeta reads the first JSONL line and extracts session id, timestamp, and cwd.
//
// Supports:
// - new format: {"type":"session_meta","payload":{...}}
// - legacy format: {"id":"...","timestamp":"...","cwd":"..."}
func ReadSessionMeta(path string) (SessionMeta, error) {
	file, err := os.Open(path)
	if err != nil {
		return SessionMeta{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return SessionMeta{}, err
		}
		return SessionMeta{}, ErrNoSessionMeta
	}
	line := scanner.Bytes()
	var raw rawLine
	if err := json.Unmarshal(line, &raw); err != nil {
		return SessionMeta{}, err
	}

	// New format
	if raw.Type == "session_meta" {
		var payload sessionMetaPayload
		if err := json.Unmarshal(raw.Payload, &payload); err != nil {
			return SessionMeta{}, err
		}
		ts, err := parseTimestamp(payload.Timestamp)
		if err != nil {
			return SessionMeta{}, err
		}
		return SessionMeta{ID: payload.ID, Timestamp: ts, Cwd: payload.Cwd, Path: path}, nil
	}

	// Legacy format
	if raw.ID != "" && raw.Timestamp != "" {
		ts, err := parseTimestamp(raw.Timestamp)
		if err != nil {
			return SessionMeta{}, err
		}
		return SessionMeta{ID: raw.ID, Timestamp: ts, Cwd: raw.Cwd, Path: path}, nil
	}

	return SessionMeta{}, ErrNoSessionMeta
}
