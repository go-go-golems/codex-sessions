"""Configuration defaults for reflect_sessions."""

from __future__ import annotations

from pathlib import Path

CACHE_SCHEMA_VERSION = "2026-01-12-v1"
DEFAULT_MAX_REFLECTIONS = 10
DEFAULT_MAX_WORKERS = 4
DEFAULT_CODEX_TIMEOUT_SECONDS = 120
DEFAULT_SESSIONS_ROOT = Path.home() / ".codex" / "sessions"
DEFAULT_CACHE_DIR_NAME = "reflection_cache"
DEFAULT_PREFIX = "[SELF-REFLECTION] "
DEFAULT_PROMPT_PATH = Path(__file__).parent / "prompts" / "reflection.md"
PROMPT_VERSION_STATE_PATH = Path(__file__).parent / "prompts" / "reflection_version.json"
AUTO_REFRESH_REASONS = {
    "conversation_updated_after_reflection",
    "cache_schema_mismatch",
}
