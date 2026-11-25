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
| `Auth`, `Signer` | `security` | `Register`, `RequestWork` (session secrets: `Register`, `volunteer`) |
| `Workloads` | `workload` | `SubmitJob`, `finishJobIfDone` |

## Main flows

**Submit a job:** `SubmitJob` → workload `Split` → commit `JobAdded`.

**Register:** `Register` → `Auth.Authenticate(token)` → new volunteer ID and `security.NewSession()` → return the ID and the secret; keep only the hash.

**Check a volunteer (`volunteer`):** used first by `RequestWork` and `SubmitResult`. An unknown ID gives `NotFound`. A missing or wrong session secret gives `Unauthenticated`, counts `sc_requests_rejected_total` and logs `request rejected: bad session`. Gossip shares every volunteer ID, so the ID alone is not proof.

**Give out work:** `RequestWork` → find open units that need a volunteer and that this volunteer does not have → `Scheduler.Pick` → commit `UnitAssigned` with the lease expiry → sign the payload.

**Take a result:** `SubmitResult` → commit `ResultRecorded` → `decide` → `Verifier.Decide` → commit `CopiesRaised` and/or `UnitAccepted` → when all units are done, workload `Merge` → commit `JobFinished`.

- `SubmitResult` holds `decideMu` from recording the result to the end of the vote, so each result is recorded and voted on in one step. Two results that arrive together give one decision (one accept or one raise, each wrong result counted once, no error for a volunteer whose result was recorded), and no result is recorded unseen while another vote is halfway. The job merge also runs under this lock, so a large merge pauses other votes for that time.
- **Late copies** (`latecopies.go`): when a unit is accepted, the volunteers that still hold a lease on it are remembered until their lease ends. If one of them sends its result later, the result is not recorded, but it is compared with the accepted output and counted as a wrong result if it differs (BOINC checks late results the same way). Results from any other volunteer are ignored. Together with the session secrets (only the real volunteer can send under its ID), nobody can make another volunteer look faulty. This is leader-local and is lost after a failover.

**Every tick (leader only, `Deps.TickInterval`, default 1 second):**

- `releaseExpiredLeases`: `Leases.Expired` → commit `LeaseReleased`.
- `releaseDeadVolunteers`: for each lease held by a volunteer that `Membership.Dead` reports → commit `LeaseReleased`, count `sc_leases_dead_volunteer_total`, and forget the volunteer and its session. If gossip was wrong, the volunteer gets `NotFound` on its next request and registers again.
- `retryStuckVotes` (`recovery.go`): votes again for each open unit that has every copy back and no lease out, and finishes each running job whose units are all done (`Table.RunningJobs`). A vote or a finish only runs when a result arrives, so one whose commit failed, or that an old leader did not commit, would otherwise wait forever.
- `reportUnitsWithoutVolunteer` (`recovery.go`): sets `sc_units_without_volunteer` and logs `unit stuck: every volunteer already ran it` once per unit whose vote started and that needs another volunteer, but every registered volunteer has already run it.

**Every change:** `commit` → `Table.Validate` → `Replicator.Send` → `Table.Apply`. One mutex keeps the changes in order.

## Rules

- All work RPCs run only on the leader. A follower answers with gRPC `FailedPrecondition`. `internal/leaderclient` then tries the next coordinator.
- Change the table only through `commit`. Never call `Table.Apply` directly from an RPC.
- The volunteer list, the session hashes and the job start times are leader-local on purpose. After a failover, volunteers get `NotFound` and register again, with a new secret.
- Every volunteer RPC after `Register` must call `volunteer(id, secret)` before it reads or changes anything.

## Tests

`server_test.go` runs full jobs in one process with `bufconn` (no network). `membership_test.go` checks that a dead volunteer loses its lease before the lease expires. `session_test.go` sends requests under another volunteer's ID and checks that all are rejected. `voting_test.go` runs majority voting with faulty volunteers, concurrent decisions and late copies. `recovery_test.go` makes an accept or a job finish fail once and checks that the tick finishes them, and checks the stuck-unit report.

A test that calls `RequestWork` or `SubmitResult` directly must send the `session_secret` from its `RegisterResponse`.

- Do not change `startNode` in `server_test.go`, because many tests use it.
- For a new part, copy the `startNode` pattern into your own test file, for example `lease_test.go`, and pass your implementation in `Deps`.

```bash
go test -race ./internal/coordinator/
```
