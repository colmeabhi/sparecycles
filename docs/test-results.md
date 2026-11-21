# Test results

Test cases for the report (section 10). Each case lists how to run it and the real result.

**Machine:** Apple silicon laptop, 10 CPU cores, arm64, macOS. All nodes run as local processes.

| ID | Test | How to run | Result |
| --- | --- | --- | --- |
| TC-01 | Success scenario: one job end to end | `make run-client` with 1 coordinator and 2 volunteers; `TestEndToEndFractal` | Pass |
| TC-02 | Volunteer crash | `make fail-volunteer` | Pass |
| TC-03 | Faulty volunteer is outvoted | `make fail-faulty` | Not run yet (needs #7) |
| TC-04 | Leader crash | `make fail-leader` | Not run yet (needs #13, #14) |
| TC-05 | Unknown token is rejected | Manual run; `TestUnknownTokenIsRejectedAndCounted` | Pass |
| TC-06 | Wrong signing key | Manual run; `TestVolunteerWithWrongKeyStopsAndSendsNothing` | Pass |
| TC-07 | Throughput grows with volunteers (baseline) | `make load-sweep` | Pass |
| TC-08 | Cost of fault tolerance (full vs baseline) | `make load-sweep` | Pass (as the model predicts) |
| TC-09 | Gossip load does not grow with group size | `TestLoadPerPeriodDoesNotGrowWithGroupSize` | Pass |
| TC-10 | Gossip finds a failed node over a real network | `TestGRPCClusterDetectsFailure` | Pass |

## TC-01 Success scenario

- **Goal:** a client job is split, computed by volunteers and merged.
- **Steps:** start 1 coordinator and 2 volunteers; run the client with a 1024×1024 fractal (16 units).
- **Expected:** the client saves a correct 1024×1024 PNG.
- **Actual (2026-11-20):** `16/16 units done`, `done in 1.002s`, `saved result.png (50587 bytes)`. The image is the Mandelbrot set. The automated test `TestEndToEndFractal` passes on every CI run.

## TC-02 Volunteer crash

- **Goal:** work held by a dead volunteer moves to another volunteer before its lease ends.
- **Steps:** `make fail-volunteer`. 1 coordinator, 2 volunteers, 1 copy per unit, fast gossip (300 ms interval, 2 s suspect timeout). A 256-unit job starts, then volunteer A gets `kill -9`.
- **Expected:** A is suspect, then dead; its unit is released; the job finishes.
- **Actual (2026-11-20):**

  ```text
  18:28:01 member suspect                  member=vol-b907b9010959
  18:28:03 member dead                     silent_for=2.914s
  18:28:03 lease released: volunteer dead  unit=job-0dada023146c-105
  18:28:03 volunteer removed: dead
  18:28:09 job finished                    state=done
  PASS: volunteer-a was marked dead, its unit moved to volunteer-b, and the job finished
  ```

  Without gossip, the unit would wait for the 3-minute lease.

## TC-03 Faulty volunteer

- **Goal:** a volunteer that sends wrong results does not change the final result.
- **Steps:** `make fail-faulty`. 3 honest volunteers and 1 with `SC_FAULTY_RATE=1.0`, 3 copies per unit. The script compares the image with an honest-only reference.
- **Expected:** `wrong result` lines in the log, and identical images.
- **Actual:** not run yet. The script prints `SKIP: needs issue #7`.

## TC-04 Leader crash

- **Goal:** a new leader takes over and finishes the job.
- **Steps:** `make fail-leader`. 3 coordinators, 3 volunteers; `kill -9` the leader (node 3) during a job.
- **Expected:** node 2 logs `became leader`, and the job finishes.
- **Actual:** not run yet. The script prints `SKIP: needs issues #13 (Bully election) and #14 (replication)`.

## TC-05 Unknown token

- **Goal:** only known volunteers can join.
- **Steps:** coordinator with `SC_VOLUNTEER_TOKENS=alice=...,bob=...`; a volunteer with a guessed token.
- **Expected:** registration fails, and the coordinator counts it.
- **Actual (2026-11-20):** volunteer: `volunteer stopped ... Unauthenticated desc = invalid volunteer token`. Coordinator: `registration rejected name=intruder`, `sc_registrations_rejected_total 1`. Known volunteers log their token name (`token=alice`).

## TC-06 Wrong signing key

- **Goal:** a volunteer runs only tasks that the leader signed.
- **Steps:** coordinator with `SC_SIGNING_KEY`; one volunteer with the matching public key, one with a different public key; a 256-unit job.
- **Expected:** the wrong-key volunteer runs nothing and stops; the job still finishes.
- **Actual (2026-11-20):** `bad signature: check SC_SIGNING_PUBLIC_KEY unit=job-92434a3495ec-0`, then the volunteer stopped. Gossip marked it dead 7.4 s later (default timings), the leader released unit 0, and the job finished with all 256 units.

## TC-07 and TC-08 Load tests

- **Goal:** measure throughput and job time as volunteers grow, with and without fault tolerance.
- **Steps:** `make load-sweep`. 8 jobs of 64 units each (1024×1024 fractal, tile 128, 2000 iterations), 2 clients at the same time, 1 coordinator.
  - `baseline`: 1 copy per unit (no fault tolerance).
  - `full`: 3 copies per unit (for majority voting). Until #7, the leader accepts the first result.
- **Actual (2026-11-20, `results/load.csv`):**

  | Mode | Volunteers | Total time | Units/s | Jobs/min | Mean job | p95 job |
  | --- | ---: | ---: | ---: | ---: | ---: | ---: |
  | baseline | 1 | 16.9 s | 30.4 | 28.5 | 3.95 s | 4.59 s |
  | baseline | 2 | 10.0 s | 51.2 | 48.0 | 2.31 s | 2.77 s |
  | baseline | 4 | 5.9 s | 86.1 | 80.8 | 1.40 s | 1.85 s |
  | full | 1 | 17.5 s | 29.2 | 27.4 | 4.13 s | 4.79 s |
  | full | 2 | 16.4 s | 31.3 | 29.3 | 3.84 s | 4.59 s |
  | full | 4 | 13.9 s | 36.8 | 34.5 | 3.30 s | 4.27 s |

- **Analysis:**
  - Baseline throughput grows 2.8× from 1 to 4 volunteers. It is not quite 4×, because 2 clients and one coordinator share the same laptop.
  - In full mode, each unit runs on up to 3 different volunteers, so most of the work is repeated. With N volunteers, the useful throughput is about N / min(N, 3) times one volunteer: 1×, 1×, 1.33× for N = 1, 2, 4. The measurements follow this: 29.2, 31.3, 36.8 units/s.
  - So with 4 volunteers, fault tolerance costs about 57% of the baseline throughput (36.8 vs 86.1 units/s). This is the price of 3 copies per unit, and the report can compare it with BOINC's replication.

## TC-09 Gossip load

- **Goal:** SWIM sends a fixed number of messages per node per period, whatever the group size.
- **Actual:** in a healthy group, a node sends exactly 1 direct ping per period and 0 indirect requests, for 5, 20 and 50 members. `TestLoadPerPeriodDoesNotGrowWithGroupSize` passes on every CI run.

## TC-10 Gossip over a real network

- **Goal:** all nodes find a failed node, and no live node is marked dead.
- **Steps:** 5 SWIM nodes over gRPC on localhost (100 ms interval, 600 ms suspect timeout); stop one.
- **Actual (2026-11-20):** all 4 other nodes marked it dead, and none marked a live node dead. `TestGRPCClusterDetectsFailure` passed 5 runs in a row with the race detector.

## How to add a case

Add a row to the first table and a section with goal, steps, expected and actual (with the date). Paste real output, not expected output.
