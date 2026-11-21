# lease

Decides how long a volunteer can keep a unit, and which leases have expired. This is the "resource allocation with leases" algorithm.

**Owner:** Tejas · **State:** stub (`NoExpiry`) · **Issue:** #6

## How it fits

- `RequestWork` in the coordinator calls `Duration(volunteer)` and stores `now + duration` in the task table.
- `tick` in the coordinator calls `Expired(openUnits, now)` every second and releases each returned lease.
- After a release, the unit needs a volunteer again, and the next volunteer gets it.

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
| `NoExpiry` | Fixed length, but never releases a lease. Base stub. |

## Rules

- A policy keeps no state. Expiry times live in the task table, so a new leader can continue after a failover.
- Design decision: 3 minutes for a 60-second unit.

## Settings

| Variable | Default | Meaning |
| --- | --- | --- |
| `SC_LEASE_DURATION` | `3m` | Lease length |

## Metrics

`sc_leases_expired_total` (counted in the coordinator).
