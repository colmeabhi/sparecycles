# Test results

Test cases for the report (section 10). Each case lists how to run it and the real result.

**Machine:** Apple silicon laptop, 10 CPU cores, arm64, macOS. All nodes run as local processes, started from a terminal in the foreground.

| ID | Test | How to run | Result |
| --- | --- | --- | --- |
| TC-01 | Success scenario: one job end to end | `make run-client` with 1 coordinator and 2 volunteers; `TestEndToEndFractal` | Pass |
| TC-02 | Volunteer crash | `make fail-volunteer` | Pass |
| TC-03 | Faulty volunteer is outvoted | `make fail-faulty`; `TestFaultyVolunteerIsOutvoted` | Pass |
| TC-04 | Leader crash | `make fail-leader` | Not run yet (needs #13, #14) |
| TC-05 | Unknown token is rejected | Manual run; `TestUnknownTokenIsRejectedAndCounted` | Pass |
| TC-06 | Wrong signing key | Manual run; `TestVolunteerWithWrongKeyStopsAndSendsNothing` | Pass |
| TC-07 | Throughput grows with volunteers (baseline) | `make load-sweep` | Pass |
| TC-08 | Cost of fault tolerance (full vs baseline) | `make load-sweep` | Pass (measured) |
| TC-11 | Performance as clients (users) increase | `scripts/load.sh 4 8 C full` for C = 1, 2, 4, 8 | Pass |
| TC-09 | Gossip load does not grow with group size | `TestLoadPerPeriodDoesNotGrowWithGroupSize` | Pass |
| TC-10 | Gossip finds a failed node over a real network | `TestGRPCClusterDetectsFailure` | Pass |
| TC-12 | Docker Compose cluster: job, gossip, Prometheus, a killed container | `make up`, `make run-client`, `docker kill` | Pass |
| TC-14 | Trust blocks a faulty volunteer; slow volunteers get less work | Manual run in `internal/scheduler/README.md`; `TestFaultyVolunteerLosesTrustAndGetsNoWork` | Pass |

## TC-01 Success scenario

- **Goal:** a client job is split, computed by volunteers and merged.
- **Steps:** start 1 coordinator and 2 volunteers; run the client with a 1024×1024 fractal (16 units).
- **Expected:** the client saves a correct 1024×1024 PNG.
- **Actual (2026-11-20):** `16/16 units done`, `done in 1.002s`, `saved result.png (50587 bytes)`. The image is the Mandelbrot set. The automated test `TestEndToEndFractal` passes on every CI run.

## TC-02 Volunteer crash

- **Goal:** work held by a dead volunteer moves to another volunteer before its lease ends.
- **Steps:** `make fail-volunteer`. 1 coordinator, 2 volunteers, 1 copy per unit, fast gossip (300 ms interval, 2 s suspect timeout). A 256-unit job starts, then volunteer A gets `kill -9`.
- **Expected:** A is suspect, then dead; its unit is released; the job finishes.
- **Actual (2026-11-21):** the run in the report (Fig. 4, `docs/report/screenshots/s2-volunteer-crash.png`):

  ```text
  18:46:59 member suspect                  member=vol-30f3c3c0baac
  18:47:01 member dead                     silent_for=2.886s
  18:47:01 lease released: volunteer dead  unit=job-9e089c50a83b-116
  18:47:01 volunteer removed: dead
  18:47:02 job finished                    state=done
  PASS: volunteer-a was marked dead, its unit moved to volunteer-b, and the job finished
  ```

  An earlier run on 2026-11-20 gave the same result (dead after 2.914 s, unit 105 released). Without gossip, the unit would wait for the 3-minute lease.

## TC-03 Faulty volunteer

- **Goal:** a volunteer that sends wrong results does not change the final result.
- **Steps:** `make fail-faulty`. 3 honest volunteers and 1 with `SC_FAULTY_RATE=1.0`, 3 copies per unit. The script compares the image with an honest-only reference.
- **Expected:** `wrong result` lines in the log, and identical images.
- **Actual (2026-11-22):** `PASS: the faulty volunteer was outvoted and the image is correct`. The coordinator started with `copies=3 quorum=2`. The faulty volunteer sent 14 results, and all 14 were caught: 11 lost the vote, and 3 came back after the 2 honest copies had already won and were caught as late copies. The two images are identical (`cmp` finds no difference, 42258 bytes each).

  ```text
  13:37:05 wrong result  unit=job-786948cf8d3b-38 volunteer=vol-21a5f9a9e1ac
  13:37:05 wrong result  unit=job-786948cf8d3b-44 volunteer=vol-21a5f9a9e1ac
  13:37:05 job finished  job=job-786948cf8d3b state=done
  13:37:05 wrong result  unit=job-786948cf8d3b-45 volunteer=vol-21a5f9a9e1ac late=true
  ```

  A first run on the same day, before late copies were checked, failed with `no result was outvoted`: the faulty volunteer joined last, so it always ran the third copy, and the 2 honest copies won before it finished. Its 5 results were turned away and never compared. The automated tests `TestFaultyVolunteerIsOutvoted`, `TestNoMajorityAsksForTwoMoreCopies` and `TestLateCopyIsChecked` cover both paths.

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
- **Steps:** `make load-sweep`, from a terminal in the foreground. 8 jobs of 64 units each (1024×1024 fractal, tile 128, 2000 iterations), 2 clients at the same time, 1 coordinator.
  - `baseline`: 1 copy per unit (no fault tolerance).
  - `full`: 3 copies per unit (for majority voting). Until #7, the leader accepts the first result.
- **Actual (2026-11-21, `results/load.csv`):**

  | Mode | Volunteers | Total time | Units/s | Jobs/min | Mean job | p95 job |
  | --- | ---: | ---: | ---: | ---: | ---: | ---: |
  | baseline | 1 | 7.4 s | 69.1 | 64.8 | 1.75 s | 2.55 s |
  | baseline | 2 | 4.7 s | 109.6 | 102.8 | 1.12 s | 2.14 s |
  | baseline | 4 | 3.7 s | 139.6 | 130.8 | 0.89 s | 1.64 s |
  | full | 1 | 7.5 s | 68.2 | 63.9 | 1.76 s | 2.45 s |
  | full | 2 | 7.4 s | 69.2 | 64.9 | 1.75 s | 2.55 s |
  | full | 4 | 6.9 s | 74.1 | 69.5 | 1.63 s | 2.15 s |

- **Analysis:**
  - Baseline throughput grows 1.6× with 2 volunteers and 2.0× with 4. In this test job one unit takes only about 15 ms, instead of the planned 60 s. So the per-unit messages to the coordinator (request, result, table change) are a large part of the cost, and adding volunteers helps less. With real 60-second units, this overhead would be negligible.
  - In full mode, each unit runs on up to 3 different volunteers. With 1 or 2 volunteers, full mode equals one volunteer's throughput (68.2 and 69.2 units/s), as the model N / min(N, 3) predicts.
  - With 4 volunteers, full mode reaches 74.1 units/s: 53% of the baseline, so fault tolerance costs 47% here. The model predicts one third for N ≥ 3; the measured share is higher because the baseline itself scales sublinearly in this test.
  - A repeat of the baseline run with 1 volunteer gave 69.0 units/s, against 69.1, so the runs are stable.
  - **Note:** an earlier sweep (2026-11-20) gave about half of these numbers. It ran as a background task, which macOS can schedule on its slower efficiency cores. Its rows were removed. Run load tests from a terminal in the foreground.

## TC-11 Performance as clients increase

- **Goal:** show how throughput and job time change as more users submit jobs at the same time. The course demo asks for this.
- **Steps:** `scripts/load.sh 4 8 C full` for C = 1, 2, 4 and 8 clients. 4 volunteers, full mode, 8 jobs of 64 units.
- **Actual (2026-11-21, `results/load.csv`):**

  | Clients | Total time | Units/s | Jobs/min | Mean job | p95 job |
  | ---: | ---: | ---: | ---: | ---: | ---: |
  | 1 | 12.1 s | 42.5 | 39.8 | 1.51 s | 2.03 s |
  | 2 | 7.0 s | 73.1 | 68.5 | 1.64 s | 2.25 s |
  | 4 | 6.6 s | 77.4 | 72.6 | 2.75 s | 3.57 s |
  | 8 | 7.5 s | 68.1 | 63.9 | 4.66 s | 7.52 s |

- **Analysis:**
  - With 1 client, the volunteers wait between jobs (submit, split, poll), so throughput is low.
  - With 2 clients, the volunteers are busy, and throughput rises to 73 units/s. More clients add almost nothing: the volunteers are the limit (about 75 units/s in full mode).
  - Job time grows with the number of clients, because more jobs share the same volunteers: the mean is 1.5 s with 1 client and 4.7 s with 8. This is the expected queueing behavior.
  - With 8 clients, throughput drops a little (68 units/s), because more jobs and more polling compete for the single coordinator.

## TC-09 Gossip load

- **Goal:** SWIM sends a fixed number of messages per node per period, whatever the group size.
- **Actual:** in a healthy group, a node sends exactly 1 direct ping per period and 0 indirect requests, for 5, 20 and 50 members. `TestLoadPerPeriodDoesNotGrowWithGroupSize` passes on every CI run.

## TC-10 Gossip over a real network

- **Goal:** all nodes find a failed node, and no live node is marked dead.
- **Steps:** 5 SWIM nodes over gRPC on localhost (100 ms interval, 600 ms suspect timeout); stop one.
- **Actual (2026-11-20):** all 4 other nodes marked it dead, and none marked a live node dead. `TestGRPCClusterDetectsFailure` passed 5 runs in a row with the race detector.

## TC-12 Docker Compose cluster

- **Goal:** the local cluster in containers runs a job, gossips across the Docker network, and Prometheus scrapes the coordinator.
- **Steps:** `make up` (1 coordinator, 3 volunteers, Prometheus; default gossip settings: 1 s interval, 5 s suspect timeout). `make run-client`. Then `docker kill sparecycles-volunteer-1` and `make run-client` again. `make down`.
- **Expected:** the job finishes on 3 volunteers; gossip marks the killed volunteer dead; the second job still finishes.
- **Actual (2026-11-21):**
  - All 3 volunteers registered and joined gossip with their container host names. The first job (16 units, 3 copies) finished in 1.0 s; all 3 volunteers sent results.
  - Prometheus at `localhost:9090` reported `up{job="coordinator"} = 1`. The coordinator metrics showed `sc_gossip_members{state="alive"} 3`, 18 pings and 0 ping failures.
  - After the kill (times in UTC):

    ```text
    03:08:19 member suspect                  member=vol-6c89bf47adbb
    03:08:21 job finished                    job=job-7013d3ae1e90 state=done
    03:08:25 member dead                     member=vol-6c89bf47adbb silent_for=6.771s
    03:08:26 volunteer removed: dead         volunteer=vol-6c89bf47adbb
    ```

    The second job finished on the 2 other volunteers in 2.0 s. Detection takes longer than in TC-02 (6.8 s against 2.9 s) because TC-02 uses fast demo gossip settings.

## TC-14 Trust and speed in resource discovery

- **Goal:** a volunteer that keeps sending wrong results is blocked and gets no more units; slow volunteers get fewer units while fast ones are active.
- **Steps:** 1 coordinator with the default rules (`min_trust=0.3 last_copy_trust=0.6 slow_speed=0.5`). 4 volunteers: `fast-1` and `fast-2` with `SC_SPEED_SCORE=2.0`, `slow` with `SC_SPEED_SCORE=0.3`, and `faulty` with `SC_FAULTY_RATE=1.0`. Two 1024×1024 jobs of 64 units, one after the other. The steps are in `internal/scheduler/README.md`.
- **Expected:** `volunteer blocked: low trust` for the faulty volunteer after its 2nd wrong result; it runs no more units; fast volunteers run more units than the slow one; both images are correct.
- **Actual (2026-11-22):**

  ```text
  13:50:29 wrong result                  unit=job-f0fe1a9f933a-1 volunteer=vol-857c08921db6
  13:50:29 wrong result                  unit=job-f0fe1a9f933a-2 volunteer=vol-857c08921db6
  13:50:29 volunteer blocked: low trust  volunteer=vol-857c08921db6 trust=0.25 correct=0 wrong=2
  ```

  | Volunteer | Units run |
  | --- | ---: |
  | fast-1 (speed 2.0) | 126 |
  | fast-2 (speed 2.0) | 123 |
  | slow (speed 0.3) | 69 |
  | faulty | 4, all caught as wrong |

  The faulty volunteer took 4 first copies before any vote, then was blocked and got nothing for the rest of the 2 jobs. The metrics showed `sc_scheduler_picks_total{speed="fast"} 253`, `{speed="slow"} 69`, `sc_volunteers_blocked_total 1` and `sc_wrong_results_total 4`. Both job images are identical. All volunteers compute at the same real speed on one machine, so the gap between fast and slow comes from the pick rules alone.

## How to add a case

Add a row to the first table and a section with goal, steps, expected and actual (with the date). Paste real output, not expected output.
