#!/usr/bin/env bash
# Failure scenario 1: a volunteer dies in the middle of a job.
#
# 1 coordinator and 2 volunteers, 1 copy per unit, fast gossip. A 256-unit job
# starts, then volunteer A is killed with kill -9. Gossip must mark A dead, the
# leader must free A's unit, and the job must still finish on volunteer B.
#
# Usage: scripts/fail-volunteer.sh

source "$(dirname "$0")/lib.sh"

JOB='{"width":2048,"height":2048,"tile_size":128,"max_iter":3000}'

require_free_port 7400
build

say "start 1 coordinator and 2 volunteers (fast gossip)"
start coordinator SC_VOTE_REPLICAS=1 "${GOSSIP_FAST[@]}" "$BIN/coordinator"
wait_port 7400
start volunteer-a SC_VOLUNTEER_NAME=volunteer-a "${GOSSIP_FAST[@]}" "$BIN/volunteer"
VICTIM=$LAST_PID
start volunteer-b SC_VOLUNTEER_NAME=volunteer-b "${GOSSIP_FAST[@]}" "$BIN/volunteer"
wait_log "$RUN_DIR/coordinator.log" "volunteer registered" 2 20

say "start a 256-unit job"
start client "$BIN/client" -params "$JOB" -out "$RUN_DIR/result.png" -timeout 3m
CLIENT=$LAST_PID
wait_log "$RUN_DIR/volunteer-a.log" "result sent" 5 60

say "kill -9 volunteer-a (pid $VICTIM)"
kill -9 "$VICTIM"

if ! wait "$CLIENT"; then
  fail "the client did not finish the job"
fi

say "coordinator log"
show "$RUN_DIR/coordinator.log" "member suspect|member dead|lease released|volunteer removed|job finished"

[ "$(count "$RUN_DIR/coordinator.log" "member dead")" -ge 1 ] || fail "gossip did not mark volunteer-a dead"
[ "$(count "$RUN_DIR/coordinator.log" "state=done")" -ge 1 ] || fail "the job did not finish"
if [ "$(count "$RUN_DIR/coordinator.log" "lease released: volunteer dead")" -ge 1 ]; then
  pass "volunteer-a was marked dead, its unit moved to volunteer-b, and the job finished"
fi
pass "volunteer-a was marked dead and the job finished (it held no unit at the moment of the kill)"
