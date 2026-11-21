# verify

Decides when the results for a unit are good enough to accept. This is the "agreement with majority voting" algorithm.

**Owner:** Tejas · **State:** stub (`FirstResult`) · **Issue:** #7

## How it fits

After each recorded result, the coordinator (`decide`) calls `Decide(unit)` and acts on the `Decision`:

| Field | Coordinator action |
| --- | --- |
| `Wrong` | Counts `sc_wrong_results_total` and logs `wrong result` for each volunteer |
| `ExtraCopies` | Commits `CopiesRaised`, so more volunteers run the unit |
| `Accept`, `Output` | Commits `UnitAccepted` with the output |

## Interface

```go
type Verifier interface {
    Decide(unit tasktable.Unit) Decision
}
```

## Implementations

| Type | Behavior |
| --- | --- |
| `FirstResult` | Accepts the first result. Base stub. |

## Rules

- Design decision: accept when 2 of 3 agree. If no 2 agree, ask 2 more volunteers.
- Compare `Result.Hash`, not the bytes. Workloads are deterministic.
- `Decide` runs after every result. It must give the same decision for the same unit state, and must not ask for extra copies twice in one round.
