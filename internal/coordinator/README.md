# coordinator

The coordinator server. It handles the client and volunteer RPCs and connects all other parts.

**Owner:** all (discuss changes with the team) · **State:** complete

## How it fits

`cmd/coordinator` builds a `Server` with `Deps`. Each field in `Deps` is an interface from another package, so each owner can replace a stub without changing this package.

| Dep | Package | Used in |
| --- | --- | --- |
| `Elector` | `election` | `requireLeader`, `tick`, `Run` |
| `Replicator` | `replication` | `commit` |
| `Membership` | `membership` | `Register`, `Run` |
| `Leases` | `lease` | `RequestWork` (lease length), `tick` (expired leases) |
| `Verifier` | `verify` | `decide` |
| `Scheduler` | `scheduler` | `RequestWork` |
| `Auth`, `Signer` | `security` | `Register`, `RequestWork` |
| `Workloads` | `workload` | `SubmitJob`, `finishJobIfDone` |

## Main flows

**Submit a job:** `SubmitJob` → workload `Split` → commit `JobAdded`.

**Give out work:** `RequestWork` → find open units that need a volunteer and that this volunteer does not have → `Scheduler.Pick` → commit `UnitAssigned` with the lease expiry → sign the payload.

**Take a result:** `SubmitResult` → commit `ResultRecorded` → `decide` → `Verifier.Decide` → commit `CopiesRaised` and/or `UnitAccepted` → when all units are done, workload `Merge` → commit `JobFinished`.

**Every tick (leader only, `Deps.TickInterval`, default 1 second):**

- `releaseExpiredLeases`: `Leases.Expired` → commit `LeaseReleased`.
- `releaseDeadVolunteers`: for each lease held by a volunteer that `Membership.Dead` reports → commit `LeaseReleased`, count `sc_leases_dead_volunteer_total`, and forget the volunteer. If gossip was wrong, the volunteer gets `NotFound` on its next request and registers again.

**Every change:** `commit` → `Table.Validate` → `Replicator.Send` → `Table.Apply`. One mutex keeps the changes in order.

## Rules

- All work RPCs run only on the leader. A follower answers with gRPC `FailedPrecondition`. `internal/leaderclient` then tries the next coordinator.
- Change the table only through `commit`. Never call `Table.Apply` directly from an RPC.
- The volunteer list and the job start times are leader-local on purpose. After a failover, volunteers get `NotFound` and register again.

## Tests

`server_test.go` runs full jobs in one process with `bufconn` (no network). `membership_test.go` checks that a dead volunteer loses its lease before the lease expires.

- Do not change `startNode` in `server_test.go`, because many tests use it.
- For a new part, copy the `startNode` pattern into your own test file, for example `lease_test.go`, and pass your implementation in `Deps`.

```bash
go test -race ./internal/coordinator/
```
