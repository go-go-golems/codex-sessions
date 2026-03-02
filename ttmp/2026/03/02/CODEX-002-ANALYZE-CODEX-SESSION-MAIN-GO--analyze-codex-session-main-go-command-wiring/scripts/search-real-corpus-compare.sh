#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage:
  $0 --sessions-root <path> --query <text> [--project <name>] [--since <iso>] [--until <iso>] [--scope <messages|tools|all>] [--include-most-recent] [--skip-index-build]

Compares search results between indexed mode and fallback mode for the same query/filter set.
USAGE
}

REPO_ROOT="$(git rev-parse --show-toplevel)"
GO_RUN=(go run ./cmd/codex-session)
SESSIONS_ROOT=""
QUERY=""
PROJECT=""
SINCE=""
UNTIL=""
SCOPE="messages"
INCLUDE_MOST_RECENT=0
SKIP_INDEX_BUILD=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sessions-root) SESSIONS_ROOT="$2"; shift 2 ;;
    --query) QUERY="$2"; shift 2 ;;
    --project) PROJECT="$2"; shift 2 ;;
    --since) SINCE="$2"; shift 2 ;;
    --until) UNTIL="$2"; shift 2 ;;
    --scope) SCOPE="$2"; shift 2 ;;
    --include-most-recent) INCLUDE_MOST_RECENT=1; shift ;;
    --skip-index-build) SKIP_INDEX_BUILD=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown arg: $1"; usage; exit 1 ;;
  esac
done

if [[ -z "$SESSIONS_ROOT" || -z "$QUERY" ]]; then
  usage
  exit 1
fi

COMMON_ARGS=(--sessions-root "$SESSIONS_ROOT" --query "$QUERY" --scope "$SCOPE" --output json)
if [[ -n "$PROJECT" ]]; then COMMON_ARGS+=(--project "$PROJECT"); fi
if [[ -n "$SINCE" ]]; then COMMON_ARGS+=(--since "$SINCE"); fi
if [[ -n "$UNTIL" ]]; then COMMON_ARGS+=(--until "$UNTIL"); fi
if [[ "$INCLUDE_MOST_RECENT" -eq 1 ]]; then COMMON_ARGS+=(--include-most-recent); fi

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT
IDX_JSON="$TMPDIR/index.json"
FB_JSON="$TMPDIR/fallback.json"

(
  if [[ "$SKIP_INDEX_BUILD" -eq 0 ]]; then
    cd "$REPO_ROOT"
    INDEX_ARGS=(index build --sessions-root "$SESSIONS_ROOT" --include-reflection-copies --include-tool-calls --include-tool-outputs --output json)
    if [[ "$INCLUDE_MOST_RECENT" -eq 1 ]]; then INDEX_ARGS+=(--include-most-recent); fi
    if [[ -n "$PROJECT" ]]; then INDEX_ARGS+=(--project "$PROJECT"); fi
    if [[ -n "$SINCE" ]]; then INDEX_ARGS+=(--since "$SINCE"); fi
    if [[ -n "$UNTIL" ]]; then INDEX_ARGS+=(--until "$UNTIL"); fi
    "${GO_RUN[@]}" "${INDEX_ARGS[@]}" >/dev/null
  fi
)

(
  cd "$REPO_ROOT"
  "${GO_RUN[@]}" search "${COMMON_ARGS[@]}" --use-index=true --stale-index-policy=ignore >"$IDX_JSON" || true
)
(
  cd "$REPO_ROOT"
  "${GO_RUN[@]}" search "${COMMON_ARGS[@]}" --use-index=false >"$FB_JSON" || true
)

normalize_json() {
  local f="$1"
  if [[ ! -s "$f" ]]; then
    echo "[]"
    return
  fi
  cat "$f"
}

idx_count=$(normalize_json "$IDX_JSON" | jq 'if type=="array" then length else 0 end')
fb_count=$(normalize_json "$FB_JSON" | jq 'if type=="array" then length else 0 end')

echo "query: $QUERY"
echo "scope: $SCOPE"
echo "indexed_count: $idx_count"
echo "fallback_count: $fb_count"

echo
echo "indexed_only_session_ids:"
comm -23 \
  <(normalize_json "$IDX_JSON" | jq -r '.[].session_id // empty' | sort -u) \
  <(normalize_json "$FB_JSON" | jq -r '.[].session_id // empty' | sort -u) || true

echo
echo "fallback_only_session_ids:"
comm -13 \
  <(normalize_json "$IDX_JSON" | jq -r '.[].session_id // empty' | sort -u) \
  <(normalize_json "$FB_JSON" | jq -r '.[].session_id // empty' | sort -u) || true

echo
echo "sample_indexed_results:"
normalize_json "$IDX_JSON" | jq '.[0:5]'

echo
echo "sample_fallback_results:"
normalize_json "$FB_JSON" | jq '.[0:5]'
