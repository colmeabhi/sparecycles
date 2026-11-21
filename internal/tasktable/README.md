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
| `table_test.go` | Life cycle, lease release, replay and snapshot tests |

## Change kinds

| Kind | Effect |
| --- | --- |
| `JobAdded` | Adds a job and its units, each with `Copies` volunteers to run it |
| `UnitAssigned` | Gives a lease on a unit to a volunteer |
| `LeaseReleased` | Removes a lease without a result (for example, it expired) |
| `ResultRecorded` | Stores a result and removes the lease |
| `CopiesRaised` | Asks more volunteers to run a unit (no majority) |
| `UnitAccepted` | Marks a unit as done with its accepted output |
| `JobFinished` | Stores the merged result or an error |

## Rules

- `Apply` must be deterministic. Do not read the clock, random numbers or maps in random order inside `Apply`. Put such values in the `Change` instead.
- In each case of `apply`, do all checks before the first modification. Then a failed change leaves the table as it was.
- `Validate` runs the same checks without changes. The coordinator calls it before replication.
- Getters (`Job`, `Unit`, `OpenUnits`) return copies. Callers can not change the table by accident.
- If you add a change kind, add a test that replays it on two tables and compares the snapshots.

## Test

```bash
go test ./internal/tasktable/
```
