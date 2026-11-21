# scheduler

Decides which unit a volunteer gets. This is the "resource discovery" algorithm, based on VECTrust [4].

**Owner:** Tejas (review: Abhishek) · **State:** stub (`FIFO`) · **Issue:** #8

## How it fits

- Volunteers report their resources when they register: CPU cores, memory and a speed score.
- `RequestWork` in the coordinator builds the candidates: open units that need a volunteer and that this volunteer does not have yet, oldest first.
- `Pick(candidates, volunteer)` returns one unit, or nil for "no work now". With nil, the volunteer asks again after 1 second.

## Types and interface

```go
type Volunteer struct { ID, Name, Address string; Resources Resources }
type Resources struct { CPUCores, MemoryMB int; SpeedScore float64 }

type Scheduler interface {
    Pick(candidates []tasktable.Unit, v Volunteer) *tasktable.Unit
}
```

## Implementations

| Type | Behavior |
| --- | --- |
| `FIFO` | Gives the oldest candidate to any volunteer. Base stub. |

## Rules

- `Pick` runs in many goroutines at the same time. Protect any state with a mutex.
- The volunteer list is leader-local. After a failover, volunteers register again.
- Do not change the `Pick` signature without the team.
