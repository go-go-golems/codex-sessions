package tracesmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-go-golems/codex-session/internal/sessions"
)

const (
	indent      = "  "
	defaultHead = "# Trace Examples (response_item text/arguments/output only)"
)

var errStopWalk = errors.New("stop walk")

type Options struct {
	EntriesPerFile int
	MaxStrLen      int
	MaxListLen     int

	IncludeEntryMetadata bool
	PayloadTypes         []string
	IncludeRawPayload    bool
}

func truncateStrings(value any, limit int) any {
	if limit <= 0 {
		return value
	}
	switch v := value.(type) {
	case string:
		runes := []rune(v)
		if len(runes) <= limit {
			return v
		}
		if limit <= 1 {
			return "…"
		}
		return string(runes[:limit-1]) + "…"
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, truncateStrings(item, limit))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, val := range v {
			out[k] = truncateStrings(val, limit)
		}
		return out
	default:
		return value
	}
}

func truncateLists(value any, limit int) any {
	if limit <= 0 {
		return value
	}
	switch v := value.(type) {
	case []any:
		if len(v) > limit {
			v = v[:limit]
		}
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, truncateLists(item, limit))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, val := range v {
			out[k] = truncateLists(val, limit)
		}
		return out
	default:
		return value
	}
}

func collectTexts(value any) []string {
	out := []string{}
	switch v := value.(type) {
	case map[string]any:
		for k, val := range v {
			if k == "text" {
				if s, ok := val.(string); ok {
					out = append(out, s)
					continue
				}
			}
			out = append(out, collectTexts(val)...)
		}
	case []any:
		for _, item := range v {
			out = append(out, collectTexts(item)...)
		}
	}
	return out
}

func collectByKey(value any, target string) []any {
	out := []any{}
	switch v := value.(type) {
	case map[string]any:
		for k, val := range v {
			if k == target {
				out = append(out, val)
				continue
			}
			out = append(out, collectByKey(val, target)...)
		}
	case []any:
		for _, item := range v {
			out = append(out, collectByKey(item, target)...)
		}
	}
	return out
}

func reasoningTextSource(payload map[string]any) any {
	if v, ok := payload["content"]; ok && v != nil {
		return v
	}
	return payload["summary"]
}

func buildPayloadView(payload map[string]any) map[string]any {
	payloadType, _ := payload["type"].(string)

	var texts []string
	if payloadType == "reasoning" {
		texts = collectTexts(reasoningTextSource(payload))
	} else {
		texts = collectTexts(payload)
	}

	textItems := make([]any, 0, len(texts))
	for _, t := range texts {
		textItems = append(textItems, t)
	}

	return map[string]any{
		"text":      textItems,
		"arguments": collectByKey(payload, "arguments"),
		"output":    collectByKey(payload, "output"),
	}
}

func renderMultiline(value string, indentPrefix string) []string {
	lines := []string{`"""`, ""}
	for _, line := range strings.Split(value, "\n") {
		lines = append(lines, indentPrefix+line)
	}
	lines = append(lines, `"""`)
	return lines
}

func renderJSON(value any, indentLevel int) []string {
	curIndent := strings.Repeat(indent, indentLevel)
	nextIndent := strings.Repeat(indent, indentLevel+1)

	switch v := value.(type) {
	case map[string]any:
		if len(v) == 0 {
			return []string{"{}"}
		}
		lines := []string{"{"}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			keyJSON, _ := json.Marshal(k)
			val := v[k]
			if s, ok := val.(string); ok && strings.Contains(s, "\n") {
				lines = append(lines, fmt.Sprintf("%s%s: \"\"\"", nextIndent, string(keyJSON)))
				lines = append(lines, nextIndent)
				for _, line := range strings.Split(s, "\n") {
					lines = append(lines, nextIndent+indent+line)
				}
				lines = append(lines, fmt.Sprintf("%s\"\"\"", nextIndent))
			} else {
				rendered := renderJSON(val, indentLevel+1)
				lines = append(lines, fmt.Sprintf("%s%s: %s", nextIndent, string(keyJSON), rendered[0]))
				if len(rendered) > 1 {
					lines = append(lines, rendered[1:]...)
				}
			}
			if i < len(keys)-1 {
				lines[len(lines)-1] = lines[len(lines)-1] + ","
			}
		}
		lines = append(lines, fmt.Sprintf("%s}", curIndent))
		return lines
	case []any:
		if len(v) == 0 {
			return []string{"[]"}
		}
		lines := []string{"["}
		for i, item := range v {
			rendered := renderJSON(item, indentLevel+1)
			lines = append(lines, fmt.Sprintf("%s%s", nextIndent, rendered[0]))
			if len(rendered) > 1 {
				lines = append(lines, rendered[1:]...)
			}
			if i < len(v)-1 {
				lines[len(lines)-1] = lines[len(lines)-1] + ","
			}
		}
		lines = append(lines, fmt.Sprintf("%s]", curIndent))
		return lines
	case string:
		if strings.Contains(v, "\n") {
			return renderMultiline(v, curIndent)
		}
		b, _ := json.Marshal(v)
		return []string{string(b)}
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return []string{fmt.Sprintf("%q", fmt.Sprintf("%v", v))}
		}
		return []string{string(b)}
	}
}

func formatListLines(items []any, parseJSON bool) []string {
	lines := []string{}
	for _, item := range items {
		if parseJSON {
			if s, ok := item.(string); ok {
				var parsed any
				if err := json.Unmarshal([]byte(s), &parsed); err == nil {
					switch parsed.(type) {
					case map[string]any, []any:
						lines = append(lines, renderJSON(parsed, 0)...)
						continue
					case string:
						if strings.Contains(parsed.(string), "\n") {
							lines = append(lines, renderMultiline(parsed.(string), "")...)
							continue
						}
					}
					b, _ := json.Marshal(parsed)
					lines = append(lines, string(b))
					continue
				}
				if strings.Contains(s, "\n") {
					lines = append(lines, renderMultiline(s, "")...)
					continue
				}
				lines = append(lines, s)
				continue
			}
		}

		switch v := item.(type) {
		case map[string]any, []any:
			lines = append(lines, renderJSON(v, 0)...)
		case string:
			if strings.Contains(v, "\n") {
				lines = append(lines, renderMultiline(v, "")...)
			} else {
				lines = append(lines, v)
			}
		default:
			b, err := json.Marshal(v)
			if err != nil {
				lines = append(lines, fmt.Sprintf("%v", v))
			} else {
				lines = append(lines, string(b))
			}
		}
	}
	return lines
}

func renderCodeFenceBlock(contentLines []string) []string {
	// Pick a fence that is longer than any run of backticks in the content.
	maxTicks := 0
	for _, line := range contentLines {
		run := 0
		for i := 0; i < len(line); i++ {
			if line[i] == '`' {
				run++
				if run > maxTicks {
					maxTicks = run
				}
				continue
			}
			run = 0
		}
	}
	fence := strings.Repeat("`", maxTicks+3)
	out := []string{fence}
	out = append(out, contentLines...)
	out = append(out, fence)
	return out
}

func BuildMarkdown(paths []string, opts Options) ([]string, error) {
	lines := []string{defaultHead, ""}

	if opts.IncludeEntryMetadata == false && opts.IncludeRawPayload == false && len(opts.PayloadTypes) == 0 {
		// keep as-is; explicit false is allowed. No-op guard not needed.
	}

	typeSet := map[string]bool{}
	for _, t := range opts.PayloadTypes {
		tt := strings.TrimSpace(t)
		if tt != "" {
			typeSet[tt] = true
		}
	}

	for _, p := range paths {
		p = filepath.Clean(p)
		meta, _ := sessions.ReadSessionMeta(p)
		updatedAt, _ := sessions.ConversationUpdatedAt(p)
		title, _ := sessions.ConversationTitle(p, sessions.DefaultSelfReflectionPrefix, 80)

		lines = append(lines, fmt.Sprintf("## %s", filepath.Base(p)))
		lines = append(lines, fmt.Sprintf("_Source: %s_", p))
		if meta.ID != "" || meta.Cwd != "" {
			lines = append(lines, "")
			if meta.ID != "" {
				lines = append(lines, fmt.Sprintf("- session_id: %s", meta.ID))
			}
			if meta.Cwd != "" {
				lines = append(lines, fmt.Sprintf("- project: %s", meta.ProjectName()))
			}
			if !meta.Timestamp.IsZero() {
				lines = append(lines, fmt.Sprintf("- started_at: %s", meta.Timestamp.UTC().Format("2006-01-02T15:04:05Z")))
			}
			if !updatedAt.IsZero() {
				lines = append(lines, fmt.Sprintf("- updated_at: %s", updatedAt.UTC().Format("2006-01-02T15:04:05Z")))
			}
			if strings.TrimSpace(title) != "" {
				lines = append(lines, fmt.Sprintf("- title: %s", title))
			}
		}
		lines = append(lines, "")

		entryIndex := 0
		err := sessions.WalkJSONLLines(p, func(line sessions.JSONLLine) error {
			if line.Type != "response_item" {
				return nil
			}

			var wrapper struct {
				Payload map[string]any `json:"payload"`
			}
			if err := json.Unmarshal(line.Raw, &wrapper); err != nil {
				return nil
			}
			payload := wrapper.Payload
			if payload == nil {
				return nil
			}
			payloadType, _ := payload["type"].(string)
			if len(typeSet) > 0 && !typeSet[payloadType] {
				return nil
			}

			entryIndex++
			style := "payload/unknown"
			if strings.TrimSpace(payloadType) != "" {
				style = "payload/" + payloadType
			}
			lines = append(lines, fmt.Sprintf("### Entry %d (%s)", entryIndex, style))

			if opts.IncludeEntryMetadata {
				lines = append(lines, fmt.Sprintf("- line_no: %d", line.LineNo))
				if strings.TrimSpace(line.Timestamp) != "" {
					lines = append(lines, fmt.Sprintf("- timestamp: %s", line.Timestamp))
				}
				if tn, ok := payload["tool_name"].(string); ok && strings.TrimSpace(tn) != "" {
					lines = append(lines, fmt.Sprintf("- tool_name: %s", tn))
				}
				lines = append(lines, "")
			}

			viewAny := any(buildPayloadView(payload))
			viewAny = truncateLists(viewAny, opts.MaxListLen)
			viewAny = truncateStrings(viewAny, opts.MaxStrLen)

			view, _ := viewAny.(map[string]any)
			texts, _ := view["text"].([]any)
			args, _ := view["arguments"].([]any)
			outputs, _ := view["output"].([]any)

			if len(texts) > 0 {
				lines = append(lines, "**text**")
				block := renderCodeFenceBlock(formatListLines(texts, false))
				lines = append(lines, block...)
			}
			if len(args) > 0 {
				lines = append(lines, "**arguments**")
				block := renderCodeFenceBlock(formatListLines(args, true))
				lines = append(lines, block...)
			}
			if len(outputs) > 0 {
				lines = append(lines, "**output**")
				block := renderCodeFenceBlock(formatListLines(outputs, false))
				lines = append(lines, block...)
			}

			if opts.IncludeRawPayload {
				lines = append(lines, "**payload**")
				payloadAny := truncateLists(any(payload), opts.MaxListLen)
				payloadAny = truncateStrings(payloadAny, opts.MaxStrLen)
				payloadLines := renderJSON(payloadAny, 0)
				block := renderCodeFenceBlock(payloadLines)
				lines = append(lines, block...)
			}

			if opts.EntriesPerFile > 0 && entryIndex >= opts.EntriesPerFile {
				return errStopWalk
			}
			return nil
		})
		if err != nil && err != errStopWalk {
			return nil, err
		}

		lines = append(lines, "")
	}

	return lines, nil
}
