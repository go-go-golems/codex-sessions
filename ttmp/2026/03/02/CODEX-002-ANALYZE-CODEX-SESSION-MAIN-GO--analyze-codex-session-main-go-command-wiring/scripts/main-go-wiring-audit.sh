#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${1:-/home/manuel/workspaces/2026-03-02/fix-codex-sessions/codex-sessions}"
MAIN_GO="${REPO_ROOT}/cmd/codex-session/main.go"

if [[ ! -f "${MAIN_GO}" ]]; then
  echo "error: main.go not found: ${MAIN_GO}" >&2
  exit 1
fi

cd "${REPO_ROOT}"

echo "repo_root=${REPO_ROOT}"

echo
echo "== 1) Constructor inventory vs registrations =="
constructors=$(rg -n "func New[A-Za-z0-9_]*Command\(" cmd/codex-session -S | wc -l | tr -d ' ')
registrations=$(rg -n "BuildCobraCommand\(" cmd/codex-session/main.go -S | wc -l | tr -d ' ')
parser_cfg=$(rg -n "WithParserConfig\(" cmd/codex-session/main.go -S | wc -l | tr -d ' ')

echo "constructors=${constructors}"
echo "build_cobra_registrations=${registrations}"
echo "parser_config_repetitions=${parser_cfg}"

echo
echo "constructors:"
rg -n "func New[A-Za-z0-9_]*Command\(" cmd/codex-session -S

echo
echo "registrations in main.go:"
rg -n "BuildCobraCommand\(" cmd/codex-session/main.go -S

echo
echo "== 2) Runtime check with workspace go.work (expected to fail in this checkout) =="
if go run ./cmd/codex-session --help >/tmp/codex-main-go-audit-gohelp.out 2>&1; then
  echo "go_run_with_go_work=ok"
else
  echo "go_run_with_go_work=FAIL"
  sed -n '1,40p' /tmp/codex-main-go-audit-gohelp.out
fi

echo
echo "== 3) Runtime check with GOWORK=off (baseline expected success) =="
if GOWORK=off go run ./cmd/codex-session --help >/tmp/codex-main-go-audit-gohelp-gowork-off.out 2>&1; then
  echo "go_run_gowork_off=ok"
  sed -n '1,40p' /tmp/codex-main-go-audit-gohelp-gowork-off.out
else
  echo "go_run_gowork_off=FAIL"
  sed -n '1,40p' /tmp/codex-main-go-audit-gohelp-gowork-off.out
fi

echo
echo "== 4) Root flag behavior (global glazed flags not available at root) =="
if GOWORK=off go run ./cmd/codex-session --print-schema >/tmp/codex-main-go-audit-root-flag.out 2>&1; then
  echo "unexpected_success"
else
  sed -n '1,40p' /tmp/codex-main-go-audit-root-flag.out
fi

echo
echo "== 5) CLI package test coverage around main wiring =="
if rg -n "main\.go|rootCmd|BuildCobraCommand" cmd/codex-session/*_test.go -S >/tmp/codex-main-go-audit-tests.out 2>&1; then
  sed -n '1,60p' /tmp/codex-main-go-audit-tests.out
else
  echo "no_main_wiring_tests_detected"
fi
