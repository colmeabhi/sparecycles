#!/usr/bin/env bash
# Runs the load test for 1, 2 and 4 volunteers, in baseline and full mode.
# All rows go to results/load.csv.
#
# Usage: scripts/load-sweep.sh [JOBS] [CLIENTS]   (defaults: 8 jobs, 2 clients)

set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
JOBS="${1:-8}"
CLIENTS="${2:-2}"

for MODE in baseline full; do
  for VOLUNTEERS in 1 2 4; do
    bash "$DIR/load.sh" "$VOLUNTEERS" "$JOBS" "$CLIENTS" "$MODE"
  done
done

echo
echo "PASS: sweep finished. See results/load.csv"
