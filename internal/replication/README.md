# replication

Copies task-table changes from the leader to the backups, so that a new leader has the same table.

**Owner:** Abhishek · **State:** done (`Quorum`; `Noop` kept for tests) · **Issue:** #14

## How it fits

- `commit` in the coordinator calls `Send(ctx, index, change)` before it applies a change. If `Send` fails, the change is not applied, and the client gets `Unavailable` and retries.
- The coordinator registers the replicator as the `ReplicationService` gRPC server, so backups receive `Replicate` and `Snapshot` calls.
- Changes come from `internal/tasktable` (`Change.Encode`, `Table.Apply`, `Table.Snapshot`, `Table.Restore`).
- The term comes from the elector (`Leader()`).
- `CatchUp` runs in the election's `BeforeLead` hook (wired in `cmd/coordinator`), before a new leader acts.

## Interface

```go
type Replicator interface {
    pb.ReplicationServiceServer   // Replicate and Snapshot from the leader
    Send(ctx context.Context, index uint64, change []byte) error
}
```

## Implementations

| Type | Behavior |
| --- | --- |
| `Quorum` | Leader: sends each change to all backups in parallel and returns when a majority of coordinators has it (with 3: the leader and 1 backup). Backup: applies changes in index order, and loads the leader's table when it is behind. `CatchUp` gives a new leader the newest table. Used by `cmd/coordinator`. |
| `Noop` | Copies nothing. A new leader starts with an empty table. Kept for tests. |

## How `Quorum` works

```
leader commit(change):  Validate ─► Send(index, change) ─► Apply
                                     │
                    ┌────────────────┴────────────────┐
                    ▼                                 ▼
               backup 1                          backup 2
     index = mine + 1 → apply, ok          (down or slow: the leader
     index ≤ mine, same change → ok         does not wait for it)
     gap or different change → load
       the leader's Snapshot, then apply
```

| Case on the backup | What happens |
| --- | --- |
| `index == local + 1` | Apply, answer ok |
| `index <= local`, same change | A retry. Answer ok |
| Gap, or a different change at the last index | Load the leader's snapshot, then try again |
| First change from a new term | Load the new leader's snapshot first. The old leader may have left changes here that were never confirmed. |
| Older term | Reject, with the newer term |

The leader also keeps a little state per backup:

- One call at a time per backup, so a backup gets the changes in order.
- A backup that did not answer is skipped for one `SC_REPLICATION_TIMEOUT`, so a dead backup does not slow down every change.
- If `Send` fails, a backup that may have applied the dropped change is marked. Before the next index, the leader sends it the confirmed change at that index. The backup sees the difference and loads a snapshot.

### Catch-up of a new leader

Bully picks the highest ID, not the node with the newest table. Before a new leader acts, `CatchUp` asks each reachable coordinator for its snapshot, and loads the one with the highest index if it is newer than its own. A confirmed change is on a majority, and the new leader reached a majority to win, so the newest table holds every confirmed change. The leader logs `leader continues from table index`.

A snapshot is the whole table, and building, sending and loading it takes several times the table size in memory. The table drops unit outputs that are no longer needed (see `internal/tasktable/README.md`, "Size"), so the snapshot stays small under load.

Not replicated, on purpose: the volunteer list and session secrets (volunteers register again after `NotFound`), and job start times for metrics.

## Settings

| Variable | Default | Meaning |
| --- | --- | --- |
| `SC_REPLICATION_TIMEOUT` | `1s` | Time the leader waits for the backups for one change; also the catch-up timeout |

With no `SC_PEERS`, `Send` returns nil at once, so the repo runs on one machine.

## Metrics

| Metric | Meaning |
| --- | --- |
| `sc_replication_seconds` | Leader: time until enough backups have a change |
| `sc_replication_backup_lag{peer}` | Leader: changes each backup is behind, from its last answer |
| `sc_replication_snapshots_served_total` | Snapshots this node sent |
| `sc_replication_snapshots_restored_total` | Snapshots this node loaded |
| `sc_replication_errors_total` | Changes that could not be replicated (counted by the coordinator) |

## Logs

`leader continues from table index` (new leader), `snapshot restored` (with the index and the source node), `backup behind` (with both indexes), `backup has a different change`.

## Failure demo

```bash
make fail-leader
```

It kills the leader with `kill -9` during a 256-unit job. Node 2 logs `became leader` and `leader continues from table index`, and the job finishes. With Docker: `make up`, start a large job, then `docker compose stop coordinator-3`.

## Rules

- Design decision: confirm a change after 1 backup has it (2 of 3 copies).
- Write the algorithm yourself. Do not use a replication or consensus library.

## Test

```bash
go test -race ./internal/replication/ ./internal/coordinator/
```

- `replication_test.go`: in order; a retry is ok; a gap gives `last_index`; a gap loads the snapshot; older term rejected; `Send` needs 1 of 2 backups; `Send` fails with both down; catch-up (index 10 to 12, same bytes); a dropped change is repaired; a new term loads the new leader's table.
- `internal/coordinator/replication_test.go`: 3 coordinators; stop the leader after 20 of 256 units; the job finishes and the new leader accepts only the other 236.
