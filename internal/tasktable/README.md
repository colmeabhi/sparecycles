# tasktable

The task table holds the state of all jobs and work units. Every coordinator has one copy.

**Owner:** all (discuss changes with the team) · **State:** complete

## How it fits

- The leader changes the table only through `Change` records and `Apply`.
- The leader sends each change to the backups (`internal/replication`) before it applies the change.
- The same changes, applied in the same order, give the same table on every coordinator. Replication depends on this.

## Files

| File | Content |
| --- | --- |
| `change.go` | `Change`, the change kinds, `Encode` and `DecodeChange` |
| `table.go` | `Table`, `Job`, `Unit`, `Assignment`, `Result`, `Apply`, `Validate`, `Snapshot`, `Restore` |
| `table_test.go` | Life cycle, lease release, replay and snapshot tests; dropped outputs after accept and after job finish |

## Change kinds

| Kind | Effect |
| --- | --- |
| `JobAdded` | Adds a job and its units, each with `Copies` volunteers to run it |
| `UnitAssigned` | Gives a lease on a unit to a volunteer |
| `LeaseReleased` | Removes a lease without a result (for example, it expired) |
| `ResultRecorded` | Stores a result and removes the lease |
| `CopiesRaised` | Asks more volunteers to run a unit (no majority) |
| `UnitAccepted` | Marks a unit as done with its accepted output and its hash. Drops the payload and the output bytes of each copy (their hashes stay). |
| `JobFinished` | Stores the merged result or an error. Drops the accepted outputs of the job's units (their hashes stay). |

## Size

The table drops the bytes that no step needs any more. Without this, each finished fractal job kept about 4 MB of tile outputs, and a coordinator under load grew by 7 to 9 MB per second. The catch-up snapshot after a failover then needed several GB, and Docker stopped the coordinators for lack of memory (2026-11-25).

| Kept for a finished job | Dropped |
| --- | --- |
| Job: parameters, unit IDs, merged result or error | Unit payloads |
| Unit: state, copies, volunteer IDs and hashes of the copies, hash of the accepted output | Output bytes of each copy, accepted output bytes |

The merged result of each job stays, because the client fetches it. For a fractal job it is a PNG of about 50 KB. The table still grows by that much per job; a long-running cluster would need a way to remove finished jobs.

## Rules

- `Apply` must be deterministic. Do not read the clock, random numbers or maps in random order inside `Apply`. Put such values in the `Change` instead.
- In each case of `apply`, do all checks before the first modification. Then a failed change leaves the table as it was.
- `Validate` runs the same checks without changes. The coordinator calls it before replication.
- Getters (`Job`, `Unit`, `OpenUnits`, `RunningJobs`) return copies. Callers can not change the table by accident. `RunningJobs` returns the IDs of the unfinished jobs, sorted; the leader uses it to finish a job whose last commit failed.
- If you add a change kind, add a test that replays it on two tables and compares the snapshots.

## Test

```bash
go test ./internal/tasktable/
```
