# Demo script

The live demo for slide 11: 3 minutes 30 seconds, 5 parts. Kushagra drives the keyboard. Each owner narrates their own scenario.

Every command runs from the repo root on one Mac. No Docker is needed.

## Before the recording

Do these the day before and again 10 minutes before you record.

| Step | Command or action |
| --- | --- |
| 1. Update and build | `git pull`, then `make build` |
| 2. Check that tests pass | `make test` |
| 3. Free the ports | Nothing may listen on 7400 to 7403 or 9400 to 9403. Stop old runs with Ctrl+C. |
| 4. Client sweep data | Done on 2026-11-21 (TC-11 in `docs/test-results.md`). Rerun only if the code changes: `scripts/load.sh 4 8 C full` for C = 1, 2, 4, 8, in the foreground. |
| 5. Record backup clips (on 12-01) | Record each part below once with QuickTime (File → New Screen Recording). |
| 6. Prepare the screen | One terminal, font size 18 or larger, window at half the screen. `results/load.csv` chart and `site/index.html` open in a second window. |
| 7. Silence the Mac | Turn on Do Not Disturb. Close chat apps. |

## Part 1: One job (0:30) — Kushagra

**Show:** 3 terminal tabs, already running before the part starts:

```bash
make run-coordinator
```

```bash
make run-volunteer
```

```bash
make run-volunteer
```

**Run** in a 4th tab:

```bash
make run-client
```

**Expected:** `submitted job-... with 16 units`, `16/16 units done`, `done in ~1s`, `saved result.png`. Open `result.png`.

**Say:** "A client submits a fractal job. The leader cuts it into 16 units. Two volunteers pull the units, and the leader joins the results into this image. Now we break things." Stop the 3 processes with Ctrl+C.

## Part 2: A volunteer crashes (0:50) — Kushagra

**Run:**

```bash
make fail-volunteer
```

**Expected (about 20 seconds):** the script kills volunteer A with `kill -9`, then prints:

```text
member suspect
member dead                     silent_for=~2.5s
lease released: volunteer dead  unit=...
job finished                    state=done
PASS: volunteer-a was marked dead, its unit moved to volunteer-b, and the job finished
```

**Say (while it runs):** "Two volunteers work on a 256-unit job. We kill one with kill -9, so it cannot say goodbye. SWIM gossip suspects it, and declares it dead in under 3 seconds. The leader releases its unit, the other volunteer takes it, and the job still finishes. Without gossip, that unit would wait 3 minutes for its lease."

## Part 3: A volunteer lies (0:40) — Tejas narrates

Needs issue #7. Until then the script prints `SKIP`, and you use the backup clip of the reference image only.

**Run:**

```bash
make fail-faulty
```

**Expected:** `wrong result` lines in the log, then `PASS: the faulty volunteer was outvoted and the image is correct`.

**Say:** "One of four volunteers sends wrong results on purpose. Every unit runs on three volunteers, and two must agree. The faulty one loses two to one, and the final image is identical to the one honest volunteers make."

## Part 4: The leader crashes (0:50) — Abhishek narrates

Needs issues #13 and #14. Until then the script prints `SKIP`.

**Run:**

```bash
make fail-leader
```

**Expected:** coordinator 2 logs `election started`, then `became leader` with a higher term; then `PASS: node 2 took over after the leader crash, and the job finished`.

**Say:** "Three coordinators; node 3 leads. We kill it in the middle of a job. Node 2 misses the heartbeats, starts an election in a new term, gets the newest task table from the backup, and continues. The client and the volunteers follow the new leader on their own."

## Part 5: Scaling (0:40) — Kushagra

**Show** the chart of `results/load.csv` (slide 12 can reuse it), then **run** one short load test live:

```bash
scripts/load.sh 4 8 4 full
```

**Expected (about 15 seconds):** a summary with `throughput ... units/s` and `job time mean ..., p95 ...`.

**Say:** "With more volunteers, throughput grows: 69, 110 and 140 units per second for 1, 2 and 4 volunteers without redundancy. With three copies per unit, 4 volunteers give 74 units per second, so fault tolerance costs about half the throughput. With more users, throughput rises from 42 to about 75 units per second at 2 clients, then stays flat, while job time grows from 1.5 to 4.7 seconds at 8 clients: the volunteers are the limit."

## If something fails during the recording

| Problem | Action |
| --- | --- |
| A script prints `FAIL` or hangs | Say "we recorded this run earlier", and play the backup clip. Look into the logs in `runs/` after the recording. |
| `port 7400 is busy` | An old run is still alive. Stop it with Ctrl+C in its tab, or close that terminal. |
| A script prints `SKIP` | That issue is not merged. Use the backup clip only if it was recorded with the merged code; otherwise say the feature is in progress. |
| The run is much slower than above | Close other apps. The numbers in the narration come from `results/load.csv`; say them as "in our measured runs". |
