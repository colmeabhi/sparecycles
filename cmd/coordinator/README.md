# coordinator (binary)

Runs one coordinator node: the gRPC server, the metrics server and the leader background loop.

**Owner:** all · **State:** complete with stub parts

## Run

```bash
make run-coordinator
```

## What `main` does

1. Loads `.env` and the coordinator settings (`internal/config`).
2. Builds each part and passes it to `coordinator.New` in `Deps`. The SWIM member has the ID `coord-<SC_NODE_ID>`, uses `SC_ADVERTISE_ADDR`, and has the other coordinators in `SC_PEERS` as seeds.
3. Picks the security parts (`newSecurity`): `SC_VOLUNTEER_TOKENS`, else `SC_VOLUNTEER_TOKEN`, else open; and Ed25519 signing if `SC_SIGNING_KEY` is set. It logs the choice.
4. Serves gRPC on `SC_LISTEN_ADDR` (default `:7400`).
5. Serves `/metrics` and `/healthz` on `SC_METRICS_ADDR` (default `:9400`).
6. Logs `coordinator started` with the copies per unit and the quorum, and `resource discovery` with the scheduler rules.
7. Stops cleanly on Ctrl+C or `SIGTERM`.

## Parts in use

| Part | Now | After |
| --- | --- | --- |
| Elector | `election.NewBully` (done, #13) | — |
| Replicator | `replication.NewQuorum` (done, #14) | — |
| Membership | `membership.NewSWIM` (done, #10) | — |
| Leases | `lease.NoExpiry` | Timeout (#6) |
| Verifier | `verify.Majority` with `SC_VOTE_QUORUM` (done, #7) | — |
| Scheduler | `scheduler.NewMatcher` with the `SC_TRUST_*`, `SC_SLOW_SPEED` and `SC_MIN_MEMORY_MB` settings (done, #8) | — |
| Auth, Signer | `security.TokenList` or `SharedToken`, and `Ed25519Signer` or `NoSign`, from the settings (done, #11) | — |

To replace a part, change only its line in `main.go`, and update this table.

## Election and replication

`newElectionAndReplication` builds the Bully elector and the Quorum replicator together:

- The elector uses `SC_NODE_ID`, `SC_PEERS`, `SC_ELECTION_HEARTBEAT` and `SC_ELECTION_TIMEOUT`. With no peers, the node leads at once, so `make run-coordinator` works alone.
- The replicator shares the task table and uses `SC_REPLICATION_TIMEOUT`.
- The elector's `BeforeLead` hook calls `Quorum.CatchUp`, so a new leader loads the newest table before it acts.
- Both use one connection pool to the other coordinators (`internal/peerconn`). The gRPC server and the pool accept messages up to 256 MB, for table snapshots and large results.

## Three coordinators

`make up` (Docker Compose) runs `coordinator-1` to `coordinator-3`. Each has its own `SC_NODE_ID` and the other two in `SC_PEERS`:

| Service | gRPC on the host | Metrics on the host |
| --- | --- | --- |
| `coordinator-1` | 7401 | 9401 |
| `coordinator-2` | 7402 | 9402 |
| `coordinator-3` | 7403 | 9403 |

Coordinator 3 leads first (highest ID). `docker compose stop coordinator-3` makes coordinator 2 the leader; it continues from the replicated table.
