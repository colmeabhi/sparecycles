#!/usr/bin/env bash
# Runs the load test in baseline mode (1, 2 and 4 volunteers) and full mode
# (2, 3 and 4 volunteers). All rows go to results/load.csv.
#
# Full mode starts at 2 volunteers: one volunteer never gets 2 copies of the
# same unit, so 2 of 3 copies can never agree.
#
# Usage: scripts/load-sweep.sh [JOBS] [CLIENTS]   (defaults: 8 jobs, 2 clients)

set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
JOBS="${1:-8}"
CLIENTS="${2:-2}"

for VOLUNTEERS in 1 2 4; do
  bash "$DIR/load.sh" "$VOLUNTEERS" "$JOBS" "$CLIENTS" baseline
done
for VOLUNTEERS in 2 3 4; do
  bash "$DIR/load.sh" "$VOLUNTEERS" "$JOBS" "$CLIENTS" full
done

echo
echo "PASS: sweep finished. See results/load.csv"
