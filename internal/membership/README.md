# membership

Tracks which nodes are alive. This is the "gossip" algorithm, based on SWIM [2].

**Owner:** Kushagra · **State:** stub (`Static`) · **Issue:** #10

## How it fits

- The coordinator registers the membership as the `GossipService` gRPC server.
- `Run` in the coordinator calls `Start(ctx)`.
- `Register` in the coordinator calls `Join(member)` for each new volunteer, with its gossip address.

## Interface

```go
type Membership interface {
    pb.GossipServiceServer       // Ping and PingReq from other members
    Start(ctx context.Context) error
    Join(m Member)
    Alive(id string) bool
    Members() []Member
}
```

## Implementations

| Type | Behavior |
| --- | --- |
| `Static` | Every member that joined is alive forever. Base stub. |

## Rules

- Design decision: volunteers and coordinators gossip together.
- Never treat an unknown member as dead. After a failover, a new leader does not know the old volunteers yet.
- Write the algorithm yourself. Do not use a gossip library.
