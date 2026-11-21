# membership

Tracks which nodes are alive. This is the "gossip" algorithm, based on SWIM [2].

**Owner:** Kushagra · **State:** complete (`SWIM`) · **Issue:** #10

## How it fits

- Coordinators and volunteers each run a `SWIM` member and gossip together.
- The coordinator registers it as the `GossipService` gRPC server, on the same port as its other services. The other coordinators (`SC_PEERS`) are the seeds.
- A volunteer runs its own small gRPC server for gossip (`SC_GOSSIP_ADDR`) and sends that address when it registers.
- `Register` in the coordinator calls `Join(member)` for each new volunteer. The leader then pings it, and the volunteer learns the other members from that ping.
- Every second, the leader releases the leases of volunteers that are `Dead`, and forgets them (see `internal/coordinator`).

## Interface

```go
type Membership interface {
    pb.GossipServiceServer       // Ping and PingReq from other members
    Start(ctx context.Context) error
    Join(m Member)
    Alive(id string) bool        // alive or suspect
    Dead(id string) bool         // declared dead; an unknown member is not dead
    Members() []Member
}
```

## Implementations

| Type | Behavior |
| --- | --- |
| `SWIM` | The SWIM failure detector. Used by both binaries. |
| `Static` | Every member that joined is alive forever. Tests use it. |

## How SWIM works here

Each protocol period (`SC_GOSSIP_INTERVAL`):

1. Ping the next member, in a shuffled round-robin order.
2. No valid ack within `SC_GOSSIP_PING_TIMEOUT`: ask `SC_GOSSIP_INDIRECT` other members to ping it (`PingReq`).
3. Still no ack: mark it **suspect**.
4. Suspect for `SC_GOSSIP_SUSPECT_TIMEOUT`: mark it **dead**.

Every message carries the sender's own record and up to 8 recent updates (piggybacking). Each update is sent about 3 × log2(n) times.

| Rule | Why |
| --- | --- |
| alive(i) overrides any state with incarnation < i | A member that refutes or restarts wins over old news |
| suspect(i) overrides alive(j ≤ i) and suspect(j < i) | Suspicion spreads, but not over a newer alive |
| dead(i) overrides alive(j ≤ i) and suspect(j ≤ i) | Death is final for that incarnation |
| A member that hears it is suspect or dead raises its incarnation | It refutes a false suspicion |
| An ack from a different ID at the same address is a failure | A volunteer that registered again is a new member |
| The start incarnation is the Unix time | A restarted node overrules old "dead" news about itself |

## Files

| File | Content |
| --- | --- |
| `membership.go` | `Membership` interface, `Member`, `Static` |
| `swim.go` | The SWIM algorithm |
| `transport.go` | `Transport` interface and the gRPC transport (fast reconnect) |
| `swim_test.go` | Exact tests on an in-memory network with a fake clock |
| `swim_grpc_test.go` | 5 real nodes over gRPC: stop one, all others mark it dead |

## Settings

| Variable | Default | Meaning |
| --- | --- | --- |
| `SC_GOSSIP_INTERVAL` | `1s` | Time between probes |
| `SC_GOSSIP_PING_TIMEOUT` | `400ms` | Wait for a direct ack. Must be less than the interval. |
| `SC_GOSSIP_SUSPECT_TIMEOUT` | `5s` | Time from suspect to dead |
| `SC_GOSSIP_INDIRECT` | `3` | Helpers for indirect pings |
| `SC_ADVERTISE_ADDR` | `127.0.0.1:<listen port>` | Coordinator address that other members use |
| `SC_GOSSIP_ADDR` | `:0` | Volunteer gossip listen address. `off` turns gossip off. |
| `SC_GOSSIP_HOST` | `127.0.0.1` | Volunteer host that others use. `hostname` uses the machine name (Docker). |

## Metrics

`sc_gossip_members{state}`, `sc_gossip_pings_total`, `sc_gossip_ping_failures_total`, `sc_gossip_indirect_probes_total`, `sc_gossip_refutations_total`, `sc_gossip_detection_seconds`. The coordinator also counts `sc_leases_dead_volunteer_total`.

## Demo

Run 1 coordinator and 2 volunteers with fast settings (`SC_GOSSIP_INTERVAL=300ms SC_GOSSIP_PING_TIMEOUT=150ms SC_GOSSIP_SUSPECT_TIMEOUT=2s`) and `SC_VOTE_REPLICAS=1`. Start a large job and `kill -9` one volunteer. The coordinator logs:

```text
member suspect                    member=vol-f771...
member dead                       member=vol-f771... silent_for=2.385s
lease released: volunteer dead    unit=job-6b7c...-106
volunteer removed: dead
job finished                      state=done
```

## Rules

- Act on failures only with `Dead`, never with `!Alive`. After a failover, a new leader does not know old volunteers yet.
- A volunteer behind a home router can not receive pings. Run it with `SC_GOSSIP_ADDR=off`. The lease then handles its failure.
- Write the algorithm yourself. Do not use a gossip library.
