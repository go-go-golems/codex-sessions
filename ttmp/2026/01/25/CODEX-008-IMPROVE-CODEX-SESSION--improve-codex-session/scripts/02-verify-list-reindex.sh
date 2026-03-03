#!/usr/bin/env bash
set -euo pipefail

ROOT="/home/manuel/.codex/sessions"

echo "== index stats (before) =="
go run ./cmd/codex-session index stats --sessions-root "$ROOT"

echo "== list (limit 5) =="
go run ./cmd/codex-session list --sessions-root "$ROOT" --limit 5

echo "== index stats (after) =="
go run ./cmd/codex-session index stats --sessions-root "$ROOT"
