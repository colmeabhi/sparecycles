# Final report outline

The plan for the final report: each required section, its owner, its sources in this repo, its page budget and its state.

**Format:** IEEE double column, 10 to 15 pages without images. **Due:** 2026-12-04 (the presentation is due 2026-12-03).
**Requirements:** `docs/project-instructions.md`, "Final Reports". **Checklist:** `docs/grading-checklist.md`, part 3.

## Plan

| Date | Step |
| --- | --- |
| 2026-11-24 | Agree on this outline and the 2 decisions below |
| 2026-11-30 | First draft of every section and every diagram |
| 2026-12-01 to 12-02 | Each person reviews the sections of the other two |
| 2026-12-03 | Final text; record the presentation |
| 2026-12-04 | Submit |

## Decisions

| Topic | Decision | Why |
| --- | --- | --- |
| Writing tool | LaTeX with `IEEEtran` in `docs/report/` (one file per section). Build with `make report`, or upload the folder to Overleaf. | IEEE double-column format; sections are reviewed in PRs |
| Diagram tool | PlantUML in `docs/report/diagrams/`, with the rendered PNG committed | Diagrams are reviewed like code and stay true when the code changes |
| Language | Clear academic English, as in a conference paper | The report is graded as an IEEE paper |

See [README.md](README.md) for the build steps and the writing rules.

## Sections

State: ☐ not started · ◐ draft · ☑ done. Pages are a budget, without figures.

| # | Section | Owner | Pages | Sources | State |
| --- | --- | --- | ---: | --- | :---: |
| 1 | Cover page: names, student IDs, emails | Kushagra (collects from all) | — | Each person sends their ID and email | ◐ Kushagra done |
| 2 | Goals and motivation | Tejas | 0.5 | `docs/proposal.md` §1–2, `site/index.html` | ☐ |
| 3 | Distributed systems challenges addressed | Abhishek | 0.5 | `docs/proposal.md` §2, `AGENTS.md` design decisions | ☐ |
| 4 | Literature review | Each person: own papers | 2.4 | Proposal §3 and references | ☐ |
| 4a | — BOINC [1], [5] | Tejas | 0.8 | | ☐ |
| 4b | — SWIM [2], VECA [6] | Kushagra | 0.8 | | ◐ |
| 4c | — Bully [3], VECTrust [4] | Abhishek | 0.8 | | ☐ |
| 5.1 | Design goals per challenge | Split, see below | 1.2 | `AGENTS.md`, module READMEs | ◐ Kushagra part done |
| 5.2 | Key components and algorithms | Each person: own algorithms | 1.5 | `internal/*/README.md` | ◐ Kushagra part done |
| 5.3 | Architecture and design UML (D1–D6) | Abhishek (text); diagrams below | 0.3 | `internal/coordinator/README.md`, `proto/README.md` | ☐ |
| 6.1 | Why the approach is good: intuition, correctness, complexity | Each person: own algorithms | 1.2 | Papers, code, tests | ◐ Kushagra part done |
| 6.2 | Fault tolerance, performance, scalability, consistency, concurrency, security | Split, see below | 0.9 | `docs/test-results.md`, `results/load.csv` | ◐ Kushagra part done |
| 7.1 | Architectural style | Kushagra | 0.3 | `AGENTS.md` code layout, `coordinator.Deps` | ◐ |
| 7.2 | Implementation UML (I1–I3) | Diagrams below | — | Code | ☐ |
| 7.3 | Software components and how they interact (gRPC) | Kushagra | 0.4 | `proto/sparecycles/v1/sparecycles.proto`, `internal/leaderclient` | ◐ |
| 8.1 | Success scenario with screenshots | Tejas | 0.2 | TC-01 in `docs/test-results.md` | ☐ |
| 8.2a | Failure: volunteer crash | Kushagra | 0.25 | `make fail-volunteer`, TC-02 | ☑ |
| 8.2b | Failure: faulty volunteer | Tejas | 0.25 | `make fail-faulty`, TC-03 | ☐ |
| 8.2c | Failure: leader crash | Abhishek | 0.25 | `make fail-leader`, TC-04 (after #13, #14) | ☐ |
| 9 | Growth rate of the major algorithms | Each person: own algorithms | 0.6 | 6.1 | ◐ Kushagra part done (in 6.1) |
| 9.1 | Overall result and reflection on design decisions | Abhishek | 0.3 | All | ☐ |
| 9.2–9.3 | Metrics, comparison with the baseline, throughput and latency | Kushagra | 0.7 | `results/load.csv`, TC-07, TC-08, `docs/observability.md` | ◐ volunteer and client sweeps done (rerun after #7) |
| 10 | Test cases and results (at least 5) | Tejas (each adds own cases) | 0.6 | `docs/test-results.md` | ◐ 15 of 15 have results |
| 11 | Conclusions, lessons learned, improvements | Abhishek | 0.4 | All | ☐ |
| — | References (IEEE style) | Kushagra | — | Proposal §7 | ◐ (`references.bib`) |

**Total:** about 13 pages. **Per person:** Kushagra about 4.0, Tejas about 4.0, Abhishek about 4.1.

### Split sections

| Section | Kushagra | Tejas | Abhishek |
| --- | --- | --- | --- |
| 5.1 Design goals | Security, failure handling, openness | Heterogeneity, quality of service, scalability | Concurrency, transparency |
| 5.2, 6.1, 9 Algorithms | SWIM gossip, tokens and signed tasks | Leases, majority voting, resource discovery | Bully election, replication |
| 6.2 Properties | Fault tolerance, security | Performance, scalability | Consistency, concurrency |

## UML diagrams

The course needs at least 3 design diagrams (5.3) and at least 3 implementation diagrams (7.2). This plan has 6 and 3, so that each person draws 3.

| ID | Type | Shows | Owner | Section | State |
| --- | --- | --- | --- | --- | --- |
| D1 | Use case | Client, volunteer and operator actions | Tejas | 5.3 | ☐ |
| D2 | Deployment | 3 coordinators in 3 zones, volunteers, Prometheus on AWS | Abhishek | 5.3 | ☐ |
| D3 | Sequence | One job: submit, pull, run, vote, merge | Kushagra | 5.3 | ☑ |
| D4 | State | Unit: open, leased, result recorded, copies raised, accepted | Tejas | 5.3 | ☐ |
| D5 | Sequence | Leader crash: heartbeats stop, election, catch-up, new leader | Abhishek | 5.3 | ☐ |
| D6 | State | SWIM member: alive, suspect, dead, refuted | Kushagra | 5.3 | ☑ |
| I1 | Class | `coordinator.Server`, `Deps` and the part interfaces | Kushagra | 7.2 | ☑ (update when a stub is replaced) |
| I2 | Component | Go packages and their dependencies | Abhishek | 7.2 | ☐ |
| I3 | Object | Task table during a job: jobs, units, assignments, results | Tejas | 7.2 | ☐ |

## Screenshots to capture

Save them in `docs/report/screenshots/`.

| For | What | How | File |
| --- | --- | --- | --- |
| 8.1 | Client progress and the result image | `make run-client` | `s1-success.png` |
| 8.2a | Coordinator log: suspect, dead, lease released | `make fail-volunteer` | `s2-volunteer-crash.png` ☑ |
| 8.2b | Coordinator log: wrong result; same image as the reference | `make fail-faulty` | `s3-faulty-volunteer.png` |
| 8.2c | Logs of node 2: election, became leader | `make fail-leader` | `s4-leader-crash.png` |
| 9.2 | Prometheus panels: throughput, members by state | Queries in `docs/observability.md` |
| 9.3 | Chart from `results/load.csv`: units/s by volunteers, baseline and full | Any chart tool |

## How to update this file

When you start a section, set it to ◐. When it is final in the report, set it to ☑. Change owners only after the team agrees, and keep the pages per person about equal.
