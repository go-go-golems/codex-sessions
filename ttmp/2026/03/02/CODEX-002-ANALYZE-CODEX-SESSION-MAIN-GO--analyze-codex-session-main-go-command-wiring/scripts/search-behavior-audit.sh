#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
GO_RUN=(go run ./cmd/codex-session)

search_count() {
  local root="$1"
  local query="$2"
  shift 2
  local out
  out="$(
    cd "$REPO_ROOT"
    "${GO_RUN[@]}" search \
      --sessions-root "$root" \
      --query "$query" \
      --output json \
      "$@"
  )"
  if [[ -z "${out//[[:space:]]/}" ]]; then
    echo "0"
    return
  fi
  printf '%s\n' "$out" | jq 'if type == "array" then length else 0 end'
}

mk_session() {
  local path="$1"
  local sid="$2"
  local ts="$3"
  local cwd="$4"
  local user_msg="$5"
  cat > "$path" <<JSON
{"type":"session_meta","payload":{"id":"$sid","timestamp":"$ts","cwd":"$cwd"}}
{"type":"event_msg","timestamp":"$ts","payload":{"type":"user_message","message":"$user_msg"}}
JSON
}

section() {
  echo
  echo "== $1 =="
}

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

SESS="$TMPDIR/sessions"
mkdir -p "$SESS/2026/03/02"

section "1) Case-sensitive parity (indexed vs fallback)"
mk_session "$SESS/2026/03/02/rollout-2026-03-02T10-00-00-a.jsonl" "sid-a" "2026-03-02T10:00:00Z" "/tmp/proj" "AlphaBeta alphabeta"
(
  cd "$REPO_ROOT"
  "${GO_RUN[@]}" index build --sessions-root "$SESS" --include-most-recent --include-tool-outputs --output json >/dev/null
)
idx_cs_false=$(search_count "$SESS" "ALPHABETA" --case-sensitive=false --include-most-recent --use-index=true)
idx_cs_true=$(search_count "$SESS" "ALPHABETA" --case-sensitive=true --include-most-recent --use-index=true)
fb_cs_false=$(search_count "$SESS" "ALPHABETA" --case-sensitive=false --include-most-recent --use-index=false)
fb_cs_true=$(search_count "$SESS" "ALPHABETA" --case-sensitive=true --include-most-recent --use-index=false)

echo "indexed case-insensitive count: $idx_cs_false"
echo "indexed case-sensitive count (falls back): $idx_cs_true"
echo "fallback case-insensitive count: $fb_cs_false"
echo "fallback case-sensitive count: $fb_cs_true"

section "2) Flag parity checks"
# add second session to exercise --limit mismatch
mk_session "$SESS/2026/03/02/rollout-2026-03-02T10-01-00-b.jsonl" "sid-b" "2026-03-02T10:01:00Z" "/tmp/proj" "shared-term"
mk_session "$SESS/2026/03/02/rollout-2026-03-02T10-02-00-c.jsonl" "sid-c" "2026-03-02T10:02:00Z" "/tmp/proj" "shared-term"
mk_session "$SESS/2026/03/02/rollout-2026-03-02T10-02-30-punct.jsonl" "sid-punct" "2026-03-02T10:02:30Z" "/tmp/proj" "Investigate CODEX-001 in go-go-os at /tmp/test.txt and call functions.shell_command with foo/bar"
(
  cd "$REPO_ROOT"
  "${GO_RUN[@]}" index build --sessions-root "$SESS" --include-most-recent --include-reflection-copies --output json >/dev/null
)
idx_limit=$(search_count "$SESS" "shared-term" --include-most-recent --limit=1 --use-index=true)
fb_limit=$(search_count "$SESS" "shared-term" --include-most-recent --limit=1 --use-index=false)

echo "indexed count with --limit=1: $idx_limit"
echo "fallback count with --limit=1: $fb_limit"

section "3) Scope correctness (messages/tools/all)"
cat > "$SESS/2026/03/02/rollout-2026-03-02T10-03-00-tools.jsonl" <<'JSON'
{"type":"session_meta","payload":{"id":"sid-tools","timestamp":"2026-03-02T10:03:00Z","cwd":"/tmp/proj"}}
{"type":"event_msg","timestamp":"2026-03-02T10:03:01Z","payload":{"type":"user_message","message":"message-only-token"}}
{"type":"response_item","timestamp":"2026-03-02T10:03:02Z","payload":{"type":"custom_tool_call","status":"completed","call_id":"call_1","name":"functions.shell_command","input":"{\"command\":\"echo tool-call-token\"}"}}
{"type":"response_item","timestamp":"2026-03-02T10:03:03Z","payload":{"type":"custom_tool_call_output","call_id":"call_1","output":"tool-output-token"}}
JSON
(
  cd "$REPO_ROOT"
  "${GO_RUN[@]}" index build --sessions-root "$SESS" --include-most-recent --include-tool-calls --include-tool-outputs --force --output json >/dev/null
)
msg_scope=$(search_count "$SESS" "message-only-token" --scope messages --include-most-recent --use-index=true)
tools_scope_call=$(search_count "$SESS" "tool-call-token" --scope tools --include-most-recent --use-index=true)
tools_scope_out=$(search_count "$SESS" "tool-output-token" --scope tools --include-most-recent --use-index=true)
all_scope=$(search_count "$SESS" "tool-output-token" --scope all --include-most-recent --use-index=true)

echo "scope=messages message-only-token: $msg_scope"
echo "scope=tools tool-call-token: $tools_scope_call"
echo "scope=tools tool-output-token: $tools_scope_out"
echo "scope=all tool-output-token: $all_scope"

section "4) Staleness detection gap"
mk_session "$SESS/2026/03/02/rollout-2026-03-02T10-04-00-stale.jsonl" "sid-stale" "2026-03-02T10:04:00Z" "/tmp/proj" "old-term"
(
  cd "$REPO_ROOT"
  "${GO_RUN[@]}" index build --sessions-root "$SESS" --include-most-recent --force --output json >/dev/null
)
# mutate JSONL after indexing so index becomes stale
cat >> "$SESS/2026/03/02/rollout-2026-03-02T10-04-00-stale.jsonl" <<'JSON'
{"type":"event_msg","timestamp":"2026-03-02T10:05:00Z","payload":{"type":"user_message","message":"new-term-from-late-write"}}
JSON
idx_new=$(search_count "$SESS" "new-term-from-late-write" --include-most-recent --use-index=true)
fb_new=$(search_count "$SESS" "new-term-from-late-write" --include-most-recent --use-index=false)

echo "indexed new-term count after late write: $idx_new"
echo "fallback new-term count after late write: $fb_new"

section "5) Query semantics safety smoke"
for q in "CODEX-001" "go-go-os" "/tmp/test.txt" "functions.shell_command" "foo/bar"; do
  c=$(search_count "$SESS" "$q" --include-most-recent --use-index=true)
  echo "query=$q indexed_count=$c"
done

echo
echo "Audit complete."
