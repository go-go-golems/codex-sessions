package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	reflectcli "codex-reflect-skill/internal/reflect"
	"codex-reflect-skill/internal/sessions"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type ReflectSettings struct {
	SessionsRoot      string `glazed.parameter:"sessions-root"`
	CacheDir          string `glazed.parameter:"cache-dir"`
	Project           string `glazed.parameter:"project"`
	Since             string `glazed.parameter:"since"`
	Until             string `glazed.parameter:"until"`
	SessionID         string `glazed.parameter:"session-id"`
	SessionIDs        string `glazed.parameter:"session-ids"`
	Limit             int    `glazed.parameter:"limit"`
	IncludeMostRecent bool   `glazed.parameter:"include-most-recent"`

	Prefix       string `glazed.parameter:"prefix"`
	PromptPreset string `glazed.parameter:"prompt-preset"`
	PromptFile   string `glazed.parameter:"prompt-file"`
	PromptText   string `glazed.parameter:"prompt-text"`
	RefreshMode  string `glazed.parameter:"refresh-mode"`

	CodexSandbox        string `glazed.parameter:"codex-sandbox"`
	CodexApproval       string `glazed.parameter:"codex-approval"`
	CodexTimeoutSeconds int    `glazed.parameter:"codex-timeout-seconds"`
	CodexPath           string `glazed.parameter:"codex-path"`
	Debug               bool   `glazed.parameter:"debug"`

	ExtraMetadata bool `glazed.parameter:"extra-metadata"`
	DryRun        bool `glazed.parameter:"dry-run"`
}

type ReflectCommand struct {
	*cmds.CommandDescription
}

func NewReflectCommand() (*ReflectCommand, error) {
	desc := cmds.NewCommandDescription(
		"reflect",
		cmds.WithShort("Generate cached self-reflections for sessions"),
		cmds.WithLong(`Generate a reflection for each selected session, caching the result.

This mirrors the Python tool's behavior:
- Create a temporary copy of the session JSONL with a new id
- Prefix the first user message with the self-reflection prefix
- Run "codex exec resume <copy_id> -" with the reflection prompt on stdin
- Extract the last assistant message as the reflection
- Delete the temporary copy
- Cache to <sessions-root>/reflection_cache/<session_id>-<prompt_key>.json

Use --dry-run to compute cache status and selection without invoking Codex.
`),
		cmds.WithFlags(
			fields.New(
				"sessions-root",
				fields.TypeString,
				fields.WithDefault(defaultSessionsRoot()),
				fields.WithHelp("Root directory containing Codex session JSONL files"),
			),
			fields.New(
				"cache-dir",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Cache directory (default: <sessions-root>/reflection_cache)"),
			),
			fields.New(
				"project",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only include sessions matching this derived project label"),
			),
			fields.New(
				"since",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only include sessions on/after this ISO date or datetime"),
			),
			fields.New(
				"until",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only include sessions on/before this ISO date or datetime"),
			),
			fields.New(
				"session-id",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Specific session id to reflect (overrides filters/limit)"),
			),
			fields.New(
				"session-ids",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Comma-separated session ids to reflect (overrides filters/limit)"),
			),
			fields.New(
				"limit",
				fields.TypeInteger,
				fields.WithDefault(10),
				fields.WithHelp("Limit to the most recent N sessions after filtering"),
			),
			fields.New(
				"include-most-recent",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Include the most recent session (skipped by default)"),
			),
			fields.New(
				"prefix",
				fields.TypeString,
				fields.WithDefault(reflectcli.DefaultPrefix),
				fields.WithHelp("Prefix inserted into the duplicated session's first user message"),
			),
			fields.New(
				"prompt-preset",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Prompt preset name (default: reflection). Presets: reflection, summary, bloat, incomplete, decisions, next_steps"),
			),
			fields.New(
				"prompt-file",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Path to a custom prompt file (mutually exclusive with preset/text)"),
			),
			fields.New(
				"prompt-text",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Inline prompt text (mutually exclusive with preset/file)"),
			),
			fields.New(
				"refresh-mode",
				fields.TypeChoice,
				fields.WithDefault("never"),
				fields.WithChoices("never", "auto", "always"),
				fields.WithHelp("Cache reuse policy (never/auto/always)"),
			),
			fields.New(
				"codex-sandbox",
				fields.TypeString,
				fields.WithDefault(reflectcli.DefaultCodexSandbox),
				fields.WithHelp("Sandbox mode passed to codex"),
			),
			fields.New(
				"codex-approval",
				fields.TypeString,
				fields.WithDefault(reflectcli.DefaultCodexApproval),
				fields.WithHelp("Approval policy passed to codex"),
			),
			fields.New(
				"codex-timeout-seconds",
				fields.TypeInteger,
				fields.WithDefault(reflectcli.DefaultCodexTimeoutSecs),
				fields.WithHelp("Timeout per codex call (seconds)"),
			),
			fields.New(
				"codex-path",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Override codex binary path (or set CODEX_BIN)"),
			),
			fields.New(
				"debug",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Print codex stdout/stderr to stderr"),
			),
			fields.New(
				"extra-metadata",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Include cache/prompt metadata columns in output"),
			),
			fields.New(
				"dry-run",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Do not invoke codex; only compute selection + cache status"),
			),
		),
	)
	return &ReflectCommand{CommandDescription: desc}, nil
}

var _ cmds.GlazeCommand = &ReflectCommand{}

func parseCSVIDs(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (c *ReflectCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &ReflectSettings{}
	if err := values.DecodeSectionInto(vals, schema.DefaultSlug, settings); err != nil {
		return errors.Wrap(err, "failed to decode settings")
	}

	cacheDir, err := reflectcli.EnsureCacheDir(settings.SessionsRoot, settings.CacheDir)
	if err != nil {
		return err
	}

	selection, err := reflectcli.ResolvePromptSelection(settings.PromptFile, settings.PromptPreset, settings.PromptText)
	if err != nil {
		return err
	}
	promptState, promptHash, err := reflectcli.EnsurePromptVersionState(selection, cacheDir, time.Now().UTC())
	if err != nil {
		return err
	}
	promptUpdatedAt, _ := reflectcli.ParseTimestampBestEffort(promptState.UpdatedAt)
	promptCacheKey := selection.CacheKey(promptHash)

	// Resolve selection set
	var metas []sessions.SessionMeta
	explicitIDs := []string{}
	if strings.TrimSpace(settings.SessionID) != "" {
		explicitIDs = append(explicitIDs, strings.TrimSpace(settings.SessionID))
	}
	explicitIDs = append(explicitIDs, parseCSVIDs(settings.SessionIDs)...)

	if len(explicitIDs) > 0 {
		seen := map[string]bool{}
		for _, id := range explicitIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			meta, err := sessions.FindSessionByID(settings.SessionsRoot, id)
			if err != nil {
				return err
			}
			metas = append(metas, meta)
		}
	} else {
		var since *time.Time
		if settings.Since != "" {
			parsed, err := sessions.ParseDateOrDateTime(settings.Since)
			if err != nil {
				return errors.Wrap(err, "invalid --since")
			}
			since = &parsed
		}
		var until *time.Time
		if settings.Until != "" {
			parsed, err := sessions.ParseDateOrDateTime(settings.Until)
			if err != nil {
				return errors.Wrap(err, "invalid --until")
			}
			until = &parsed
		}

		paths, err := sessions.DiscoverRolloutFiles(settings.SessionsRoot)
		if err != nil {
			return err
		}
		metas = make([]sessions.SessionMeta, 0, len(paths))
		for _, p := range paths {
			meta, err := sessions.ReadSessionMeta(p)
			if err != nil {
				continue
			}
			if settings.Project != "" && meta.ProjectName() != settings.Project {
				continue
			}
			if since != nil && meta.Timestamp.Before(*since) {
				continue
			}
			if until != nil && meta.Timestamp.After(*until) {
				continue
			}
			metas = append(metas, meta)
		}

		sort.Slice(metas, func(i, j int) bool { return metas[i].Timestamp.Before(metas[j].Timestamp) })
		if len(metas) == 0 {
			return nil
		}

		if !settings.IncludeMostRecent {
			newest := metas[len(metas)-1].Timestamp
			filtered := metas[:0]
			for _, m := range metas {
				if !m.Timestamp.Equal(newest) {
					filtered = append(filtered, m)
				}
			}
			metas = filtered
		}

		if settings.Limit > 0 && len(metas) > settings.Limit {
			metas = metas[len(metas)-settings.Limit:]
		}
	}

	codexBin := ""
	if !settings.DryRun {
		codexBin, err = reflectcli.ResolveCodexPath(settings.CodexPath)
		if err != nil {
			return err
		}
	}

	for _, meta := range metas {
		conv, convErr := reflectcli.BuildConversationInfo(meta)

		cachePath := reflectcli.CachePath(cacheDir, meta.ID, promptCacheKey)
		entryPath := cachePath
		var entry *reflectcli.CacheEntry

		if _, err := os.Stat(cachePath); err == nil {
			e, err := reflectcli.LoadCacheEntry(cachePath)
			if err == nil {
				entry = e
			}
		} else if selection.Source == "preset" && selection.Preset == reflectcli.DefaultPromptPreset {
			legacy := reflectcli.LegacyCachePath(cacheDir, meta.ID)
			if _, err := os.Stat(legacy); err == nil {
				e, err := reflectcli.LoadCacheEntry(legacy)
				if err == nil {
					entry = e
					entryPath = legacy
				}
			}
		}

		decision := reflectcli.AssessCacheDecision(entry, conv.UpdatedAt, promptUpdatedAt, settings.RefreshMode)

		reflection := ""
		cached := false
		reflectionCreatedAt := ""
		cacheStatus := decision.Status
		cacheStatusReason := decision.Reason
		status := "generated"
		errText := ""

		if convErr != nil {
			status = "error"
			errText = convErr.Error()
		} else if decision.UseCache && entry != nil {
			reflection = entry.Reflection
			cached = true
			reflectionCreatedAt = entry.CreatedAt
			status = "cached"
		} else if settings.DryRun {
			status = "dry_run"
		} else {
			copyPath, copyID, err := reflectcli.CreateCopyWithNewID(meta.Path, settings.Prefix)
			if err != nil {
				status = "error"
				errText = err.Error()
			} else {
				func() {
					defer func() { _ = os.Remove(copyPath) }()
					if err := reflectcli.RunCodexReflection(
						ctx,
						codexBin,
						copyID,
						selection.PromptText,
						settings.CodexSandbox,
						settings.CodexApproval,
						settings.CodexTimeoutSeconds,
						settings.Debug,
					); err != nil {
						status = "error"
						errText = err.Error()
						return
					}
					r, err := reflectcli.ExtractLastAssistantTextFromFile(copyPath)
					if err != nil {
						status = "error"
						errText = err.Error()
						return
					}
					reflection = r
				}()
			}
			if status != "error" && strings.TrimSpace(reflection) != "" {
				reflectionCreatedAt = reflectcli.NowISO()
				newEntry := reflectcli.CacheEntry{
					SessionID:          meta.ID,
					SessionTimestamp:   meta.Timestamp.UTC().Format(time.RFC3339),
					Project:            meta.ProjectName(),
					SourcePath:         meta.Path,
					Reflection:         reflection,
					CreatedAt:          reflectionCreatedAt,
					CacheSchemaVersion: reflectcli.CacheSchemaVersion,
					PromptVersion:      promptState.PromptVersion,
					PromptUpdatedAt:    promptState.UpdatedAt,
					PromptHash:         promptHash,
					Prompt:             selection.PromptText,
				}
				if err := reflectcli.WriteCacheEntry(cachePath, newEntry); err != nil {
					status = "error"
					errText = err.Error()
				}
				entryPath = cachePath
			}
		}

		row := types.NewRow(
			types.MRP("status", status),
			types.MRP("session_id", meta.ID),
			types.MRP("project", meta.ProjectName()),
			types.MRP("conversation_started_at", meta.Timestamp.UTC().Format(time.RFC3339)),
			types.MRP("conversation_updated_at", conv.UpdatedAtISO),
			types.MRP("conversation_title", conv.Title),
			types.MRP("reflection", reflection),
			types.MRP("cached", cached),
			types.MRP("cache_status", cacheStatus),
			types.MRP("cache_status_reason", cacheStatusReason),
			types.MRP("reflection_created_at", reflectionCreatedAt),
			types.MRP("source_path", filepath.Clean(meta.Path)),
			types.MRP("error", errText),
		)

		if settings.ExtraMetadata {
			row.Set("cache_path", entryPath)
			row.Set("prompt_source", selection.Source)
			row.Set("prompt_label", selection.PromptLabel(promptHash))
			row.Set("prompt_cache_key", promptCacheKey)
			row.Set("prompt_hash", promptHash)
			row.Set("prompt_version", promptState.PromptVersion)
			row.Set("prompt_updated_at", promptState.UpdatedAt)
		}

		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}
