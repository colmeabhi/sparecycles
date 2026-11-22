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
| `load-sweep.sh` | `make load-sweep` | Load tests for 1, 2 and 4 volunteers, in baseline and full mode. | — |
| `lib.sh` | — | Shared helpers. Source it; do not run it. | — |

`make evidence` runs the 3 failure demos.

## Load test

```bash
scripts/load.sh VOLUNTEERS JOBS CLIENTS MODE
```

| Mode | Copies per unit | Meaning |
| --- | --- | --- |
| `baseline` | 1 | No fault tolerance. The comparison point for the report. |
| `full` | 3 | Each unit runs on 3 volunteers, for majority voting. |

Set `PARAMS` to change the job, for example `PARAMS='{"width":512,"height":512,"tile_size":128}' scripts/load.sh 2 4 2 full`.

## After an issue merges

- After #7: run `make fail-faulty`. It uses `SC_FAULTY_RATE` and expects the log line `wrong result`.
- After #13 and #14: run `make fail-leader`. It expects `SC_ELECTION_TIMEOUT` in `.env.example` and the log line `became leader`.
- If a log line or setting name differs, change the script in the same PR, and update `docs/test-results.md`.
