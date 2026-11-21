#!/usr/bin/env bash
# Shared helpers for the SpareCycles scripts. Source this file; do not run it.
#
# Each script builds the binaries, starts a local cluster as plain processes,
# and keeps every log in runs/<script>-<time>/. The processes run inside that
# folder, so a personal .env in the repo root does not change the results.
# All processes stop when the script ends, also on Ctrl+C or an error.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/bin"
RUN_DIR="$ROOT/runs/$(basename "$0" .sh)-$(date +%Y%m%d-%H%M%S)"
PIDS=()
LAST_PID=""

# Fast gossip, so that a failure is found in a few seconds during a demo.
GOSSIP_FAST=(SC_GOSSIP_INTERVAL=300ms SC_GOSSIP_PING_TIMEOUT=150ms SC_GOSSIP_SUSPECT_TIMEOUT=2s)

say()  { printf '\n== %s\n' "$*"; }
pass() { printf '\nPASS: %s\nLogs: %s\n' "$*" "$RUN_DIR"; exit 0; }
fail() { printf '\nFAIL: %s\nLogs: %s\n' "$*" "$RUN_DIR" >&2; exit 1; }
skip() { printf '\nSKIP: %s\n' "$*"; exit 0; }

stop_all() {
  local p
  for p in "${PIDS[@]:-}"; do
    [ -n "$p" ] && kill "$p" 2>/dev/null || true
  done
  wait 2>/dev/null || true
}
trap stop_all EXIT

build() {
  say "build"
  (cd "$ROOT" && go build -o bin/ ./cmd/...)
  mkdir -p "$RUN_DIR"
}

# has_setting NAME: true if .env.example documents the setting. Scripts use it
# to skip a test whose feature (another issue) is not merged yet.
has_setting() { grep -q "^$1=" "$ROOT/.env.example"; }

require_free_port() {
  if nc -z 127.0.0.1 "$1" 2>/dev/null; then
    fail "port $1 is busy. Stop other coordinators first."
  fi
}

wait_port() {
  local i
  for i in $(seq 1 100); do
    nc -z 127.0.0.1 "$1" 2>/dev/null && return 0
    sleep 0.1
  done
  fail "nothing listens on port $1"
}

# start NAME [KEY=VALUE...] BINARY [ARGS...]
# Runs a process in the run folder, logs to NAME.log, and sets LAST_PID.
start() {
  local name="$1"; shift
  ( cd "$RUN_DIR" && exec env "$@" ) >"$RUN_DIR/$name.log" 2>&1 &
  LAST_PID=$!
  PIDS+=("$LAST_PID")
}

# count FILE PATTERN: number of lines in FILE that match PATTERN.
count() { grep -c -- "$2" "$1" 2>/dev/null || true; }

# wait_log FILE PATTERN N SECONDS: wait until FILE has N lines that match.
wait_log() {
  local i limit=$(( $4 * 10 ))
  for i in $(seq 1 "$limit"); do
    [ "$(count "$1" "$2")" -ge "$3" ] && return 0
    sleep 0.1
  done
  fail "timed out waiting for $3 x '$2' in $(basename "$1")"
}

# show FILE PATTERN: print matching log lines, with the time shortened.
show() {
  grep -E -- "$2" "$1" | sed -E 's/^time=[0-9-]+T([0-9:]+)\.[0-9]+[^ ]* /\1 /; s/ component=[a-z]+//' || true
}
