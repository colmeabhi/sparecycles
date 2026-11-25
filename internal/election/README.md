# election

Chooses the leader among the coordinators. This is the "leader election" algorithm, based on Bully [3] with term numbers.

**Owner:** Abhishek · **State:** done (`Bully`; `Static` kept for tests) · **Issue:** #13

## How it fits

- The coordinator registers the elector as the `ElectionService` gRPC server.
- `Run` in the coordinator calls `Start(ctx)`.
- Every work RPC calls `IsLeader()`. A follower answers `FailedPrecondition`, and clients try the next coordinator (`internal/leaderclient`). So a failover needs no client change.
- `Leader()` returns the leader ID and the term. Replication uses the term.
- `BeforeLead` runs after a win and before the node acts. Replication (#14) uses it to load the newest task table.

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
| `Bully` | Bully election with terms and a leader lease. The highest alive ID leads. Used by `cmd/coordinator`. `GRPCTransport` sends its messages on the shared `peerconn.Pool`. |
| `Static` | The highest node ID is always the leader, in term 1. A leader crash stops the system. Kept for tests. |

## How `Bully` works

```
follower ──(no heartbeat for SC_ELECTION_TIMEOUT + random delay)──► candidate, term + 1
candidate: send Election to every higher ID
   ├─ a higher node answers ok ──► wait for its Coordinator message (retry after a timeout)
   └─ nobody answers ──► send Coordinator to all; 2 of 3 accept ──► leader
leader: run BeforeLead, wait for the old lease to end, then act
leader: send Heartbeat every SC_ELECTION_HEARTBEAT; no majority within the lease ──► step down
```

| Message | Receiver does |
| --- | --- |
| `Election` from a lower ID | Answers ok. Starts its own election, unless a leader is alive. |
| `Coordinator` | Accepts the sender as leader of the term if the rules below allow it. |
| `Heartbeat` | Accepts the leader and resets its timer. If the leader has a lower ID, starts an election to take over (Bully). |

Rules that keep one leader, also when the network splits:

1. Each election uses a new term. A message with an older term is rejected: the answer carries the newer term, and the sender steps down.
2. A node accepts one leader per term. A winner needs a majority (2 of 3) to accept it.
3. The leader acts (`IsLeader() == true`) only while a majority answered a heartbeat within the lease (`SC_ELECTION_TIMEOUT - SC_ELECTION_HEARTBEAT`). The lease counts from when the heartbeat was sent.
4. A node does not accept a new leader within one lease of answering another leader. A winner waits one lease after it last answered another leader. Majorities overlap, so the old lease has ended before the new leader acts.
5. A node that hears from a live leader answers `Election` with ok but does not start an election. A node that is cut off cannot disturb a working leader.

With no `SC_PEERS`, the node leads in term 1 before `Start` returns, so the repo runs on one machine.

## Settings

| Variable | Default | Meaning |
| --- | --- | --- |
| `SC_NODE_ID` | `1` | ID of this coordinator |
| `SC_PEERS` | empty | Other coordinators as `id=host:port`, comma separated |
| `SC_ELECTION_HEARTBEAT` | `500ms` | Time between two leader heartbeats; also the timeout of each election RPC |
| `SC_ELECTION_TIMEOUT` | `2s` | A node that hears no leader for this long starts an election. Must be more than 2 heartbeats. |

## Metrics

| Metric | Meaning |
| --- | --- |
| `sc_election_term` | Current term |
| `sc_elections_started_total` | Elections started |
| `sc_elections_won_total` | Elections won (the node then led) |
| `sc_election_stepdowns_total` | Times the node stopped leading |
| `sc_election_failover_seconds` | On the new leader: last contact with the old leader to leading |
| `sc_coordinator_is_leader` | 1 on the leader (set by the coordinator from `IsLeader()`) |

## Logs

Info level, each with the term: `election started`, `became leader`, `new leader` (with the leader ID), `stepped down` (with the reason).

## Failure demo

```bash
make up
```

```bash
docker compose stop coordinator-3
```

The logs show `election started` and `became leader` on coordinator 2 in a higher term, and `make run-client` still works. `make fail-leader` runs the same test without Docker.

## Rules

- Design decision: a leader has a term, and must reach 2 of 3 coordinators to act. Reject messages with an older term.
- Write the algorithm yourself. Do not use an election or consensus library.

## Test

```bash
go test -race ./internal/election/ ./internal/coordinator/
```

- `bully_test.go`: in-memory network. Highest alive ID wins; older term rejected; one leader per term; a cut-off leader steps down; a higher node that comes back takes over in a higher term; `BeforeLead`.
- `grpc_test.go`: 3 nodes over real gRPC. Stop the leader; one new leader; a watch loop checks that two nodes never lead at the same time or in the same term.
- `internal/coordinator/election_test.go`: 3 coordinators; stop the leader; a new job finishes through `leaderclient`.
