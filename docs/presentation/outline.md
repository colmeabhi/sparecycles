# Presentation plan

The recorded presentation: about 15 minutes, slides plus a live demo. Due **2026-12-03**, the same day as the code.
Requirements: `docs/project-instructions.md`, "Final Project Presentation". Checklist: `docs/grading-checklist.md`, part 4.

The slides must be clear to anyone who took the course. Use one idea per slide, large diagrams, and few words.

## Plan

| Date | Step |
| --- | --- |
| 2026-11-24 | Agree on this plan, the speakers and the slide tool |
| 2026-11-30 | Slides drafted; each speaker writes their own slides |
| 2026-12-01 | First full run-through with a timer; record the demo clips as a backup |
| 2026-12-02 | Final run-through; record the presentation |
| 2026-12-03 | Upload the recording and the slides |

## Decisions to make

| Topic | Recommendation | Why |
| --- | --- | --- |
| Slide tool | Google Slides, from one shared deck | All 3 can edit at the same time; exports to PDF for the submission |
| Recording | One Zoom call, all 3 present, the screen recorded at 1080p | One file, natural hand-offs between speakers |
| Demo | Live in the recording, with pre-recorded clips as a backup | The course asks for a live demo; the clips save the recording if a run fails |
| Look | The product page style: paper background, pixel headings, tile colors | Ready-made visuals in `site/index.html` |

## Slides

Times add up to 15 minutes. Each person speaks about 5 minutes.

| # | Slide | Speaker | Time | Content | Visual and source |
| ---: | --- | --- | ---: | --- | --- |
| 1 | Title | Tejas | 0:20 | SpareCycles: a fault-tolerant volunteer computing platform. Team names. | Product page hero |
| 2 | The problem | Tejas | 0:50 | Big jobs, idle machines. Volunteers leave, lie, and the server can crash. | 3 failure cards from `site/index.html` |
| 3 | Goals | Tejas | 0:40 | Correct results despite churn, wrong results and leader crashes. No single point of failure. Jobs that split into independent units. | Fit / not-a-fit list |
| 4 | Related work | Tejas, Kushagra, Abhishek | 1:30 | BOINC (replication, central server). SWIM (gossip). VECA (fail-over, availability forecasting). Bully (election). VECTrust (trust). One line each: what we took. | Small table |
| 5 | Architecture | Abhishek | 1:20 | Client, 3 coordinators (1 leader, 2 backups), volunteers. Pull-based work. Replicated task table. | Diagram D2 (deployment) |
| 6 | Life of a job | Abhishek | 1:00 | Split, pull, run, vote, merge. Every table change is replicated first. | Diagram D3 |
| 7 | Design choices vs. challenges | Abhishek | 1:00 | Churn → leases and gossip. Wrong results → voting. Crash → election. Split brain → terms and 2-of-3 lease. Heterogeneity → resource discovery. | Two-column table |
| 8 | Leases, voting, resource discovery | Tejas | 1:20 | How each works, in one sentence each, and the 2-of-3 rule. | Diagram D4 (unit states) |
| 9 | Gossip and security | Kushagra | 1:00 | SWIM probe, indirect probe, suspect, dead. Signed tasks and tokens. | Diagram D6 |
| 10 | Election and replication | Abhishek | 1:00 | Bully with terms, leader lease, catch-up before acting. | Diagram D5 |
| 11 | Live demo | Kushagra (drives), each narrates their scenario | 3:30 | See [demo-script.md](demo-script.md) | Terminal and Prometheus |
| 12 | Evaluation | Kushagra | 1:00 | Throughput with 1, 2, 4 volunteers; cost of 3 copies; detection time; test results. | Chart from `results/load.csv` |
| 13 | Next steps | Abhishek | 0:30 | Adaptive replication, early cancel of copies, availability forecasting, confidential computing, real volunteers on AWS. | List |
| 14 | Summary | Tejas | 0:20 | 3 key points: correct despite failures, 6 algorithms from scratch, measured cost. | — |

**Per speaker:** Tejas about 5:00, Kushagra about 5:00 (with the demo), Abhishek about 5:00.

## Requirement coverage

| Course requirement | Slides |
| --- | --- |
| Introduce the project and the problem | 1–3 |
| Related work | 4 |
| Key design choices and the challenges | 7–10 |
| Components, architecture and features | 5–6 |
| Live demo: a crash | 11 (volunteer crash, leader crash) |
| Live demo: scaling with concurrent access | 11 (more volunteers) |
| Live demo: performance as users increase | 11 (more clients) |
| Evaluation with metrics | 12 |
| Next steps | 13 |
| Summary | 14 |

## Depends on

| Slide | Needs |
| --- | --- |
| 5, 10 | Diagrams D2 and D5 (Abhishek) |
| 8 | Diagram D4 (Tejas) |
| 11, faulty volunteer | Issue #7 |
| 11, leader crash | Issues #13 and #14 |
| 12, "full" numbers | Rerun `make load-sweep` after #7 |

## How to update this file

Change speakers or times only after the team agrees, and keep the total at about 15 minutes. After each run-through, write the real times here.
