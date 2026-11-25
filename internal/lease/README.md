# lease

Decides how long a volunteer can keep a unit, and which leases have expired. This is the "resource allocation with leases" algorithm.

**Owner:** Tejas · **State:** done (`Timeout`) · **Issue:** #6

## How it fits

- `RequestWork` in the coordinator calls `Duration(volunteer)` and stores `now + duration` in the task table.
- `tick` in the coordinator calls `Expired(openUnits, now)` on the leader every tick (1 second) and commits a `LeaseReleased` change for each result.
- After a release, the unit needs a volunteer again, and the next volunteer that asks gets it. The old volunteer can no longer send a result for it: `ResultRecorded` needs an active lease.
- The volunteer stops its own work at the lease expiry, so it does not keep computing a unit that is gone.

Gossip (`internal/membership`) also frees units: it releases the leases of a volunteer that it marks dead, usually within seconds. The lease is the safety net for what gossip cannot see: a volunteer with gossip off (behind a home router), a volunteer that is alive but stuck, or a unit that takes far too long.

## Interface

```go
type Policy interface {
    Duration(v scheduler.Volunteer) time.Duration
    Expired(units []tasktable.Unit, now time.Time) []Expired
}
```

## Implementations

| Type | Behavior |
| --- | --- |
| `Timeout` | Fixed length for every volunteer. Releases a lease when its expiry is before `now`. Used by the coordinator. |
| `NoExpiry` | Fixed length, but never releases a lease. Kept for tests that must not lose leases. |

`Timeout.Expired`:

- skips units that are not open;
- keeps a lease that expires exactly at `now`, and releases it right after;
- returns the leases sorted by unit ID, then volunteer ID, so logs and tests are the same on every run.

## Settings

| Variable | Default | Meaning |
| --- | --- | --- |
| `SC_LEASE_DURATION` | `3m` | Lease length. Must be positive. |

## Metrics and logs

| Metric | Meaning |
| --- | --- |
| `sc_leases_expired_total` | Leases released because they expired (counted in the coordinator) |
| `sc_leases_dead_volunteer_total` | Leases released because gossip marked the volunteer dead (for comparison) |

The coordinator logs `lease expired` with the unit and the volunteer, and logs `lease` (the length) at start.

## Rules

- A policy keeps no state. Expiry times live in the task table, so a new leader can continue after a failover.
- Design decision: 3 minutes for a 60-second unit. A slow volunteer has 3 times the normal unit time before it loses the unit.
- The expiry is a wall-clock time set by the leader, and the leader compares it with its own wall clock. A clock step (for example an NTP correction) on the leader, or a clock difference between coordinators after a failover, makes the leases that are out shorter or longer by that much.
- The volunteer compares the expiry with its own clock, so a volunteer with a wrong clock stops a little early or late. With 3-minute leases and synced clocks this does not matter.
- After a failover, the new leader does not know the old volunteer IDs. Their leases end through gossip (`member dead`) or, at the latest, through this timeout.

## Demo: a lost unit moves on

Gossip finds a killed volunteer in about 5 seconds and frees its unit before a short lease ends. To show the lease, start the volunteer that you will kill with gossip off:

```bash
SC_LEASE_DURATION=10s SC_VOTE_REPLICAS=1 make run-coordinator
```

```bash
SC_VOLUNTEER_NAME=volunteer-a SC_GOSSIP_ADDR=off make run-volunteer
```

```bash
SC_VOLUNTEER_NAME=volunteer-b make run-volunteer
```

```bash
go run ./cmd/client -params '{"width":4096,"height":4096,"tile_size":256,"max_iter":5000}'
```

While the job runs, kill volunteer A with `kill -9`. About 10 seconds later the coordinator logs `lease expired`, `sc_leases_expired_total` on `:9400/metrics` goes up, and the job still finishes with all units (test case TC-13).

## How to test

```bash
go test -race ./internal/lease/ ./internal/coordinator/ -run 'Lease|Timeout'
```

- `timeout_test.go`: expired, not yet expired, exactly at expiry, done units skipped, several leases on one unit, stable order.
- `internal/coordinator/lease_test.go`: a volunteer takes a unit and goes silent; another volunteer runs the unit, the job finishes, and the late result is refused. A second test uses a fake clock to check that the leader releases a lease only after its expiry.
