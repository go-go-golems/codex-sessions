package sessions

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type DetectedError struct {
	Kind    string
	Snippet string
}

var (
	reAbsPath = regexp.MustCompile(`/(?:[A-Za-z0-9._-]+/)+[A-Za-z0-9._-]+`)
	reRelPath = regexp.MustCompile(`\b(?:[A-Za-z0-9._-]+/)+[A-Za-z0-9._-]+\b`)
	reFileExt = regexp.MustCompile(`\b[A-Za-z0-9._-]+\.(?:go|py|md|jsonl|json|yaml|yml|ts|js|txt|sh)\b`)

	reExitCode   = regexp.MustCompile(`(?i)\bexit\s*code\s*[:= ]\s*(\d+)\b`)
	reReturnCode = regexp.MustCompile(`(?i)\breturn\s*code\s*[:= ]\s*(\d+)\b`)

	reJSONExitCode = regexp.MustCompile(`"exit_code"\s*:\s*(\d+)`)
	reJSONReturn   = regexp.MustCompile(`"returncode"\s*:\s*(\d+)`)
)

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// FindPathMentions returns a best-effort list of path-like strings in text.
func FindPathMentions(text string) []string {
	matches := []string{}
	matches = append(matches, reAbsPath.FindAllString(text, -1)...)
	matches = append(matches, reRelPath.FindAllString(text, -1)...)
	matches = append(matches, reFileExt.FindAllString(text, -1)...)
	return uniqueSorted(matches)
}

func lineSnippets(text string, maxLines int) []string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, maxLines)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out) >= maxLines {
			break
		}
	}
	return out
}

func parseNonZeroInt(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	if n == 0 {
		return 0, false
	}
	return n, true
}

// FindErrorSignals returns heuristic error indicators from text.
func FindErrorSignals(text string) []DetectedError {
	lower := strings.ToLower(text)
	seen := map[string]bool{}
	out := []DetectedError{}

	add := func(kind string, snippet string) {
		if kind == "" || snippet == "" {
			return
		}
		if seen[kind+"|"+snippet] {
			return
		}
		seen[kind+"|"+snippet] = true
		out = append(out, DetectedError{Kind: kind, Snippet: snippet})
	}

	// Structured-ish signals
	for _, m := range reExitCode.FindAllStringSubmatch(text, -1) {
		if n, ok := parseNonZeroInt(m[1]); ok {
			add("exit_code", "exit code "+strconv.Itoa(n))
		}
	}
	for _, m := range reReturnCode.FindAllStringSubmatch(text, -1) {
		if n, ok := parseNonZeroInt(m[1]); ok {
			add("return_code", "return code "+strconv.Itoa(n))
		}
	}
	for _, m := range reJSONExitCode.FindAllStringSubmatch(text, -1) {
		if n, ok := parseNonZeroInt(m[1]); ok {
			add("exit_code", "exit_code "+strconv.Itoa(n))
		}
	}
	for _, m := range reJSONReturn.FindAllStringSubmatch(text, -1) {
		if n, ok := parseNonZeroInt(m[1]); ok {
			add("return_code", "returncode "+strconv.Itoa(n))
		}
	}

	// Textual signals (use first few non-empty lines as context)
	lines := lineSnippets(text, 50)
	for _, line := range lines {
		l := strings.ToLower(line)
		switch {
		case strings.Contains(l, "panic:"):
			add("panic", line)
		case strings.Contains(l, "traceback"):
			add("traceback", line)
		case strings.Contains(l, "exception"):
			add("exception", line)
		case strings.Contains(l, "fatal"):
			add("fatal", line)
		case strings.Contains(l, "error") && (strings.Contains(l, "error:") || strings.HasPrefix(l, "error")):
			add("error_text", line)
		case strings.Contains(l, "command not found"):
			add("command_not_found", line)
		case strings.Contains(l, "permission denied"):
			add("permission_denied", line)
		case strings.Contains(l, "no such file"):
			add("missing_file", line)
		}
		if len(out) >= 5 {
			break
		}
	}

	// Fallback: if the whole blob contains certain keywords but no line was caught.
	if len(out) == 0 {
		if strings.Contains(lower, "traceback") {
			add("traceback", "traceback")
		}
		if strings.Contains(lower, "panic") {
			add("panic", "panic")
		}
		if strings.Contains(lower, "error") {
			add("error_text", "error")
		}
	}

	return out
}
