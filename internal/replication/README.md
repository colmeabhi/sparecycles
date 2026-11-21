# replication

Copies task-table changes from the leader to the backups, so that a new leader has the same table.

**Owner:** Abhishek · **State:** stub (`Noop`) · **Issue:** #14

## How it fits

- `commit` in the coordinator calls `Send(ctx, index, change)` before it applies a change. If `Send` fails, the change is not applied.
- The coordinator registers the replicator as the `ReplicationService` gRPC server, so backups receive `Replicate` and `Snapshot` calls.
- Changes come from `internal/tasktable` (`Change.Encode`, `Table.Apply`, `Table.Snapshot`, `Table.Restore`).

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
| `Noop` | Copies nothing. A new leader starts with an empty table. Base stub. |

## Rules

- Design decision: confirm a change after 1 backup has it (2 of 3 copies).
- Backups apply changes in index order. A gap means the backup is behind and needs a snapshot.
- Bully picks the highest ID, not the newest table. A new leader must get the newest table before it acts.
- Write the algorithm yourself. Do not use a replication or consensus library.
