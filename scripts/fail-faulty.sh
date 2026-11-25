#!/usr/bin/env bash
# Failure scenario 2: a volunteer sends wrong results.
#
# 1 coordinator, 3 honest volunteers and 1 faulty volunteer (SC_FAULTY_RATE=1.0),
# 3 copies per unit. Majority voting must outvote the faulty volunteer, and the
# image must be the same as an image computed by honest volunteers only.
#
# Needs issue #7 (majority voting and SC_FAULTY_RATE). Until then it skips.
#
# Usage: scripts/fail-faulty.sh

source "$(dirname "$0")/lib.sh"

has_setting SC_FAULTY_RATE || skip "needs issue #7 (majority voting and SC_FAULTY_RATE)"

JOB='{"width":1024,"height":1024,"tile_size":128,"max_iter":2000}'

require_free_port 7400
build

say "start 1 coordinator (3 copies per unit) and 3 honest volunteers"
start coordinator SC_VOTE_REPLICAS=3 "$BIN/coordinator"
wait_port 7400
for i in 1 2 3; do
  start "honest-$i" SC_VOLUNTEER_NAME="honest-$i" "$BIN/volunteer"
done
wait_log "$RUN_DIR/coordinator.log" "volunteer registered" 3 20

say "reference job, honest volunteers only"
( cd "$RUN_DIR" && "$BIN/client" -params "$JOB" -out "$RUN_DIR/reference.png" -timeout 3m ) || fail "the reference job failed"

say "add a faulty volunteer and run the same job"
start faulty SC_VOLUNTEER_NAME=faulty SC_FAULTY_RATE=1.0 "$BIN/volunteer"
wait_log "$RUN_DIR/coordinator.log" "volunteer registered" 4 20
( cd "$RUN_DIR" && "$BIN/client" -params "$JOB" -out "$RUN_DIR/result.png" -timeout 3m ) || fail "the job with a faulty volunteer failed"

say "coordinator log"
show "$RUN_DIR/coordinator.log" "wrong result|no majority|volunteer blocked|job finished" | tail -n 10

[ "$(count "$RUN_DIR/coordinator.log" "wrong result")" -ge 1 ] || fail "no result was outvoted"
cmp -s "$RUN_DIR/reference.png" "$RUN_DIR/result.png" || fail "the image differs from the honest reference"
pass "the faulty volunteer was outvoted and the image is correct"
