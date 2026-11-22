# verify

Decides when the results for a unit are good enough to accept. This is the "agreement with majority voting" algorithm, BOINC style [1], [5].

**Owner:** Tejas · **State:** done (`Majority`) · **Issue:** #7

## How it fits

After each recorded result, the coordinator (`decide`) calls `Decide(unit)` and acts on the `Decision`:

| Field | Coordinator action |
| --- | --- |
| `Wrong` | Counts `sc_wrong_results_total` and logs `wrong result` for each volunteer |
| `ExtraCopies` | Commits `CopiesRaised`, counts `sc_votes_no_majority_total`, logs `no majority, more copies` |
| `Accept`, `Output` | Commits `UnitAccepted` with the output |

The coordinator records and votes on one result at a time (`decideMu` in `SubmitResult`), so two results that arrive together never accept a unit twice or raise its copies twice, and no result is recorded without being in a vote.

**Late copies.** A unit is accepted as soon as a quorum agrees, so its last copy often comes back after that. The coordinator still checks that copy against the accepted output (`internal/coordinator/latecopies.go`) and counts it in `sc_wrong_results_total`, with `late=true` in the log, if it differs. Only volunteers that held a lease at the accept are checked, until their lease ends. Without this check, a faulty volunteer that is always last would never be caught.

## Interface

```go
type Verifier interface {
    Decide(unit tasktable.Unit) Decision
}
```

## Implementations

| Type | Behavior |
| --- | --- |
| `Majority{Quorum}` | Accepts the output that `Quorum` results agree on; `Wrong` lists the others. If every copy is back and no output has a quorum, asks for 2 more copies (`ExtraCopiesPerRound`). Otherwise waits. Used by the coordinator. |
| `FirstResult` | Accepts the first result. Kept for tests. |

`Majority.Decide`:

| Unit state (3 copies, quorum 2) | Decision |
| --- | --- |
| 2 results agree (third still running or back) | Accept; `Wrong` = the ones that differ |
| 3 results back, all different | `ExtraCopies: 2`; `Wrong` is empty, because the right output is not known yet |
| Fewer results than copies, no quorum | Wait |
| Copies raised to 5, 3 different results back | Wait for the 2 new copies |

If 2 outputs both reach the quorum (possible with 5 or more copies), the one with more results wins. On a tie, the one that reached that count first wins.

## Settings

| Variable | Default | Meaning |
| --- | --- | --- |
| `SC_VOTE_REPLICAS` | `3` | Copies of each unit (coordinator) |
| `SC_VOTE_QUORUM` | majority of `SC_VOTE_REPLICAS` (2 of 3, 1 of 1) | Equal results needed to accept. Must be 1 to `SC_VOTE_REPLICAS`. |
| `SC_FAULTY_RATE` | `0` | Volunteer: share of units with a wrong output on purpose, for demos and tests |

## Metrics and logs

| Metric | Meaning |
| --- | --- |
| `sc_wrong_results_total` | Results that lost the vote, including late copies |
| `sc_votes_no_majority_total` | Rounds with no majority, after which a unit got 2 more copies |
| `sc_units_accepted_total` | Units accepted |

Logs: `wrong result` (with `late=true` for a late copy), `no majority, more copies`. The coordinator logs `copies` and `quorum` at start.

## Rules

- Design decision: accept when 2 of 3 agree. If no 2 agree, ask 2 more volunteers.
- Compare `Result.Hash`, not the bytes. Workloads are deterministic.
- `Decide` depends only on the unit. It gives the same decision for the same state, and never asks for extra copies twice in one round: after a raise, the unit has fewer results than copies.
- A unit needs `Quorum` different volunteers, because a volunteer never gets the same unit twice. With 3 copies and quorum 2, run at least 2 volunteers, or set `SC_VOTE_REPLICAS=1` for one.
- Voting stops one faulty volunteer, or several that send different wrong outputs. It does not stop faulty volunteers that agree on the same wrong output (collusion). Trust scores (#8) help there.
- There is no limit on extra copies. A unit whose outputs never agree waits until no new volunteer can take it.

## Demo: a faulty volunteer is outvoted

```bash
make fail-faulty
```

It runs 1 coordinator, 3 honest volunteers and 1 volunteer with `SC_FAULTY_RATE=1.0`, and compares the image with an honest-only run (test case TC-03).

## How to test

```bash
go test -race ./internal/verify/ ./internal/coordinator/ -run 'Majority|Faulty|NoMajority|Concurrent|Late'
```

- `majority_test.go`: 2 of 3 accept, accept before the last copy, 3 different give 2 more copies, wait cases, accept after the raise to 5, the next round, same state gives the same decision, largest group wins, quorum 1, empty outputs.
- `internal/coordinator/voting_test.go`: a faulty volunteer is outvoted and the image equals an honest-only image; 2 faulty volunteers cause no majority and 2 more copies; 9 results sent at the same time give 1 accept, no error and 1 wrong result (20 rounds); a late wrong copy is counted, a late correct one and an outsider are not.
