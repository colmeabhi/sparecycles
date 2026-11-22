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
6. Logs `coordinator started` with the lease length.
7. Stops cleanly on Ctrl+C or `SIGTERM`.

## Parts in use

| Part | Now | After |
| --- | --- | --- |
| Elector | `election.NewStatic` | Bully (#13) |
| Replicator | `replication.Noop` | Quorum (#14) |
| Membership | `membership.NewSWIM` (done, #10) | — |
| Leases | `lease.Timeout` with `SC_LEASE_DURATION` (done, #6) | — |
| Verifier | `verify.FirstResult` | Majority (#7) |
| Scheduler | `scheduler.FIFO` | Matcher (#8) |
| Auth, Signer | `security.TokenList` or `SharedToken`, and `Ed25519Signer` or `NoSign`, from the settings (done, #11) | — |

To replace a part, change only its line in `main.go`, and update this table.
