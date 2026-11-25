# Slide text

The text and the speaker notes for each slide. The plan (speakers, times, visuals) is in [outline.md](outline.md), and the live demo steps are in [demo-script.md](demo-script.md).

**The deck is in Google Slides** ("SpareCycles presentation", in Kushagra's Google Drive; ask him for edit access). It was built on 2026-11-25 from this file: each slide has its title from here, its image from [visuals/](visuals/), and these speaker notes (with the speaker and the time first). From now on, edit the words in the deck. If you change a slide here as well, keep both the same. Keep few words on a slide; the rest goes in the speaker notes.

Each image in `visuals/` has an HTML source in the product page style (1600×900). To make the PNG again after a change, open the HTML file in Chrome and take a screenshot, or run headless Chrome:

```bash
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --hide-scrollbars --force-device-scale-factor=2 --virtual-time-budget=8000 --window-size=1600,900 --screenshot=docs/presentation/visuals/s12-evaluation.png docs/presentation/visuals/s12-evaluation.html
```

| Slide | Speaker | State |
| ---: | --- | :---: |
| 1, 2, 3, 8, 14 | Tejas | ◐ drafted |
| 4 | All 3 (one part each) | ◐ drafted |
| 5, 6, 7, 10, 13 | Abhishek | ◐ drafted |
| 9, 11, 12 | Kushagra | ◐ drafted |

Each speaker reads their slides and speaker notes before the run-through on 12-01, and changes the words so that they sound like their own.

State: ☐ not started · ◐ draft · ☑ final (after the run-through on 12-01).

## Slide 1: Title (Tejas, 0:20)

**Title:** SpareCycles

**Image:** [visuals/s01-title.png](visuals/s01-title.png)

**On the slide:**

- SpareCycles: a fault-tolerant volunteer computing platform
- One job. Many machines. Verified.
- Tejas Chakkarwar · Kushagra Bainsla · Abhishek Ahirrao

**Speaker notes:**

> Hi, we are Tejas, Kushagra and Abhishek. Our project is SpareCycles, a volunteer computing platform that keeps working when machines fail or lie.

## Slide 2: The problem (Tejas, 0:50)

**Title:** Idle machines can do big jobs. But they fail.

**Image:** [visuals/s02-problem.png](visuals/s02-problem.png)

**On the slide:**

- A big job splits into many small units.
- Volunteers run the units on their idle machines.
- 3 things go wrong:
  - A volunteer leaves in the middle of a unit.
  - A volunteer sends a wrong result.
  - The leader that hands out the work crashes.

**Speaker notes:**

> Most computers are idle for much of the day. Projects like SETI@home and Folding@home use this idle time for science. A big job is cut into small units, and volunteers run them.
> But volunteers are not servers. A laptop closes, or a spot instance is taken back, in the middle of a unit. A volunteer can also send a wrong result, because of bad hardware or on purpose. And the coordinator that hands out the work is one machine. If it crashes, the whole job stops.
> SpareCycles handles all three.

## Slide 3: Goals (Tejas, 0:40)

**Title:** What we set out to build

**Image:** [visuals/s03-goals.png](visuals/s03-goals.png)

**On the slide:**

- Correct results when volunteers leave or crash.
- Correct results when some volunteers lie.
- No single point of failure: 3 coordinators.
- Every algorithm written by us. No library for election, gossip or replication.
- Good fit: units that run alone and give the same bytes every time (fractal tiles, Monte Carlo).
- Not a fit: pieces that talk to each other, replies in milliseconds, private data.

**Speaker notes:**

> We had three goals for correctness. A job must give the right result when volunteers leave, when some volunteers lie, and when the leader crashes. So there are three coordinators, and no single point of failure.
> We also wrote every distributed algorithm ourselves.
> Not every job fits. A good unit runs alone and gives the same bytes on every machine, so that results can be compared. Our two workloads, fractal tiles and a Monte Carlo estimate of pi, are like that. Jobs whose pieces talk to each other, or that need replies in milliseconds, are not a fit.

## Slide 4: Related work (all three, 1:30)

**Title:** What we took from 5 papers

**Image:** [visuals/s04-related-work.png](visuals/s04-related-work.png)

**On the slide:**

| Paper | Main idea | What we took |
| --- | --- | --- |
| BOINC [1], [5] | Volunteers pull work; each unit runs on several hosts; a quorum must agree; one central server | Pull, 3 copies with 2 that agree, deadlines (our leases), late results still checked. We removed the single server. |
| SWIM [2] | Ping one random member per period; ask others to ping it; suspect before dead; updates ride on pings | The whole protocol, for volunteers and coordinators together |
| VECA [6] | Predict when volunteers are online; fail over; enclaves for privacy | The problem. We detect and tolerate failures instead of predicting them. |
| Bully [3] | The highest live node wins; safe only if messages arrive in time | The election, plus terms and a 2-of-3 leader lease against split brain |
| VECTrust [4] | Trust scores from a volunteer's history decide where work goes | Trust from votes: (correct + 1) / (correct + wrong + 2) |

**Speaker notes:**

> **Tejas:** BOINC runs most volunteer projects today. Volunteers pull work, each unit runs on several hosts, and a quorum must agree before a result counts. We took all of that. But BOINC has one project server, and when it is down, nothing moves. We removed that single point of failure.
> **Kushagra:** SWIM gives us failure detection that stays cheap as the group grows. Every member pings one random member per period, and membership news rides on those pings. We use it for volunteers and coordinators together.
> VECA works on the same problem from the other side. It predicts which volunteers will stay online. We assume failures will happen: we detect them, recover, and outvote wrong results.
> **Abhishek:** The Bully algorithm elects the highest live node. It is simple, but it is safe only when messages arrive in time. We added terms and a leader lease, so a network split cannot give two leaders.
> VECTrust keeps a trust score for each volunteer and uses it to place work. We learn trust directly from the votes, because a result that loses a vote is known to be wrong.

## Slide 5: Architecture (Abhishek, 1:20)

**Title:** Three roles, one leader

**Image:** [visuals/s05-architecture.png](visuals/s05-architecture.png)

**On the slide:**

- Client: submits a job and reads the result.
- 3 coordinators: 1 leader, 2 backups. Each has a copy of the task table.
- Volunteers: pull units from the leader, run them, send results.
- Everyone gossips (SWIM). Prometheus scrapes the metrics.
- All traffic is gRPC. A follower answers "not the leader", and the caller tries the next coordinator.

**Speaker notes:**

> There are three roles. A client submits a job and reads the result. Three coordinators keep the task table: which units exist, who runs them, and which results came back. One coordinator is the leader. The other two are backups with a copy of the table.
> Volunteers pull work from the leader. The leader never pushes work, so a volunteer behind a home router needs no open port for its main task.
> All nodes, coordinators and volunteers, form one gossip group. Every coordinator exports Prometheus metrics.
> Clients and volunteers know all three coordinators. If they reach a backup, it answers "not the leader", and they try the next one. So a failover is almost invisible to them.
> We run this as local processes, in Docker Compose, and we have Terraform for AWS with the three coordinators in three availability zones.

## Slide 6: Life of a job (Abhishek, 1:00)

**Title:** Split, pull, run, vote, merge

**Image:** [visuals/s06-job-life.png](visuals/s06-job-life.png)

**On the slide:**

1. The leader splits the job into units.
2. Volunteers pull units. Each unit goes to 3 different volunteers, with a lease and a signature.
3. Each volunteer checks the signature, runs the unit and sends the result.
4. When 2 results agree, the leader accepts the unit.
5. When all units are accepted, the leader merges the outputs.

Every change to the task table reaches a backup before the leader applies it.

**Speaker notes:**

> Here is one job. The leader splits it into units, for example 16 tiles of an image.
> Volunteers ask for work. Each unit goes to three different volunteers. Each copy has a lease, which is a deadline, and a signature from the leader.
> A volunteer checks the signature, runs the unit and sends the result. As soon as two results have the same hash, the leader accepts the unit. When every unit is accepted, the leader merges the outputs into the final image.
> One rule holds everywhere: every change to the task table goes to a backup first. The leader applies the change only after a backup has it. So if the leader crashes, nothing it confirmed is lost.

## Slide 7: Challenges and design choices (Abhishek, 1:00)

**Title:** One mechanism for each challenge

**Image:** [visuals/s07-challenges.png](visuals/s07-challenges.png)

**On the slide:**

| Challenge | Our answer |
| --- | --- |
| Volunteers leave | Leases + SWIM gossip |
| Wrong results | 3 copies, 2 must agree; trust scores |
| The leader crashes | Bully election with terms |
| Split brain | A leader acts only while 2 of 3 coordinators confirm it |
| State lost in a failover | Every change on 2 of 3 coordinators; catch-up before acting |
| Different machines | Resource discovery: memory, speed, trust |
| Untrusted volunteers | Tokens, session secrets, signed tasks |

**Speaker notes:**

> This slide maps each challenge to the mechanism that handles it.
> A volunteer that leaves is found by gossip in seconds, and its lease is the safety net. A wrong result is outvoted, because two of three copies must agree, and the volunteer loses trust. A leader crash starts a Bully election.
> A timeout cannot tell a crashed leader from a slow one, so plain Bully can end with two leaders. We stop that with terms and a leader lease: a leader acts only while two of three coordinators confirm it.
> A new leader must not lose work, so every change is on two coordinators, and a new leader loads the newest table before it acts. Resource discovery handles machines that differ. Tokens, session secrets and signatures handle volunteers we do not trust.

## Slide 8: Leases, voting and resource discovery (Tejas, 1:20)

**Title:** Who runs a unit, and which result counts

**Image:** [visuals/s08-leases-voting.png](visuals/s08-leases-voting.png)

**On the slide:**

- **Lease:** each copy has a deadline, 3 minutes by default. Expired, or the volunteer is dead → the copy goes to another volunteer.
- **Vote:** compare SHA-256 hashes. 2 of 3 agree → accepted. No majority → 2 more copies. A late copy is still checked.
- **Resource discovery:** trust = (correct + 1) / (correct + wrong + 2).
  - Below 0.3 → blocked. A faulty volunteer is blocked after its 2nd wrong result.
  - Below 0.6 → does not get the last, deciding copy.
  - While fast volunteers are active, slow ones take the newest units, so the end of a job goes to fast ones.

**Speaker notes:**

> Three algorithms decide who runs a unit and which result counts.
> First, leases. Each copy of a unit has a deadline, three minutes by default. If the lease expires, or gossip says the volunteer is dead, the leader gives the copy to someone else. A volunteer that lost its lease cannot send a result for it.
> Second, voting. The leader compares the hashes of the results. When two of three agree, the unit is accepted. If all three come back and no two agree, the leader sends the unit to two more volunteers. A copy that comes back after the vote is still checked, so a liar cannot hide by always being last.
> Third, resource discovery. Each vote updates the volunteer's trust score. A new faulty volunteer falls below 0.3 at its second wrong result and is blocked. Below 0.6, a volunteer does not get the last copy, which usually decides the vote. And slow volunteers take the newest units, so they do not hold up the end of a job.

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

## Slide 10: Election and replication (Abhishek, 1:00)

**Title:** A new leader in 2.5 seconds, with no lost work

**Image:** [visuals/s10-election-replication.png](visuals/s10-election-replication.png)

**On the slide:**

- The leader sends a heartbeat every 500 ms.
- No heartbeat for 2 s → a backup starts an election with a higher term.
- It wins with 2 of 3 votes, loads the newest table, then waits out the old leader's lease.
- Replication: the leader applies a change only after 1 backup has it (2 of 3 copies).
- Measured: the leader was killed during a 256-unit job. A new leader took over 2.5 s later, and the job finished.

**Speaker notes:**

> The leader sends a heartbeat every half second. If a backup hears nothing for two seconds, it starts an election in a new, higher term.
> To win, it needs two of three coordinators. Before it acts, it asks the others for their tables and loads the newest one. It also waits until the old leader's lease is over, so two leaders never act at the same time.
> Replication is simple. The leader sends each change to both backups, and it applies the change only after one backup has it. So every confirmed change is on two of three machines, and any majority that elects a new leader includes one of them.
> In our test, we killed the leader in the middle of a 256-unit job. Node 2 became leader 2.5 seconds later and continued from the replicated table. The volunteers registered again, and the job finished.

## Slide 11: Live demo (Kushagra drives, 3:30)

**Title:** Live demo

**Image:** [visuals/s11-demo.png](visuals/s11-demo.png)

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
> Second, scaling. With one copy per unit, throughput grows from 72 to 112 units per second as we go from 1 to 4 volunteers. Our test units are very short, about 15 milliseconds, so the work at the leader for each unit is probably a large part of the cost. Real 60-second units should scale better.
> Third, the price of safety. Each unit goes to 3 volunteers, and the leader accepts it when 2 agree. With 4 volunteers, that gives 76 units per second, 32% less than one copy. That is what it costs to outvote a lying volunteer.
> Last, more users. Throughput rises up to 2 clients, and then stays flat at about 70 to 75 units per second, so extra jobs wait and job time grows.

The numbers come from `make load-sweep` and `scripts/load.sh 4 8 C full` on 2026-11-25, after all algorithms merged (TC-07, TC-08 and TC-11 in `docs/test-results.md`).

## Slide 13: Next steps (Abhishek, 0:30)

**Title:** What we would do next

**Image:** [visuals/s13-next-steps.png](visuals/s13-next-steps.png)

**On the slide:**

- Fewer copies for trusted volunteers (adaptive replication, as in BOINC).
- Cancel the third copy when 2 agree.
- Catch-up that waits for a majority of coordinators.
- TLS on all connections.
- Availability forecasting, as in VECA.
- A full run on AWS across 3 availability zones.

**Speaker notes:**

> Some next steps. Our trust scores could let trusted volunteers run units with fewer copies, which would cut the cost of voting. Cancelling the third copy once two agree would save more work. A new leader's catch-up should wait for a majority of coordinators. TLS would protect the session secrets. We could also forecast availability, as VECA does. And we want a full run of the Terraform deployment across three AWS zones.

## Slide 14: Summary (Tejas, 0:20)

**Title:** Summary

**Image:** [visuals/s14-summary.png](visuals/s14-summary.png)

**On the slide:**

- Correct results despite 3 failures: a dead volunteer is found in 2.9 s, a liar is outvoted and blocked, a new leader takes over in 2.5 s.
- 6 distributed algorithms and the security layer, written from scratch. 16 of 16 test cases pass.
- The cost is measured: 3 copies with voting cost 32% of throughput with 4 volunteers.

**Speaker notes:**

> To sum up. SpareCycles gives correct results when volunteers leave, when they lie, and when the leader crashes. We wrote six distributed algorithms and the security layer ourselves, and all sixteen test cases pass. And we measured the price: voting costs about a third of the throughput. Thank you.
