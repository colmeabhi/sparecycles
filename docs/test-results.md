# Test results

Test cases for the report (section 10). Each case lists how to run it and the real result.

**Machine:** Apple silicon laptop, 10 CPU cores, arm64, macOS. All nodes run as local processes, started from a terminal in the foreground.

| ID | Test | How to run | Result |
| --- | --- | --- | --- |
| TC-01 | Success scenario: one job end to end | `make run-client` with 1 coordinator and 2 volunteers; `TestEndToEndFractal` | Pass |
| TC-02 | Volunteer crash | `make fail-volunteer` | Pass |
| TC-03 | Faulty volunteer is outvoted | `make fail-faulty`; `TestFaultyVolunteerIsOutvoted` | Pass |
| TC-04 | Leader crash | `make fail-leader`; `TestLeaderCrashJobContinues` | Pass |
| TC-05 | Unknown token is rejected | Manual run; `TestUnknownTokenIsRejectedAndCounted` | Pass |
| TC-06 | Wrong signing key | Manual run; `TestVolunteerWithWrongKeyStopsAndSendsNothing` | Pass |
| TC-07 | Throughput grows with volunteers (baseline) | `make load-sweep` | Pass |
| TC-08 | Cost of fault tolerance (full vs baseline) | `make load-sweep` | Pass (measured) |
| TC-11 | Performance as clients (users) increase | `scripts/load.sh 4 8 C full` for C = 1, 2, 4, 8 | Pass |
| TC-09 | Gossip load does not grow with group size | `TestLoadPerPeriodDoesNotGrowWithGroupSize` | Pass |
| TC-10 | Gossip finds a failed node over a real network | `TestGRPCClusterDetectsFailure` | Pass |
| TC-12 | Docker Compose cluster: job, gossip, Prometheus, a killed container | `make up`, `make run-client`, `docker kill` | Pass |
| TC-13 | Lease expiry moves a lost unit (gossip off) | Manual run in `internal/lease/README.md`; `TestExpiredLeaseMovesUnitToAnotherVolunteer` | Pass |
| TC-14 | Trust blocks a faulty volunteer; slow volunteers get less work | Manual run in `internal/scheduler/README.md`; `TestFaultyVolunteerLosesTrustAndGetsNoWork` | Pass |
| TC-15 | Second workload: Monte Carlo estimate of pi, the same bytes on arm64 and amd64 | Client command in `docs/setup.md`; `TestEndToEndMonteCarlo`, `TestMonteCarloGoldenCounts` | Pass |
| TC-16 | Requests under another volunteer's ID are rejected | `TestRequestsUnderAnotherVolunteersIDAreRejected` | Pass |

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

  A run on 2026-11-25, on `main` with resource discovery, passed too. The faulty volunteer sent 8 wrong results, and all 8 were caught. After its 2nd wrong result the coordinator logged `volunteer blocked: low trust trust=0.25 correct=0 wrong=2`. The other 6 were units that it had already taken before the block. Both images were identical (42258 bytes).

  A first run on 2026-11-22, before late copies were checked, failed with `no result was outvoted`: the faulty volunteer joined last, so it always ran the third copy, and the 2 honest copies won before it finished. Its 5 results were turned away and never compared. The automated tests `TestFaultyVolunteerIsOutvoted`, `TestNoMajorityAsksForTwoMoreCopies` and `TestLateCopyIsChecked` cover both paths.

## TC-04 Leader crash

- **Goal:** a new leader takes over and finishes the job.
- **Steps:** `make fail-leader`. 3 coordinators, 3 volunteers; `kill -9` the leader (node 3) during a job.
- **Expected:** node 2 logs `became leader`, and the job finishes.
- **Actual (2026-11-25, branch of #14):** `PASS: node 2 took over after the leader crash, and the job finished`. Node 2 logged `election started term=3`, `leader continues from table index index=58` and `became leader term=3`, about 2.5 s after the last heartbeat of node 3. Node 1 logged `snapshot restored from=2 index=58`. The 256-unit job finished in 12 s.
- **Actual (2026-11-25, `main` with all algorithms, `SC_LEASE_DURATION=20s`):** `PASS` again. The script killed node 3 at 21:11:09.5, right after volunteer-1 sent its 5th result. Node 2 logged:

  ```text
  21:11:12.025 election started                  term=3
  21:11:12.026 member dead                       member=coord-3 silent_for=3.293s
  21:11:12.040 leader continues from table index index=296 term=3
  21:11:12.040 became leader                     term=3
  21:11:12.289 volunteer registered              name=volunteer-2
  21:11:16.525 lease released: volunteer dead    unit=job-8497fb4ac85d-57
  21:11:18.301 job finished                      state=done
  ```

  The new leader took over 2.5 s after the kill. The 3 volunteers registered again within 1.3 s, under new IDs. Gossip marked the old IDs dead, and the leader released their leases 4.5 s after the takeover, before the 20 s lease timeout. The client saw 3 failed progress checks during the election, then finished the 256-unit job in 10.0 s.
- **Unit test:** `TestLeaderCrashJobContinues` stops the leader after 20 of 256 units. The new leader accepts exactly the other 236, so no accepted unit runs again.

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
  - `full`: 3 copies per unit, and the leader accepts a unit when 2 copies agree (majority voting, #7). Full mode starts at 2 volunteers, because 1 volunteer never gets 2 copies of the same unit.
- **Actual (2026-11-25, `main` after #31 to #38, `results/load.csv`):**

  | Mode | Volunteers | Total time | Units/s | Jobs/min | Mean job | p95 job | Repeat units/s |
  | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
  | baseline | 1 | 7.1 s | 72.0 | 67.5 | 1.68 s | 2.25 s | 68.1 |
  | baseline | 2 | 5.6 s | 91.6 | 85.9 | 1.30 s | 1.74 s | 84.1 |
  | baseline | 4 | 4.6 s | 112.0 | 105.0 | 1.09 s | 2.05 s | 104.9 |
  | full | 2 | 7.4 s | 69.1 | 64.8 | 1.75 s | 2.55 s | 67.3 |
  | full | 3 | 7.5 s | 68.1 | 63.9 | 1.78 s | 2.56 s | 69.1 |
  | full | 4 | 6.7 s | 76.4 | 71.6 | 1.59 s | 2.15 s | 71.1 |

  The "Repeat" column is a second full sweep run right after the first one. The two sweeps differ by less than 10%.

- **Results per unit (from the volunteer logs of the repeat sweep):** with 2 volunteers, each volunteer sent 512 results, so every unit ran exactly 2 times. With 3 volunteers, the volunteers sent 1128 results (2.2 per unit); with 4, they sent 1331 (2.6 per unit). The leader accepts a unit at 2 equal results, but a third copy that is already running still finishes, and the leader checks it as a late copy.
- **Analysis:**
  - Baseline throughput grows 1.3× with 2 volunteers and 1.6× with 4. In this test job one unit takes only about 15 ms, instead of the planned 60 s. So the per-unit messages to the coordinator (request, result, table change) are a large part of the cost, and adding volunteers helps less. With real 60-second units, this overhead would be negligible.
  - In full mode, every unit runs at least 2 times. With 2 or 3 volunteers, full mode equals one volunteer's throughput (69.1 and 68.1 units/s).
  - With 4 volunteers, full mode reaches 76.4 units/s: 68% of the baseline, so fault tolerance costs 32% here. The repeat sweep gives the same share (71.1 against 104.9 units/s, 68%).
  - **Compared with the sweep of 2026-11-21** (before #6 to #9 merged): that full mode accepted the first result, and fault tolerance cost 47% at 4 volunteers. Do not compare 47% with 32% directly. Full mode with 4 volunteers changed only a little (74.1 then, 76.4 now), but the baseline then reached 139.6 units/s, against 112.0 now. We did not isolate the cause. The coordinator now runs resource discovery and trust bookkeeping on every request, and with 15 ms units the per-request work is a large part of the cost. The older rows stay in `results/load.csv` with their dates.
  - **Note:** an even earlier sweep (2026-11-20) gave about half of the 2026-11-21 numbers. It ran as a background task, which macOS can schedule on its slower efficiency cores. Its rows were removed. Run load tests from a terminal in the foreground.

## TC-11 Performance as clients increase

- **Goal:** show how throughput and job time change as more users submit jobs at the same time. The course demo asks for this.
- **Steps:** `scripts/load.sh 4 8 C full` for C = 1, 2, 4 and 8 clients. 4 volunteers, full mode, 8 jobs of 64 units.
- **Actual (2026-11-25, `main` with majority voting, `results/load.csv`):**

  | Clients | Total time | Units/s | Jobs/min | Mean job | p95 job |
  | ---: | ---: | ---: | ---: | ---: | ---: |
  | 1 | 12.9 s | 39.7 | 37.3 | 1.61 s | 1.82 s |
  | 2 | 7.2 s | 71.1 | 66.7 | 1.69 s | 2.55 s |
  | 4 | 6.8 s | 75.1 | 70.4 | 2.93 s | 4.08 s |
  | 8 | 7.3 s | 69.9 | 65.5 | 4.53 s | 7.33 s |

- **Analysis:**
  - With 1 client, the volunteers wait between jobs (submit, split, poll), so throughput is low.
  - With 2 clients, the volunteers are busy, and throughput rises to 71 units/s. More clients add almost nothing: the volunteers are the limit (about 75 units/s in full mode).
  - Job time grows with the number of clients, because more jobs share the same volunteers: the mean is 1.6 s with 1 client and 4.5 s with 8. This is the expected queueing behavior.
  - With 8 clients, throughput drops a little (70 units/s), because more jobs and more polling compete for the single coordinator.
  - The sweep of 2026-11-21 (first result accepted) gave the same shape: 42.5, 73.1, 77.4 and 68.1 units/s.

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

## TC-13 Lease expiry without gossip

- **Goal:** a unit held by a volunteer that gossip cannot see moves to another volunteer when its lease ends.
- **Steps:** 1 coordinator with `SC_LEASE_DURATION=10s` and 1 copy per unit; volunteer A with `SC_GOSSIP_ADDR=off`, volunteer B with gossip on. Start a 4096×4096 job (256 units), then `kill -9` A after it sent 5 results. The steps are in `internal/lease/README.md`.
- **Expected:** about 10 s after the kill, `lease expired` for A's unit; the job finishes with all 256 units.
- **Actual (2026-11-22):**

  ```text
  13:11:44 kill -9 volunteer-a
  13:11:55 lease expired  unit=job-19002f44ec14-57 volunteer=vol-353c32b28c54
  13:12:18 job finished   job=job-19002f44ec14 state=done
  ```

  The client saved the full image (`done in 34.007s`). The metrics showed `sc_leases_expired_total 1` and `sc_leases_dead_volunteer_total 0`: the lease, not gossip, freed the unit. The automated test `TestExpiredLeaseMovesUnitToAnotherVolunteer` checks the same with a silent volunteer and a 300 ms lease. `TestLeaseIsReleasedOnlyAfterItsExpiry` checks, with a fake clock, that the lease is released only after its expiry, and that a volunteer whose lease expired cannot send a result while the unit is still open.

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

## TC-15 Monte Carlo workload

- **Goal:** the second workload runs through the cluster, gives a correct estimate of pi, and gives the same bytes on every CPU, so that votes can compare hashes.
- **Steps:** 1 coordinator and 2 volunteers on one machine. Run the client command from `docs/setup.md` (8 units of 1,000,000 samples), then the client with no `-params` and no `-out` (the default size, 16 units of 50,000,000 samples, saved to `result.json`). Run the golden test on arm64 and on amd64 (`GOARCH=amd64 go test -run MonteCarlo ./internal/workload/`, with Rosetta on Apple silicon).
- **Expected:** both jobs finish with an estimate near 3.1416. The golden counts are the same on both CPUs.
- **Actual (2026-11-24):** the client printed:

  ```text
  submitted job-7d75ab6cbafe with 8 units
  8/8 units done
  done in 1.003s
  saved pi.json (65 bytes)
  ```

  `pi.json` holds `{"estimate":3.1419955,"inside":6283991,"total":8000000,"units":8}`. The default run printed `submitted job-dba55a5bbc38 with 16 units`, `done in 4.001s` and `saved result.json (71 bytes)`, and `result.json` holds `{"estimate":3.14158544,"inside":628317088,"total":800000000,"units":16}`.

  The counts for seed 42 (786162, 785489 and 785008 inside of 1,000,000) were the same on arm64 and on amd64. One sample takes about 3 ns (3.2 ns on arm64, 2.2 ns on amd64 under Rosetta). The automated test `TestEndToEndMonteCarlo` runs a job through the coordinator and checks that the result is byte for byte the same as the job run in the test.

## TC-16 Session secrets

- **Goal:** a volunteer that knows another volunteer's ID (gossip shares all IDs) cannot take work or send results in its name.
- **Steps:** 1 coordinator with a token list. The victim and the attacker register, and the victim takes a unit. The attacker sends `RequestWork` and `SubmitResult` under the victim's ID, with no secret and with its own secret. Then the victim sends its real result.
- **Expected:** every attacker request gets `Unauthenticated`, nothing is recorded, and the victim's result finishes the job.
- **Actual (2026-11-23):** all 4 attacker requests got `Unauthenticated`. `sc_requests_rejected_total` = 4 and `sc_results_received_total` = 0. The victim's result was recorded, and the job finished. The test passed 10 runs in a row with the race detector. `TestNewRegistrationGetsNewSecret` also checks that an old secret does not work for a new registration, and `TestVolunteerRegistersAgainAfterRejectedSession` checks that a rejected volunteer registers again and uses its new secret.
- **Also checked:** `make fail-volunteer` passes, and a short load test (`scripts/load.sh 2 4 2 full`) finished all jobs, so real volunteers send their secrets correctly.

## How to add a case

Add a row to the first table and a section with goal, steps, expected and actual (with the date). Paste real output, not expected output.
