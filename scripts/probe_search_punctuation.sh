#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR="$(mktemp -d)"
SESS="$TMPDIR/sessions/2026/03/02"

cleanup() {
  rm -rf "$TMPDIR"
}
trap cleanup EXIT

mkdir -p "$SESS"
cat > "$SESS/rollout-2026-03-02T10-00-00-test.jsonl" <<'EOF'
{"type":"session_meta","payload":{"id":"sid-1","timestamp":"2026-03-02T10:00:00Z","cwd":"/tmp/proj"}}
{"type":"event_msg","timestamp":"2026-03-02T10:00:01Z","payload":{"type":"user_message","message":"Investigate CODEX-001 in go-go-os at /tmp/test.txt and call functions.shell_command with foo/bar"}}
EOF

echo "Building index in: $TMPDIR/sessions/session_index.sqlite"
(
  cd "$ROOT_DIR"
  go run ./cmd/codex-session index build \
    --sessions-root "$TMPDIR/sessions" \
    --include-most-recent \
    --limit 100
)

queries=(
  "CODEX-001"
  "go-go-os"
  "/tmp/test.txt"
  "functions.shell_command"
  "foo/bar"
)

for q in "${queries[@]}"; do
  echo
  echo "---- QUERY: $q"
  (
    cd "$ROOT_DIR"
    go run ./cmd/codex-session search \
      --sessions-root "$TMPDIR/sessions" \
      --query "$q" \
      --include-most-recent \
      --max-results 10
  )
done

echo
echo "Probe completed successfully."
