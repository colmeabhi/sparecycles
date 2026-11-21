#!/usr/bin/env bash
# Runs one load test and appends the result to results/load.csv.
#
# Usage: scripts/load.sh [VOLUNTEERS] [JOBS] [CLIENTS] [MODE]
#   VOLUNTEERS  volunteers to start (default 2)
#   JOBS        jobs to submit (default 8)
#   CLIENTS     clients that submit at the same time (default 2)
#   MODE        full (3 copies per unit) or baseline (1 copy, no voting). Default full.
# Set PARAMS to change the job, for example PARAMS='{"width":512,"height":512,"tile_size":128}'.

source "$(dirname "$0")/lib.sh"

VOLUNTEERS="${1:-2}"
JOBS="${2:-8}"
CLIENTS="${3:-2}"
MODE="${4:-full}"
case "$MODE" in
  full) REPLICAS=3 ;;
  baseline) REPLICAS=1 ;;
  *) fail "MODE must be full or baseline, not '$MODE'" ;;
esac

require_free_port 7400
build

say "start 1 coordinator ($MODE: $REPLICAS copies per unit) and $VOLUNTEERS volunteers"
start coordinator SC_VOTE_REPLICAS="$REPLICAS" "$BIN/coordinator"
wait_port 7400
for i in $(seq 1 "$VOLUNTEERS"); do
  start "volunteer-$i" SC_VOLUNTEER_NAME="volunteer-$i" "$BIN/volunteer"
done
wait_log "$RUN_DIR/coordinator.log" "volunteer registered" "$VOLUNTEERS" 20

say "run $JOBS jobs with $CLIENTS clients"
mkdir -p "$ROOT/results"
args=(-jobs "$JOBS" -concurrency "$CLIENTS" -label "$MODE" -volunteers "$VOLUNTEERS" -csv "$ROOT/results/load.csv")
if [ -n "${PARAMS:-}" ]; then
  args+=(-params "$PARAMS")
fi
( cd "$RUN_DIR" && "$BIN/loadgen" "${args[@]}" ) || fail "load run had failed jobs"

pass "load run finished ($MODE, $VOLUNTEERS volunteers)"
