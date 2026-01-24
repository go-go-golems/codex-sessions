package sessions

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// JSONLLine is a minimally parsed JSONL entry from a Codex session file.
type JSONLLine struct {
	Path      string
	LineNo    int
	Type      string
	Timestamp string
	Raw       json.RawMessage
}

type jsonlEnvelope struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
}

// WalkJSONLLines streams a JSONL file line-by-line and calls fn for each parsed line.
//
// This is intentionally tolerant and keeps the full raw JSON line for unknown formats.
func WalkJSONLLines(path string, fn func(line JSONLLine) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Bump buffer size to tolerate larger JSONL lines (tool outputs, etc.).
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		rawBytes := append([]byte(nil), scanner.Bytes()...)
		var env jsonlEnvelope
		if err := json.Unmarshal(rawBytes, &env); err != nil {
			return &ParseError{Path: filepath.Clean(path), LineNo: lineNo, Err: err}
		}
		if err := fn(JSONLLine{
			Path:      path,
			LineNo:    lineNo,
			Type:      env.Type,
			Timestamp: env.Timestamp,
			Raw:       rawBytes,
		}); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
