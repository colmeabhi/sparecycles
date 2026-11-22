# scheduler

Decides which unit a volunteer gets. This is the "resource discovery" algorithm, based on VECTrust [4]: trust-based allocation of volunteer resources.

**Owner:** Tejas (review: Abhishek) · **State:** done (`Matcher`) · **Issue:** #8

## How it fits

- Volunteers report their resources when they register: CPU cores, memory and a speed score (`SC_CPU_CORES`, `SC_MEMORY_MB`, `SC_SPEED_SCORE`).
- `RequestWork` in the coordinator builds the candidates: open units that need a volunteer and that this volunteer does not have yet, oldest first.
- `Pick(candidates, volunteer)` returns one unit, or nil for "no work now". With nil, the volunteer asks again after 1 second.
- After each vote, the coordinator tells the scheduler which results were right (`TrustRecorder`), if the scheduler implements it. It does this for the results in the vote (`decide`) and for late copies (`checkLateResult`).
- When gossip declares a volunteer dead, the coordinator calls `Forget` (`Forgetter`), if the scheduler implements it. A dead fast volunteer then no longer makes slow volunteers wait.

## Types and interfaces

```go
type Volunteer struct { ID, Name, Address string; Resources Resources }
type Resources struct { CPUCores, MemoryMB int; SpeedScore float64 }

type Scheduler interface {
    Pick(candidates []tasktable.Unit, v Volunteer) *tasktable.Unit
}

// Optional. The coordinator calls it after each vote.
type TrustRecorder interface {
    RecordResult(volunteerID string, correct bool)
}

// Optional. The coordinator calls it when gossip declares a volunteer dead.
type Forgetter interface {
    Forget(volunteerID string)
}
```

## Implementations

| Type | Behavior |
| --- | --- |
| `Matcher` (`NewMatcher`) | Trust, speed and memory rules below. Implements `TrustRecorder` and `Forgetter`. Used by the coordinator. |
| `FIFO` | Gives the oldest candidate to any volunteer. Kept for tests. |

## Trust

Trust is a beta reputation, as in VECTrust: `(correct + 1) / (correct + wrong + 2)`.

| History | Trust |
| --- | --- |
| New volunteer | 0.50 |
| 1 correct | 0.67 |
| 1 wrong | 0.33 |
| 2 wrong | 0.25 (blocked with the default minimum of 0.3) |
| 10 correct, 1 wrong | 0.85 |

A volunteer is blocked once its trust falls below `SC_TRUST_MIN`. It then gets no units, so its trust cannot change again. The block lasts until the leader changes (trust is leader-local) or the volunteer registers again with a new ID.

## Pick rules, in order

| # | Rule | Effect |
| --- | --- | --- |
| 1 | Memory | `SC_MIN_MEMORY_MB` above 0 and the volunteer reports less: no unit. A volunteer that reports no memory (0) is allowed. |
| 2 | Trust | Trust below `SC_TRUST_MIN`: no unit. |
| 3 | Last copy | Trust below `SC_TRUST_LAST_COPY`: skip units where this volunteer would take the last copy, if another unit is open. The last copy often decides the vote, so it goes to a volunteer with a good history. A new volunteer (0.5) also leaves last copies to others. |
| 4 | Speed | Speed score below `SC_SLOW_SPEED` while a fast volunteer is active: take the **newest** unit, not the oldest, so the oldest jobs finish on fast volunteers. At the end of the work, when the active fast volunteers can take every open copy themselves, the slow volunteer gets nothing. If some copy is one that the fast volunteers already ran (after a vote with no majority), the slow volunteer takes it, so a unit never waits for nobody. |
| 5 | Default | The oldest unit (FIFO). |

A fast volunteer counts as active for one lease length (`SC_LEASE_DURATION`) after its last request, because it may be busy with a unit for that long. It stops counting at once when gossip declares it dead (`Forget`). A fast volunteer with gossip off that dies still counts until the window ends.

**Trade-off of rule 3:** at the start of a run every volunteer is new (0.5), so all of them leave last copies until no other unit needs 2 or more copies. Usually the first 2 copies agree and the last copy is never needed, which saves work. But a unit whose first 2 copies disagree gets its deciding copy later, so it finishes near the end of the job.

## Settings

| Variable | Default | Meaning |
| --- | --- | --- |
| `SC_TRUST_MIN` | `0.3` | Below this trust, no units (0 to 1; 0 turns the rule off) |
| `SC_TRUST_LAST_COPY` | `0.6` | Below this trust, avoid last copies (0 to 1; 0 turns the rule off) |
| `SC_SLOW_SPEED` | `0.5` | A speed score below this is slow (0 turns the rule off) |
| `SC_MIN_MEMORY_MB` | `0` | Minimum reported memory; 0 means no limit |

The coordinator logs these at start (`resource discovery`).

## Metrics and logs

| Metric | Meaning |
| --- | --- |
| `sc_scheduler_picks_total{speed}` | Units picked, by volunteer speed class: `slow` (below `SC_SLOW_SPEED`) or `fast` (not slow, so normal speed counts as `fast`) |
| `sc_scheduler_refusals_total{reason}` | Requests that got no unit on purpose: `low_trust`, `low_memory`, `slow_endgame` |
| `sc_volunteers_blocked_total` | Volunteers blocked for low trust |

Logs: `volunteer blocked: low trust` (info, with the trust and the counts). At debug level: `unit picked` with the reason, `no unit for volunteer` with the reason, `trust updated`, and `last copies left for trusted volunteers`.

## Rules

- `Pick` and `RecordResult` run in many goroutines at the same time. `Matcher` protects its state with a mutex.
- Trust, blocks and the active-volunteer list are leader-local. After a failover, volunteers register again and start at 0.5. `Forget` removes the state of dead volunteers, so it does not grow with churn (volunteers with gossip off are not removed).
- Trust is per volunteer ID. A blocked volunteer that restarts gets a new ID and a new neutral trust. Keying trust by the per-volunteer token (`SC_VOLUNTEER_TOKENS`) would close this, and is a possible next step.
- Speed and memory are reported by the volunteer and are not checked.
- Do not change the `Pick` signature without the team.

## Demo: trust and speed

```bash
make run-coordinator
SC_VOLUNTEER_NAME=fast-1 SC_SPEED_SCORE=2.0 make run-volunteer
SC_VOLUNTEER_NAME=fast-2 SC_SPEED_SCORE=2.0 make run-volunteer
SC_VOLUNTEER_NAME=slow SC_SPEED_SCORE=0.3 make run-volunteer
SC_VOLUNTEER_NAME=faulty SC_FAULTY_RATE=1.0 make run-volunteer
make run-client
make run-client
```

The coordinator logs `volunteer blocked: low trust` after the faulty volunteer's 2nd wrong result. On `:9400/metrics`, `sc_scheduler_picks_total{speed="fast"}` is well above `{speed="slow"}`, and `sc_volunteers_blocked_total` is 1 (test case TC-14). On one machine all volunteers compute at the same real speed, so the difference comes from the rules alone.

## How to test

```bash
go test -race ./internal/scheduler/ ./internal/coordinator/ -run 'Trust|Matcher|Slow|Memory|FIFO|Pick|Last'
```

- `scheduler_test.go`: `Forget` drops a dead volunteer; a Matcher with no metrics works; trust values; a new volunteer gets the oldest unit; trust goes up and down; low trust gets nothing and is counted once; low trust avoids the last copy but takes it when it is the only unit; a slow volunteer takes the newest unit while a fast one is active and the oldest when none is; a slow volunteer waits at the end, but takes copies that fast volunteers cannot; the memory limit; zero rules work like FIFO; concurrent `Pick` and `RecordResult` under `-race`.
- `internal/coordinator/scheduler_test.go`: a faulty volunteer is outvoted in job 1, blocked, and runs no unit of job 2, and both images are correct; when gossip declares a fast volunteer dead, a slow volunteer gets the last copy at once.
