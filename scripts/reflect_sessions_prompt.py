"""Prompt handling for reflect_sessions."""

from __future__ import annotations

import json
import hashlib
from datetime import datetime
from pathlib import Path

from reflect_sessions_config import DEFAULT_PROMPT_PATH
from reflect_sessions_models import PromptVersionState
from reflect_sessions_time import now_iso, utc_date_str


def next_prompt_version(*, previous_version: str | None, date_str: str) -> str:
    """Build the next prompt version for the given date.

    Args:
        previous_version: Last prompt version string.
        date_str: Date string in YYYY-MM-DD format.

    Returns:
        New prompt version string.
    """
    if previous_version and previous_version.startswith(f"{date_str}-v"):
        suffix = previous_version.split("-v", maxsplit=1)[-1]
        if suffix.isdigit():
            return f"{date_str}-v{int(suffix) + 1}"
    return f"{date_str}-v1"


def load_prompt_version_state(*, path: Path) -> PromptVersionState:
    """Load prompt version state from disk.

    Args:
        path: Path to the prompt version state file.

    Returns:
        PromptVersionState instance.
    """
    if not path.exists():
        return PromptVersionState(prompt_version="", prompt_hash="", updated_at="")
    data = json.loads(path.read_text())
    assert isinstance(data, dict), "Prompt version state is not a dict"
    return PromptVersionState.from_dict(data=data)


def write_prompt_version_state(*, path: Path, state: PromptVersionState) -> None:
    """Write prompt version state to disk.

    Args:
        path: Path to the prompt version state file.
        state: Prompt version state to write.
    """
    payload = json.dumps(state.to_dict(), indent=2, ensure_ascii=True)
    path.write_text(payload + "\n")


def ensure_prompt_version_state(
    *, state_path: Path, prompt_hash: str, now: datetime
) -> PromptVersionState:
    """Ensure the prompt version state matches the current prompt hash.

    Args:
        state_path: Path to the prompt version state file.
        prompt_hash: SHA-256 hash of the prompt text.
        now: Current timestamp in UTC.

    Returns:
        PromptVersionState reflecting the latest prompt hash.

    Example:
        state = ensure_prompt_version_state(
            state_path=Path("prompts/reflection_version.json"),
            prompt_hash="hash",
            now=datetime(2026, 1, 12, tzinfo=timezone.utc),
        )
    """
    state = load_prompt_version_state(path=state_path)
    date_str = utc_date_str(timestamp=now)
    if not state.prompt_version or state.prompt_hash != prompt_hash:
        prompt_version = next_prompt_version(
            previous_version=state.prompt_version or None,
            date_str=date_str,
        )
        state = PromptVersionState(
            prompt_version=prompt_version,
            prompt_hash=prompt_hash,
            updated_at=now_iso(),
        )
        write_prompt_version_state(path=state_path, state=state)
    return state


def resolve_prompt_path(*, prompt_file: Path | None) -> Path:
    """Resolve the prompt file path.

    Args:
        prompt_file: Optional file path containing the prompt.

    Returns:
        Path to the prompt file.

    Example:
        prompt_path = resolve_prompt_path(prompt_file=None)
    """
    resolved = prompt_file or DEFAULT_PROMPT_PATH
    assert resolved.exists(), f"Prompt file not found: {resolved}"
    return resolved


def load_prompt_text(*, prompt_path: Path) -> str:
    """Load the reflection prompt text.

    Args:
        prompt_path: Path containing the prompt.

    Returns:
        Prompt string.

    Example:
        prompt = load_prompt_text(prompt_path=Path("prompts/reflection.md"))
    """
    text = prompt_path.read_text().strip()
    assert text, f"Prompt file is empty: {prompt_path}"
    return text


def compute_prompt_hash(*, prompt_text: str) -> str:
    """Compute a SHA-256 hash for the prompt text.

    Args:
        prompt_text: Prompt text to hash.

    Returns:
        Hex digest for the prompt text.

    Example:
        digest = compute_prompt_hash(prompt_text="Hello")
    """
    return hashlib.sha256(prompt_text.encode("utf-8")).hexdigest()
