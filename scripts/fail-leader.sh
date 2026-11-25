#!/usr/bin/env bash
# Failure scenario 3: the leader coordinator crashes in the middle of a job.
#
# 3 coordinators (node 3 leads, because Bully picks the highest ID) and
# 3 volunteers. A job starts, then node 3 is killed with kill -9. A new leader
# must be elected in a higher term, continue from the replicated task table,
# and finish the job.
#
# Needs issue #14 (replication); #13 (Bully election) is done. Until #14 it skips.
#
# Usage: scripts/fail-leader.sh

source "$(dirname "$0")/lib.sh"

has_setting SC_REPLICATION_TIMEOUT || skip "needs issue #14 (replication); #13 (Bully election) is done"

JOB='{"width":2048,"height":2048,"tile_size":128,"max_iter":3000}'
COORDINATORS="127.0.0.1:7401,127.0.0.1:7402,127.0.0.1:7403"

for p in 7401 7402 7403 9401 9402 9403; do require_free_port "$p"; done
build

say "start 3 coordinators"
for n in 1 2 3; do
  peers=""
  for m in 1 2 3; do
    [ "$m" = "$n" ] && continue
    peers="${peers:+$peers,}$m=127.0.0.1:740$m"
  done
  start "coordinator-$n" SC_NODE_ID="$n" SC_LISTEN_ADDR=":740$n" SC_METRICS_ADDR=":940$n" \
    SC_ADVERTISE_ADDR="127.0.0.1:740$n" SC_PEERS="$peers" "${GOSSIP_FAST[@]}" "$BIN/coordinator"
  [ "$n" = 3 ] && LEADER=$LAST_PID
done
for p in 7401 7402 7403; do wait_port "$p"; done

say "start 3 volunteers"
for i in 1 2 3; do
  start "volunteer-$i" SC_VOLUNTEER_NAME="volunteer-$i" SC_COORDINATORS="$COORDINATORS" "${GOSSIP_FAST[@]}" "$BIN/volunteer"
done
wait_log "$RUN_DIR/coordinator-3.log" "volunteer registered" 3 30

say "start a 256-unit job"
start client SC_COORDINATORS="$COORDINATORS" "$BIN/client" -params "$JOB" -out "$RUN_DIR/result.png" -timeout 5m
CLIENT=$LAST_PID
wait_log "$RUN_DIR/volunteer-1.log" "result sent" 5 60

say "kill -9 the leader, node 3 (pid $LEADER)"
kill -9 "$LEADER"

if ! wait "$CLIENT"; then
  fail "the client did not finish the job after the leader crash"
fi

say "coordinator 2 log"
show "$RUN_DIR/coordinator-2.log" "election started|became leader|new leader|snapshot restored|job finished"

[ "$(count "$RUN_DIR/coordinator-2.log" "became leader")" -ge 1 ] || fail "node 2 did not become the leader"
pass "node 2 took over after the leader crash, and the job finished"
