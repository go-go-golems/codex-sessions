package reflect

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	presetfs "github.com/go-go-golems/codex-session/scripts/prompts"
)

type PromptPreset struct {
	Name        string
	Filename    string
	Description string
}

var promptPresets = []PromptPreset{
	{
		Name:        "reflection",
		Filename:    "reflection.md",
		Description: "Full reflection on repetition, friction, and skill ideas.",
	},
	{
		Name:        "summary",
		Filename:    "summary.md",
		Description: "Concise summary of goals, actions, outputs, and decisions.",
	},
	{
		Name:        "bloat",
		Filename:    "bloat.md",
		Description: "Identify bloat, dead ends, and cleanup opportunities.",
	},
	{
		Name:        "incomplete",
		Filename:    "incomplete.md",
		Description: "List unfinished work, open loops, and missing follow-ups.",
	},
	{
		Name:        "decisions",
		Filename:    "decisions.md",
		Description: "Capture key decisions, alternatives, and rationale.",
	},
	{
		Name:        "next_steps",
		Filename:    "next_steps.md",
		Description: "Concrete follow-up actions, tests, and validations.",
	},
}

func PromptPresetNames() []string {
	out := make([]string, 0, len(promptPresets))
	for _, p := range promptPresets {
		out = append(out, p.Name)
	}
	return out
}

func resolvePreset(name string) (PromptPreset, error) {
	for _, p := range promptPresets {
		if p.Name == name {
			return p, nil
		}
	}
	return PromptPreset{}, fmt.Errorf("unknown preset %q", name)
}

func loadEmbeddedText(filename string) (string, error) {
	b, err := presetfs.FS.ReadFile(filename)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		return "", fmt.Errorf("embedded prompt is empty: %s", filename)
	}
	return text, nil
}

func computePromptHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

type PromptSelection struct {
	PromptText string
	Source     string // preset|file|inline
	Preset     string // when Source==preset
	FilePath   string // when Source==file
}

func (ps PromptSelection) PromptLabel(promptHash string) string {
	if ps.Source == "file" && ps.FilePath != "" {
		return ps.FilePath
	}
	if ps.Source == "preset" && ps.Preset != "" {
		return "preset:" + ps.Preset
	}
	return "inline:" + promptHash[:8]
}

func (ps PromptSelection) CacheKey(promptHash string) string {
	sum := sha256.Sum256([]byte(ps.PromptLabel(promptHash)))
	return hex.EncodeToString(sum[:])[:12]
}

func ResolvePromptSelection(promptFile string, promptPreset string, promptText string) (PromptSelection, error) {
	selected := 0
	if strings.TrimSpace(promptFile) != "" {
		selected++
	}
	if strings.TrimSpace(promptPreset) != "" {
		selected++
	}
	if strings.TrimSpace(promptText) != "" {
		selected++
	}
	if selected > 1 {
		return PromptSelection{}, fmt.Errorf("use only one of --prompt-file, --prompt-preset, or --prompt-text")
	}

	if strings.TrimSpace(promptText) != "" {
		cleaned := strings.TrimSpace(promptText)
		return PromptSelection{PromptText: cleaned, Source: "inline"}, nil
	}

	if strings.TrimSpace(promptFile) == "" && strings.TrimSpace(promptPreset) == "" {
		promptPreset = DefaultPromptPreset
	}

	if strings.TrimSpace(promptPreset) != "" {
		p, err := resolvePreset(promptPreset)
		if err != nil {
			return PromptSelection{}, err
		}
		text, err := loadEmbeddedText(p.Filename)
		if err != nil {
			return PromptSelection{}, err
		}
		return PromptSelection{PromptText: text, Source: "preset", Preset: p.Name}, nil
	}

	resolved := filepath.Clean(os.ExpandEnv(promptFile))
	b, err := os.ReadFile(resolved)
	if err != nil {
		return PromptSelection{}, err
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		return PromptSelection{}, fmt.Errorf("prompt file is empty: %s", resolved)
	}
	return PromptSelection{PromptText: text, Source: "file", FilePath: resolved}, nil
}

type PromptVersionState struct {
	PromptVersion string `json:"prompt_version"`
	PromptHash    string `json:"prompt_hash"`
	UpdatedAt     string `json:"updated_at"`
}

func nextPromptVersion(previous string, dateStr string) string {
	prefix := dateStr + "-v"
	if strings.HasPrefix(previous, prefix) {
		suffix := strings.TrimPrefix(previous, prefix)
		if n, err := strconv.Atoi(suffix); err == nil {
			return fmt.Sprintf("%s-v%d", dateStr, n+1)
		}
	}
	return dateStr + "-v1"
}

func loadPromptVersionState(path string) (PromptVersionState, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return PromptVersionState{}, err
	}
	var st PromptVersionState
	if err := json.Unmarshal(b, &st); err != nil {
		return PromptVersionState{}, err
	}
	return st, nil
}

func writePromptVersionState(path string, st PromptVersionState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func (ps PromptSelection) VersionStatePath(cacheDir string, promptHash string) string {
	cachePromptVersions := filepath.Join(cacheDir, "prompt_versions")
	switch ps.Source {
	case "preset":
		return filepath.Join(cachePromptVersions, "preset_"+ps.Preset+"_version.json")
	case "file":
		verPath := strings.TrimSuffix(ps.FilePath, filepath.Ext(ps.FilePath)) + "_version.json"
		return verPath
	default:
		return filepath.Join(cachePromptVersions, "inline_"+promptHash+".json")
	}
}

func EnsurePromptVersionState(selection PromptSelection, cacheDir string, now time.Time) (PromptVersionState, string, error) {
	promptHash := computePromptHash(selection.PromptText)
	statePath := selection.VersionStatePath(cacheDir, promptHash)

	var st PromptVersionState
	stLoaded := false
	if b, err := os.ReadFile(statePath); err == nil {
		if err := json.Unmarshal(b, &st); err == nil {
			stLoaded = true
		}
	}

	// For presets, if state file doesn't exist yet, seed from embedded *_version.json when available.
	if selection.Source == "preset" && !stLoaded {
		p, err := resolvePreset(selection.Preset)
		if err != nil {
			return PromptVersionState{}, "", err
		}
		seedPath := strings.TrimSuffix(p.Filename, ".md") + "_version.json"
		if b, err := presetfs.FS.ReadFile(seedPath); err == nil {
			_ = json.Unmarshal(b, &st)
			stLoaded = st.PromptVersion != "" && st.PromptHash != ""
		}
	}

	if st.PromptVersion == "" || st.PromptHash != promptHash {
		dateStr := now.UTC().Format("2006-01-02")
		st.PromptVersion = nextPromptVersion(st.PromptVersion, dateStr)
		st.PromptHash = promptHash
		st.UpdatedAt = now.UTC().Format(time.RFC3339Nano)

		// If writing adjacent to a file prompt fails, fall back to cache_dir/prompt_versions.
		if err := writePromptVersionState(statePath, st); err != nil && selection.Source == "file" {
			fallback := filepath.Join(cacheDir, "prompt_versions", "file_"+promptHash+".json")
			if err2 := writePromptVersionState(fallback, st); err2 != nil {
				return PromptVersionState{}, "", err2
			}
			statePath = fallback
		} else if err != nil {
			return PromptVersionState{}, "", err
		}
	}

	return st, promptHash, nil
}
