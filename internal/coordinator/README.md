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
| `Scheduler` | `scheduler` | `RequestWork`; `decide` and `checkLateResult` (trust hook) |
| `Auth`, `Signer` | `security` | `Register`, `RequestWork` |
| `Workloads` | `workload` | `SubmitJob`, `finishJobIfDone` |

## Main flows

**Submit a job:** `SubmitJob` → workload `Split` → commit `JobAdded`.

**Give out work:** `RequestWork` → find open units that need a volunteer and that this volunteer does not have → `Scheduler.Pick` → commit `UnitAssigned` with the lease expiry → sign the payload.

**Take a result:** `SubmitResult` → commit `ResultRecorded` → `decide` → `Verifier.Decide` → commit `CopiesRaised` and/or `UnitAccepted` → when all units are done, workload `Merge` → commit `JobFinished`.

- `SubmitResult` holds `decideMu` from recording the result to the end of the vote, so each result is recorded and voted on in one step. Two results that arrive together give one decision (one accept or one raise, each wrong result counted once, no error for a volunteer whose result was recorded), and no result is recorded unseen while another vote is halfway. The job merge also runs under this lock, so a large merge pauses other votes for that time.
- **Late copies** (`latecopies.go`): when a unit is accepted, the volunteers that still hold a lease on it are remembered until their lease ends. If one of them sends its result later, the result is not recorded, but it is compared with the accepted output and counted as a wrong result if it differs (BOINC checks late results the same way). Results from any other volunteer are ignored, so nobody can make another volunteer look faulty. This is leader-local and is lost after a failover.
- **Trust hook:** after a unit is accepted, `recordVotes` reports each result in the vote as right or wrong, and `checkLateResult` reports each checked late copy. Both go to `Scheduler.RecordResult` if the scheduler is a `scheduler.TrustRecorder` (type assertion). `FIFO` is not, so it ignores trust.

**Every tick (leader only, `Deps.TickInterval`, default 1 second):**

- `releaseExpiredLeases`: `Leases.Expired` → commit `LeaseReleased`.
- `releaseDeadVolunteers`: for each lease held by a volunteer that `Membership.Dead` reports → commit `LeaseReleased`, count `sc_leases_dead_volunteer_total`, and forget the volunteer (also in the Scheduler, if it is a `scheduler.Forgetter`). If gossip was wrong, the volunteer gets `NotFound` on its next request and registers again.

**Every change:** `commit` → `Table.Validate` → `Replicator.Send` → `Table.Apply`. One mutex keeps the changes in order.

## Rules

- All work RPCs run only on the leader. A follower answers with gRPC `FailedPrecondition`. `internal/leaderclient` then tries the next coordinator.
- Change the table only through `commit`. Never call `Table.Apply` directly from an RPC.
- The volunteer list and the job start times are leader-local on purpose. After a failover, volunteers get `NotFound` and register again.

## Tests

`server_test.go` runs full jobs in one process with `bufconn` (no network). `membership_test.go` checks that a dead volunteer loses its lease before the lease expires. `voting_test.go` runs majority voting with faulty volunteers, concurrent decisions and late copies.

- Do not change `startNode` in `server_test.go`, because many tests use it.
- For a new part, copy the `startNode` pattern into your own test file, for example `lease_test.go`, and pass your implementation in `Deps`.

```bash
go test -race ./internal/coordinator/
```
