# scripts

One-command demos and load tests. They produce the evidence for the report and the presentation (see `docs/grading-checklist.md`, section 2).

**Owner:** Kushagra · **State:** complete; 2 failure demos wait for other issues · **Issue:** #12

## How they work

- Each script builds the binaries, starts a local cluster as plain processes, and stops everything at the end (also on Ctrl+C or an error).
- The processes run in `runs/<script>-<time>/`, so a personal `.env` does not change the results. All logs and images stay in that folder.
- Each script ends with `PASS`, `FAIL` or `SKIP` and the log folder.
- A script that needs another issue checks `.env.example` for that issue's setting and prints `SKIP` until it is merged.
- No Docker is needed. Port 7400 (or 7401 to 7403) must be free.
- **Run load tests from a terminal in the foreground**, with no other heavy work on the machine. macOS can run background tasks on its slower efficiency cores: a background sweep on 2026-11-20 measured about half the throughput of a foreground sweep. Compare only numbers from the same kind of run.

## Scripts

| Script | Make target | Shows | Needs |
| --- | --- | --- | --- |
| `fail-volunteer.sh` | `make fail-volunteer` | `kill -9` a volunteer during a 256-unit job. Gossip marks it dead, its unit moves on, the job finishes. | — |
| `fail-faulty.sh` | `make fail-faulty` | 1 faulty volunteer (`SC_FAULTY_RATE=1.0`) among 3 honest ones. It is outvoted, and the image equals an honest reference. | #7 |
| `fail-leader.sh` | `make fail-leader` | `kill -9` the leader of 3 coordinators during a job. Node 2 becomes leader and the job finishes. | #13, #14 |
| `load.sh` | `make load` | One load test. Appends a row to `results/load.csv`. | — |
| `load-sweep.sh` | `make load-sweep` | Load tests in baseline mode (1, 2 and 4 volunteers) and full mode (2, 3 and 4 volunteers). | — |
| `lib.sh` | — | Shared helpers. Source it; do not run it. | — |

`make evidence` runs the 3 failure demos.

## Load test

```bash
scripts/load.sh VOLUNTEERS JOBS CLIENTS MODE
```

| Mode | Copies per unit | Meaning |
| --- | --- | --- |
| `baseline` | 1 | No fault tolerance. The comparison point for the report. |
| `full` | 3 | Each unit goes to 3 volunteers, and the leader accepts it when 2 agree. Needs 2 or more volunteers: the script stops with an error for 1. |

The default job is a fixed fractal (1024×1024, tile 128, 2000 iterations, 64 units), the same as in every row of `results/load.csv`. Set `PARAMS` to change the job, for example `PARAMS='{"width":512,"height":512,"tile_size":128}' scripts/load.sh 2 4 2 full`. Set `WORKLOAD` for another workload: `WORKLOAD=montecarlo scripts/load.sh 2 4 2 full` uses the Monte Carlo defaults.

Run load tests from a terminal in the foreground. A background run on macOS can measure about half the throughput.

## Leader demo settings

`fail-leader.sh` sets `SC_LEASE_DURATION=20s` on the coordinators. After a failover the volunteers register under new IDs. Gossip usually frees the units of the old IDs in a few seconds; the short lease frees them in 20 s at most if gossip does not.
- If a log line or setting name differs, change the script in the same PR, and update `docs/test-results.md`.
