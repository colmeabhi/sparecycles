# election

Chooses the leader among the coordinators. This is the "leader election" algorithm, based on Bully [3] with term numbers.

**Owner:** Abhishek · **State:** stub (`Static`) · **Issue:** #13

## How it fits

- The coordinator registers the elector as the `ElectionService` gRPC server.
- `Run` in the coordinator calls `Start(ctx)`.
- Every work RPC calls `IsLeader()`. A follower answers `FailedPrecondition`, and clients try the next coordinator.
- `Leader()` returns the leader ID and the term. Replication uses the term.

## Interface

```go
type Elector interface {
    pb.ElectionServiceServer   // Election, Coordinator, Heartbeat
    Start(ctx context.Context) error
    IsLeader() bool
    Leader() (id int, term uint64)
}
```

## Implementations

| Type | Behavior |
| --- | --- |
| `Static` | The highest node ID is always the leader, in term 1. A leader crash stops the system. Base stub. |

## Settings

| Variable | Default | Meaning |
| --- | --- | --- |
| `SC_NODE_ID` | `1` | ID of this coordinator |
| `SC_PEERS` | empty | Other coordinators as `id=host:port`, comma separated |

## Rules

- Design decision: a leader has a term, and must reach 2 of 3 coordinators to act. Reject messages with an older term.
- With no peers, a single node must become the leader at once, so that the repo runs on one machine.
- Write the algorithm yourself. Do not use an election or consensus library.
