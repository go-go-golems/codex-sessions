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
}

type ToolOutput struct {
	Timestamp time.Time
	Name      string
	Output    string // JSON-ish string
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

func truncateValue(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
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

func collectTextFields(value any, path []string, out *[]TextField, ts time.Time, max int) {
	switch v := value.(type) {
	case map[string]any:
		for k, val := range v {
			childPath := append(path, k)
			if k == "text" {
				if s, ok := val.(string); ok {
					*out = append(*out, TextField{Timestamp: ts, Source: strings.Join(childPath, "."), Text: truncateValue(s, max)})
				}
			}
			collectTextFields(val, childPath, out, ts, max)
		}
	case []any:
		for i, item := range v {
			childPath := append(path, "["+strconv.Itoa(i)+"]")
			collectTextFields(item, childPath, out, ts, max)
		}
	}
}

func maybeString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok && s != ""
}

func maybeInt(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case int:
		return x, true
	case int64:
		return int(x), true
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	}
	return 0, false
}

func findToolName(m map[string]any) string {
	if s, ok := maybeString(m["tool_name"]); ok {
		return s
	}
	if s, ok := maybeString(m["name"]); ok {
		return s
	}
	if fn, ok := m["function"].(map[string]any); ok {
		if s, ok := maybeString(fn["name"]); ok {
			return s
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

func collectToolCallsAndOutputs(value any, outCalls *[]ToolCall, outOutputs *[]ToolOutput, ts time.Time, max int) {
	switch v := value.(type) {
	case map[string]any:
		name := findToolName(v)
		if name != "" {
			if args, ok := v["arguments"]; ok {
				argsStr := marshalCompact(args)
				if argsStr == "" {
					if s, ok := args.(string); ok {
						argsStr = s
					}
				}
				if argsStr != "" {
					*outCalls = append(*outCalls, ToolCall{Timestamp: ts, Name: name, Arguments: truncateValue(argsStr, max)})
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
					*outOutputs = append(*outOutputs, ToolOutput{Timestamp: ts, Name: name, Output: truncateValue(outStr, max)})
				}
			}
		}

		for _, val := range v {
			collectToolCallsAndOutputs(val, outCalls, outOutputs, ts, max)
		}
	case []any:
		for _, item := range v {
			collectToolCallsAndOutputs(item, outCalls, outOutputs, ts, max)
		}
	}
}

func extractPathsFromMessages(msgs []Message, max int) []PathMention {
	seen := map[string]bool{}
	var out []PathMention
	for _, m := range msgs {
		for _, p := range FindPathMentions(m.Text) {
			key := m.Role + "|" + p
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, PathMention{Timestamp: m.Timestamp, Path: truncateValue(p, max), Source: "message", Role: m.Role})
		}
	}
	return out
}

func extractErrorsFromMessages(msgs []Message, max int) []ErrorSignal {
	var out []ErrorSignal
	for _, m := range msgs {
		for _, sig := range FindErrorSignals(m.Text) {
			out = append(out, ErrorSignal{Timestamp: m.Timestamp, Kind: sig.Kind, Snippet: truncateValue(sig.Snippet, max), Source: "message"})
		}
	}
	return out
}

func extractPathsFromToolStrings(calls []ToolCall, outs []ToolOutput, max int, limit int) []PathMention {
	seen := map[string]bool{}
	var out []PathMention
	add := func(ts time.Time, src string, text string) {
		for _, p := range FindPathMentions(text) {
			key := src + "|" + p
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, PathMention{Timestamp: ts, Path: truncateValue(p, max), Source: src, Role: "unknown"})
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

func extractErrorsFromToolStrings(outs []ToolOutput, max int, limit int) []ErrorSignal {
	var out []ErrorSignal
	for _, o := range outs {
		for _, sig := range FindErrorSignals(o.Output) {
			out = append(out, ErrorSignal{Timestamp: o.Timestamp, Kind: sig.Kind, Snippet: truncateValue(sig.Snippet, max), Source: "tool"})
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
		collectToolCallsAndOutputs(decoded, &calls, &outs, ts, opts.MaxValueChars)
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
