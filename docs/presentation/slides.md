# Slide text

The text and the speaker notes for each slide. The plan (speakers, times, visuals) is in [outline.md](outline.md), and the live demo steps are in [demo-script.md](demo-script.md).

Copy the text into the shared deck. Put the images from [visuals/](visuals/) on the slides. Keep few words on a slide; the rest goes in the speaker notes.

Each image in `visuals/` has an HTML source in the product page style (1600×900). To make the PNG again after a change, open the HTML file in Chrome and take a screenshot, or run headless Chrome:

```bash
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --hide-scrollbars --force-device-scale-factor=2 --virtual-time-budget=8000 --window-size=1600,900 --screenshot=docs/presentation/visuals/s12-evaluation.png docs/presentation/visuals/s12-evaluation.html
```

| Slide | Owner | State |
| ---: | --- | :---: |
| 1, 2, 3, 8, 14 | Tejas | ☐ |
| 4 | All 3 (one part each) | ◐ Kushagra part done |
| 5, 6, 7, 10, 13 | Abhishek | ☐ |
| 9, 11, 12 | Kushagra | ◐ drafted |

State: ☐ not started · ◐ draft · ☑ final (after the run-through on 12-01).

## Slide 4: Related work (Kushagra part, 0:30)

**On the slide** (2 rows of the shared table):

| Paper | Main idea | What we took |
| --- | --- | --- |
| SWIM [2] | Ping one random member per period; ask others to ping it; suspect before dead; updates ride on pings | The whole protocol, for volunteers and coordinators together |
| VECA [6] | Predict when volunteers are online; fail over; enclaves for privacy | The problem. We detect and tolerate failures instead of predicting them. |

**Speaker notes:**

> SWIM gives us failure detection that stays cheap as the group grows. Every member pings one random member per period, and membership news rides on those pings. We use it as the paper describes it, for volunteers and coordinators together.
> VECA works on the same problem, volunteers that come and go, from the other side. It predicts availability and places work on nodes that will probably stay. We assume failures will happen: we detect them, recover, and outvote wrong results. Availability forecasting is one of our next steps.

## Slide 9: Gossip and security (Kushagra, 1:00)

**Title:** Gossip finds failures. Signatures stop fake work.

**Image:** [visuals/s09-gossip-security.png](visuals/s09-gossip-security.png)

**On the slide** (if the image is not used):

- SWIM: each member pings 1 random member per second.
- No ack → 3 other members try. Still no ack → suspect, then dead.
- The leader frees the units of a dead volunteer at once.
- Each volunteer has its own token. A wrong token is rejected.
- The leader signs each task with Ed25519. A bad signature stops the volunteer.

**Speaker notes:**

> A volunteer can disappear at any moment, without a goodbye. Without help, the leader learns this only when the 3-minute lease runs out.
> So everyone gossips, SWIM style. Each second, a member pings one random member. If there is no answer, it asks 3 others to try, so one lost packet does not kill a healthy node. If they also fail, the node is suspect. It can still answer and clear its name. If it stays silent, it is declared dead, and the leader frees its units at once. The news rides on the pings, so the cost per member stays the same as the group grows.
> Security has two parts. Each volunteer has its own token, so an unknown machine cannot join. And the leader signs every task with Ed25519, over the unit, the workload, the lease time and the input. A volunteer that gets a task with a bad signature stops and runs nothing.

## Slide 11: Live demo (Kushagra drives, 3:30)

**Title:** Live demo

**On the slide** (shown before the terminal):

| Part | Shows | Narrator | Time |
| ---: | --- | --- | ---: |
| 1 | One job: split, run, merge | Kushagra | 0:30 |
| 2 | A volunteer crashes | Kushagra | 0:50 |
| 3 | A volunteer lies | Tejas | 0:40 |
| 4 | The leader crashes | Abhishek | 0:50 |
| 5 | More volunteers, more users | Kushagra | 0:40 |

**Speaker notes:** follow [demo-script.md](demo-script.md). It has the commands, the expected output and the words for each part. Show this slide for about 5 seconds, then switch to the terminal.

## Slide 12: Evaluation (Kushagra, 1:00)

**Title:** What fault tolerance costs

**Image:** [visuals/s12-evaluation.png](visuals/s12-evaluation.png)

**On the slide** (if the image is not used):

- 2.9 s to declare a killed volunteer dead. The lease alone takes 3 minutes.
- More volunteers: 72 → 92 → 112 units/s (1, 2, 4 volunteers, 1 copy).
- 3 copies per unit, 2 must agree: 76 units/s with 4 volunteers, 32% less.
- More users: throughput is flat after 2 clients; job time grows from 1.6 s to 4.5 s.
- 16 of 16 test cases pass. A new leader takes over in 2.5 s.

**Speaker notes:**

> All numbers come from one laptop with 10 cores, and every node is a separate process.
> First, failure detection. Gossip declared the killed volunteer dead in 2.9 seconds. With leases alone, its unit would wait 3 minutes.
> Second, scaling. With one copy per unit, throughput grows from 72 to 112 units per second as we go from 1 to 4 volunteers. Our test units are very short, about 15 milliseconds, so the work at the leader for each unit is a large part of the cost. Real 60-second units would scale better.
> Third, the price of safety. Each unit goes to 3 volunteers, and the leader accepts it when 2 agree. With 4 volunteers, that gives 76 units per second, 32% less than one copy. That is what it costs to outvote a lying volunteer. Full mode starts at 2 volunteers, because one volunteer can never form a majority.
> Last, more users. Throughput rises up to 2 clients, and then stays flat: the volunteers are busy, so extra jobs wait and job time grows.

The numbers come from `make load-sweep` and `scripts/load.sh 4 8 C full` on 2026-11-25, after all algorithms merged (TC-07, TC-08 and TC-11 in `docs/test-results.md`).
