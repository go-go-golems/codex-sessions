package reflect

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

func newestByMtime(paths []string) string {
	type pair struct {
		path string
		mt   time.Time
	}
	var ps []pair
	for _, p := range paths {
		if !isExecutable(p) {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		ps = append(ps, pair{path: p, mt: info.ModTime()})
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].mt.After(ps[j].mt) })
	if len(ps) == 0 {
		return ""
	}
	return ps[0].path
}

func ResolveCodexPath(override string) (string, error) {
	if override != "" {
		p := filepath.Clean(os.ExpandEnv(override))
		if !isExecutable(p) {
			return "", fmt.Errorf("codex not found or not executable: %s", p)
		}
		return p, nil
	}

	if env := os.Getenv("CODEX_BIN"); env != "" {
		p := filepath.Clean(os.ExpandEnv(env))
		if !isExecutable(p) {
			return "", fmt.Errorf("CODEX_BIN not found or not executable: %s", p)
		}
		return p, nil
	}

	if p, err := exec.LookPath("codex"); err == nil && isExecutable(p) {
		return p, nil
	}

	var candidates []string
	for _, base := range []string{
		filepath.Join(os.Getenv("HOME"), ".vscode", "extensions"),
		filepath.Join(os.Getenv("HOME"), ".vscode-insiders", "extensions"),
	} {
		glob := filepath.Join(base, "openai.chatgpt-*", "bin", "*", "codex")
		matches, _ := filepath.Glob(glob)
		candidates = append(candidates, matches...)
	}
	if newest := newestByMtime(candidates); newest != "" {
		return newest, nil
	}

	for _, p := range []string{"/opt/homebrew/bin/codex", "/usr/local/bin/codex"} {
		if isExecutable(p) {
			return p, nil
		}
	}

	return "", fmt.Errorf("codex not found in PATH; use --codex-path or set CODEX_BIN")
}

func RunCodexReflection(
	ctx context.Context,
	codexBin string,
	sessionID string,
	prompt string,
	sandbox string,
	approval string,
	timeoutSeconds int,
	debug bool,
) error {
	if sandbox == "" {
		sandbox = DefaultCodexSandbox
	}
	if approval == "" {
		approval = DefaultCodexApproval
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = DefaultCodexTimeoutSecs
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	args := []string{
		"--sandbox", sandbox,
		"--ask-for-approval", approval,
		"exec",
		"--skip-git-repo-check",
		"resume",
		sessionID,
		"-",
	}

	cmd := exec.CommandContext(ctx, codexBin, args...)
	cmd.Stdin = bytes.NewBufferString(strings.TrimSpace(prompt) + "\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if debug {
		if stdout.Len() > 0 {
			_, _ = os.Stderr.Write(stdout.Bytes())
		}
		if stderr.Len() > 0 {
			_, _ = os.Stderr.Write(stderr.Bytes())
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("codex timed out after %ds", timeoutSeconds)
	}
	if err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("codex failed: %s", strings.TrimSpace(stderr.String()))
		}
		return fmt.Errorf("codex failed: %v", err)
	}
	return nil
}
