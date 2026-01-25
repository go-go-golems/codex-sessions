package reflect

import "time"

const (
	CacheSchemaVersion      = "2026-01-12-v1"
	DefaultCacheDirName     = "reflection_cache"
	DefaultPrefix           = "[SELF-REFLECTION] "
	DefaultPromptPreset     = "reflection"
	DefaultCodexSandbox     = "read-only"
	DefaultCodexApproval    = "never"
	DefaultCodexTimeoutSecs = 120
)

var autoRefreshReasons = map[string]bool{
	"conversation_updated_after_reflection": true,
	"cache_schema_mismatch":                 true,
}

func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
