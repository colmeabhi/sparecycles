# Demo script

The live demo for slide 11: 3 minutes 30 seconds, 5 parts. Kushagra drives the keyboard. Each owner narrates their own scenario.

Every command runs from the repo root on one Mac. No Docker is needed.

**Checked on 2026-11-25** on `main` after all algorithms merged: all 5 parts pass, and the outputs below come from those runs.

## Before the recording

Do these the day before and again 10 minutes before you record.

| Step | Command or action |
| --- | --- |
| 1. Update and build | `git pull`, then `make build` |
| 2. Check that tests pass | `make test` |
| 3. Free the ports | Nothing may listen on 7400 to 7403 or 9400 to 9403. Stop old runs with Ctrl+C. |
| 4. Load data | Done on 2026-11-25 on the final code (TC-07, TC-08 and TC-11 in `docs/test-results.md`). Rerun only if the code changes. |
| 5. Record backup clips (on 12-01) | Record each part below once with QuickTime (File → New Screen Recording). |
| 6. Prepare the screen | One terminal, font size 18 or larger, window at half the screen. The slide 12 image (`docs/presentation/visuals/s12-evaluation.png`) open in a second window. |
| 7. Remove the demo row | Part 5 appends a row to `results/load.csv`. After the recording, run `git restore results/load.csv`. |
| 8. Silence the Mac | Turn on Do Not Disturb. Close chat apps. |

## Part 1: One job (0:30) — Kushagra

**Show:** 3 terminal tabs, already running before the part starts:

```bash
make run-coordinator
```

```bash
SC_VOLUNTEER_NAME=volunteer-1 make run-volunteer
```

```bash
SC_VOLUNTEER_NAME=volunteer-2 make run-volunteer
```

Set the names, or the logs show the Mac's host name.

**Run** in a 4th tab:

```bash
make run-client
```

**Expected** (checked 2026-11-25):

```text
submitted job-e3b7af85f228 with 16 units
16/16 units done
done in 1.002s
saved result.png (50587 bytes)
```

Open `result.png`. The coordinator and volunteer tabs show warnings that tasks are not signed and that every volunteer is accepted. That is normal for the default settings.

**Say:** "A client submits a fractal job. The leader cuts it into 16 units. Each unit goes to both volunteers, and the leader keeps it when the two results agree. Then it joins the results into this image. Now we break things." Stop the 3 processes with Ctrl+C.

## Part 2: A volunteer crashes (0:50) — Kushagra

**Run:**

```bash
make fail-volunteer
```

**Expected (about 10 seconds):** the script kills volunteer A with `kill -9`, then prints (checked 2026-11-25):

```text
22:36:12 msg="member suspect"                  member=vol-2c0ee27252c5
22:36:14 msg="member dead"                     silent_for=2.437s
22:36:15 msg="lease released: volunteer dead"  unit=job-97308f657cf3-116
22:36:15 msg="volunteer removed: dead"
22:36:15 msg="job finished"                    state=done
PASS: volunteer-a was marked dead, its unit moved to volunteer-b, and the job finished
```

In our runs, `silent_for` was between 2.3 and 2.9 seconds.

**Say (while it runs):** "Two volunteers work on a 256-unit job. We kill one with kill -9, so it cannot say goodbye. SWIM gossip suspects it, and declares it dead in under 3 seconds. The leader releases its unit, the other volunteer takes it, and the job still finishes. Without gossip, that unit would wait 3 minutes for its lease."

## Part 3: A volunteer lies (0:40) — Tejas narrates

**Run:**

```bash
make fail-faulty
```

**Expected (about 10 seconds; checked 2026-11-25):**

```text
22:36:56 msg="wrong result"                 unit=job-4a9a18f85461-36 volunteer=vol-dfeadffae3e6
22:36:56 msg="wrong result"                 unit=job-4a9a18f85461-37 volunteer=vol-dfeadffae3e6
22:36:56 msg="volunteer blocked: low trust" trust=0.25 correct=0 wrong=2
22:36:56 msg="wrong result"                 unit=job-4a9a18f85461-38 volunteer=vol-dfeadffae3e6
22:36:57 msg="job finished"                 state=done
PASS: the faulty volunteer was outvoted and the image is correct
```

The number of `wrong result` lines changes from run to run (3 to 8 in our runs). A wrong result after the block comes from a unit that the volunteer took before the block.

**Say:** "One of four volunteers sends wrong results on purpose. Every unit runs on three volunteers, and two must agree. The faulty one loses two to one. After its second wrong result, its trust falls to 0.25, and the leader blocks it. The final image is identical to the one honest volunteers make."

## Part 4: The leader crashes (0:50) — Abhishek narrates

**Run:**

```bash
make fail-leader
```

**Expected (about 15 seconds; checked 2026-11-25):**

```text
22:36:28 msg="election started"   node=2 term=1
22:36:28 msg="new leader"         node=2 leader=3 term=2
22:36:30 msg="snapshot restored"  node=2 from=3 index=0
22:36:33 msg="election started"   node=2 term=3
22:36:33 msg="became leader"      node=2 term=3
22:36:39 msg="job finished"       node=2 state=done
PASS: node 2 took over after the leader crash, and the job finished
```

The first 3 lines are the normal start: node 3 wins term 2. The kill comes after that.

**Say:** "Three coordinators; node 3 leads in term 2. We kill it in the middle of a job. Node 2 misses the heartbeats, starts an election in term 3, loads the newest task table, and becomes the leader about 2.5 seconds after the crash. The client follows the new leader on its own, and the volunteers register again with it. The job finishes."

## Part 5: Scaling (0:40) — Kushagra

**Run** one short load test live, with 4 volunteers and 4 clients:

```bash
scripts/load.sh 4 8 4 full
```

**Expected (about 10 seconds; checked 2026-11-25):**

```text
setup        full, 4 volunteers, 4 clients
jobs         8 (0 failed)
units        512
total time   6.903s
throughput   69.54 jobs/min, 74.2 units/s
job time     mean 2.958s, p95 3.965s
PASS: load run finished (full, 4 volunteers)
```

**Say (while it runs):** "Last, load. Four users submit eight jobs at the same time to four volunteers, with three copies of every unit. The cluster accepts about 74 units per second, and a job takes about 3 seconds. The next slide shows how this changes with more volunteers and more users."

Then go back to the slides: slide 12 shows the full measurements.

After the recording, run `git restore results/load.csv` to remove the demo row.

## If something fails during the recording

| Problem | Action |
| --- | --- |
| A script prints `FAIL` or hangs | Say "we recorded this run earlier", and play the backup clip. Look into the logs in `runs/` after the recording. |
| `port 7400 is busy` | An old run is still alive. Stop it with Ctrl+C in its tab, or close that terminal. |
| A script prints `SKIP` | The checkout is old. Run `git pull` and `make build`, or play the backup clip. |
| The run is much slower than above | Close other apps. The numbers in the narration come from `results/load.csv`; say them as "in our measured runs". |
