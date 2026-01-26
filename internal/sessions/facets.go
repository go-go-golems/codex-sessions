package sessions

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

type TextField struct {
	Timestamp time.Time
	Source    string // json_path-ish
	Text      string
}

type ToolCall struct {
	Timestamp time.Time
	Name      string
	Arguments string // JSON-ish string
	CallID    string
}

type ToolOutput struct {
	Timestamp time.Time
	Name      string
	Output    string // JSON-ish string
	CallID    string
}

type PathMention struct {
	Timestamp time.Time
	Path      string
	Source    string // message|tool_arguments|tool_output
	Role      string // user|assistant|unknown
}

type ErrorSignal struct {
	Timestamp time.Time
	Kind      string // exit_code|stderr|panic|traceback|error_text
	Snippet   string
	Source    string // message|tool
}

type Facets struct {
	Texts       []TextField
	ToolCalls   []ToolCall
	ToolOutputs []ToolOutput
	Paths       []PathMention
	Errors      []ErrorSignal
}

type FacetOptions struct {
	MaxValueChars int
	MaxPaths      int
	MaxErrors     int
}

func DefaultFacetOptions() FacetOptions {
	return FacetOptions{
		MaxValueChars: 2000,
		MaxPaths:      2000,
		MaxErrors:     2000,
	}
}

func truncateValue(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return s[:maxLen-1] + "…"
}

func normalizeTimestamp(ts string) (time.Time, bool) {
	if ts == "" {
		return time.Time{}, false
	}
	t, err := parseTimestamp(ts)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func collectTextFields(value any, path []string, out *[]TextField, ts time.Time, maxValueChars int) {
	switch v := value.(type) {
	case map[string]any:
		for k, val := range v {
			childPath := append(path, k)
			if k == "text" {
				if s, ok := val.(string); ok {
					*out = append(*out, TextField{Timestamp: ts, Source: strings.Join(childPath, "."), Text: truncateValue(s, maxValueChars)})
				}
			}
			collectTextFields(val, childPath, out, ts, maxValueChars)
		}
	case []any:
		for i, item := range v {
			childPath := append(path, "["+strconv.Itoa(i)+"]")
			collectTextFields(item, childPath, out, ts, maxValueChars)
		}
	}
}

func maybeString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok && s != ""
}

func toolNameForObject(m map[string]any) string {
	// Tighten heuristics to avoid treating arbitrary objects with a "name" field
	// as tool invocations.
	if s, ok := maybeString(m["tool_name"]); ok {
		return s
	}

	typ, _ := m["type"].(string)
	switch typ {
	case "tool_call", "tool_result", "tool_output", "tool":
		// allow name-based fallbacks only for objects that explicitly look tool-ish
		if s, ok := maybeString(m["name"]); ok {
			return s
		}
		if fn, ok := m["function"].(map[string]any); ok {
			if s, ok := maybeString(fn["name"]); ok {
				return s
			}
		}
	}
	return ""
}

func marshalCompact(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func collectToolCallsAndOutputs(value any, outCalls *[]ToolCall, outOutputs *[]ToolOutput, ts time.Time, maxValueChars int) {
	switch v := value.(type) {
	case map[string]any:
		name := toolNameForObject(v)
		if name != "" {
			if args, ok := v["arguments"]; ok {
				argsStr := marshalCompact(args)
				if argsStr == "" {
					if s, ok := args.(string); ok {
						argsStr = s
					}
				}
				if argsStr != "" {
					*outCalls = append(*outCalls, ToolCall{Timestamp: ts, Name: name, Arguments: truncateValue(argsStr, maxValueChars)})
				}
			}
			if out, ok := v["output"]; ok {
				outStr := marshalCompact(out)
				if outStr == "" {
					if s, ok := out.(string); ok {
						outStr = s
					}
				}
				if outStr != "" {
					*outOutputs = append(*outOutputs, ToolOutput{Timestamp: ts, Name: name, Output: truncateValue(outStr, maxValueChars)})
				}
			}
		}

		for _, val := range v {
			collectToolCallsAndOutputs(val, outCalls, outOutputs, ts, maxValueChars)
		}
	case []any:
		for _, item := range v {
			collectToolCallsAndOutputs(item, outCalls, outOutputs, ts, maxValueChars)
		}
	}
}

func extractToolFromResponseItemPayload(
	payload map[string]any,
	ts time.Time,
	outCalls *[]ToolCall,
	outOutputs *[]ToolOutput,
	callIDToName map[string]string,
	maxValueChars int,
) bool {
	payloadType, _ := payload["type"].(string)
	switch payloadType {
	case "custom_tool_call":
		name, ok := maybeString(payload["name"])
		if !ok {
			return false
		}
		// Codex sessions typically store args as a string under "input".
		callID, _ := maybeString(payload["call_id"])
		if input, ok := payload["input"]; ok {
			inStr := marshalCompact(input)
			if inStr == "" {
				if s, ok := input.(string); ok {
					inStr = s
				}
			}
			if inStr != "" {
				*outCalls = append(*outCalls, ToolCall{Timestamp: ts, Name: name, Arguments: truncateValue(inStr, maxValueChars), CallID: callID})
			}
		}
		if callID != "" {
			callIDToName[callID] = name
		}
		return true
	case "custom_tool_call_output":
		// Outputs are linked to calls via call_id; the payload doesn't always include the tool name.
		name := ""
		callID, _ := maybeString(payload["call_id"])
		if callID != "" {
			name = callIDToName[callID]
		}
		if name == "" {
			name = "unknown"
		}
		if out, ok := payload["output"]; ok {
			outStr := marshalCompact(out)
			if outStr == "" {
				if s, ok := out.(string); ok {
					outStr = s
				}
			}
			if outStr != "" {
				*outOutputs = append(*outOutputs, ToolOutput{Timestamp: ts, Name: name, Output: truncateValue(outStr, maxValueChars), CallID: callID})
			}
		}
		return true
	case "tool_call":
		name := ""
		if s, ok := maybeString(payload["tool_name"]); ok {
			name = s
		}
		if name == "" {
			return false
		}
		callID, _ := maybeString(payload["call_id"])
		if args, ok := payload["arguments"]; ok {
			argsStr := marshalCompact(args)
			if argsStr == "" {
				if s, ok := args.(string); ok {
					argsStr = s
				}
			}
			if argsStr != "" {
				*outCalls = append(*outCalls, ToolCall{Timestamp: ts, Name: name, Arguments: truncateValue(argsStr, maxValueChars), CallID: callID})
			}
		}
		if callID != "" {
			callIDToName[callID] = name
		}
		return true
	case "tool_result", "tool_output":
		name := ""
		if s, ok := maybeString(payload["tool_name"]); ok {
			name = s
		}
		if name == "" {
			if callID, ok := maybeString(payload["call_id"]); ok {
				name = callIDToName[callID]
			}
		}
		if name == "" {
			name = "unknown"
		}
		callID, _ := maybeString(payload["call_id"])
		if out, ok := payload["output"]; ok {
			outStr := marshalCompact(out)
			if outStr == "" {
				if s, ok := out.(string); ok {
					outStr = s
				}
			}
			if outStr != "" {
				*outOutputs = append(*outOutputs, ToolOutput{Timestamp: ts, Name: name, Output: truncateValue(outStr, maxValueChars), CallID: callID})
			}
		}
		return true
	default:
		return false
	}
}

func extractPathsFromMessages(msgs []Message, maxValueChars int) []PathMention {
	seen := map[string]bool{}
	var out []PathMention
	for _, m := range msgs {
		for _, p := range FindPathMentions(m.Text) {
			key := m.Role + "|" + p
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, PathMention{Timestamp: m.Timestamp, Path: truncateValue(p, maxValueChars), Source: "message", Role: m.Role})
		}
	}
	return out
}

func extractErrorsFromMessages(msgs []Message, maxValueChars int) []ErrorSignal {
	var out []ErrorSignal
	for _, m := range msgs {
		for _, sig := range FindErrorSignals(m.Text) {
			out = append(out, ErrorSignal{Timestamp: m.Timestamp, Kind: sig.Kind, Snippet: truncateValue(sig.Snippet, maxValueChars), Source: "message"})
		}
	}
	return out
}

func extractPathsFromToolStrings(calls []ToolCall, outs []ToolOutput, maxValueChars int, limit int) []PathMention {
	seen := map[string]bool{}
	var out []PathMention
	add := func(ts time.Time, src string, text string) {
		for _, p := range FindPathMentions(text) {
			key := src + "|" + p
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, PathMention{Timestamp: ts, Path: truncateValue(p, maxValueChars), Source: src, Role: "unknown"})
			if limit > 0 && len(out) >= limit {
				return
			}
		}
	}
	for _, c := range calls {
		add(c.Timestamp, "tool_arguments", c.Arguments)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	for _, o := range outs {
		add(o.Timestamp, "tool_output", o.Output)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func extractErrorsFromToolStrings(outs []ToolOutput, maxValueChars int, limit int) []ErrorSignal {
	var out []ErrorSignal
	for _, o := range outs {
		for _, sig := range FindErrorSignals(o.Output) {
			out = append(out, ErrorSignal{Timestamp: o.Timestamp, Kind: sig.Kind, Snippet: truncateValue(sig.Snippet, maxValueChars), Source: "tool"})
			if limit > 0 && len(out) >= limit {
				return out
			}
		}
	}
	return out
}

// ExtractFacets parses a session JSONL file and derives queryable facets.
//
// This is best-effort: unknown shapes are tolerated, and extraction is heuristic.
func ExtractFacets(path string, opts FacetOptions) (*Facets, error) {
	if opts.MaxValueChars <= 0 {
		opts.MaxValueChars = DefaultFacetOptions().MaxValueChars
	}
	if opts.MaxPaths <= 0 {
		opts.MaxPaths = DefaultFacetOptions().MaxPaths
	}
	if opts.MaxErrors <= 0 {
		opts.MaxErrors = DefaultFacetOptions().MaxErrors
	}

	// Message-derived facets
	msgs, err := ExtractMessages(path)
	if err != nil {
		return nil, err
	}

	facets := &Facets{}
	facets.Paths = append(facets.Paths, extractPathsFromMessages(msgs, opts.MaxValueChars)...)
	facets.Errors = append(facets.Errors, extractErrorsFromMessages(msgs, opts.MaxValueChars)...)

	// JSON-derived facets
	var texts []TextField
	var calls []ToolCall
	var outs []ToolOutput
	callIDToName := map[string]string{}
	err = WalkJSONLLines(path, func(line JSONLLine) error {
		ts, ok := normalizeTimestamp(line.Timestamp)
		if !ok {
			// If no timestamp, use zero (still stable ordering by append order).
			ts = time.Time{}
		}
		var decoded any
		if err := json.Unmarshal(line.Raw, &decoded); err != nil {
			return nil
		}
		collectTextFields(decoded, []string{}, &texts, ts, opts.MaxValueChars)
		extractedTool := false
		if top, ok := decoded.(map[string]any); ok {
			if topType, _ := top["type"].(string); topType == "response_item" {
				if payload, ok := top["payload"].(map[string]any); ok {
					extractedTool = extractToolFromResponseItemPayload(payload, ts, &calls, &outs, callIDToName, opts.MaxValueChars)
				}
			}
		}
		if !extractedTool {
			collectToolCallsAndOutputs(decoded, &calls, &outs, ts, opts.MaxValueChars)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	facets.Texts = texts
	facets.ToolCalls = calls
	facets.ToolOutputs = outs
	facets.Paths = append(facets.Paths, extractPathsFromToolStrings(calls, outs, opts.MaxValueChars, opts.MaxPaths)...)
	facets.Errors = append(facets.Errors, extractErrorsFromToolStrings(outs, opts.MaxValueChars, opts.MaxErrors)...)

	// Sort for stable output
	sort.Slice(facets.Paths, func(i, j int) bool {
		if facets.Paths[i].Timestamp.Equal(facets.Paths[j].Timestamp) {
			return facets.Paths[i].Path < facets.Paths[j].Path
		}
		return facets.Paths[i].Timestamp.Before(facets.Paths[j].Timestamp)
	})
	sort.Slice(facets.Errors, func(i, j int) bool {
		if facets.Errors[i].Timestamp.Equal(facets.Errors[j].Timestamp) {
			return facets.Errors[i].Kind < facets.Errors[j].Kind
		}
		return facets.Errors[i].Timestamp.Before(facets.Errors[j].Timestamp)
	})
	return facets, nil
}
