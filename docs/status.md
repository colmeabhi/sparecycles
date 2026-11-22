# Project status

Read this file right after `AGENTS.md`. It says what is left, who does it next, and what blocks it.
GitHub issues hold the task details. This file only summarizes and links.

**Updated:** 2026-11-22 · **Update it in every PR** (see the definition of done in `AGENTS.md`).

## Now

| Deadline | Date | Days left |
| --- | --- | ---: |
| Issues #7, #13 | 2026-11-27 | 5 |
| Issues #8, #14 | 2026-11-28 | 6 |
| Issues #9, #15 | 2026-11-29 | 7 |
| Code and recorded presentation | 2026-12-03 | 11 |
| Final report | 2026-12-04 | 12 |

**State:** the base platform, gossip, leases, security, scripts and the product page are done. 6 of 10 issues are open. 2 of the 6 course algorithms (gossip, leases) are real today; the other 4 are stubs.

## Per person

### Tejas (`Tejas-Chakkarwar`)

| Issue | Due | Next step | Blocked by |
| --- | --- | --- | --- |
| [#7 Majority voting](https://github.com/colmeabhi/sparecycles/issues/7) | 11-27 | Add `verify.Majority` and `SC_FAULTY_RATE` | — |
| [#8 Resource discovery](https://github.com/colmeabhi/sparecycles/issues/8) | 11-28 | Add `scheduler.Matcher` with trust scores | Trust input needs #7 (can start before) |
| [#9 Monte Carlo](https://github.com/colmeabhi/sparecycles/issues/9) | 11-29 | Add the `montecarlo` workload | — |

#6 is done. Suggested order: #7, #8, #9.

### Abhishek (`colmeabhi`)

| Issue | Due | Next step | Blocked by |
| --- | --- | --- | --- |
| [#13 Bully election](https://github.com/colmeabhi/sparecycles/issues/13) | 11-27 | Add `election.Bully` with terms and the `BeforeLead` hook | — |
| [#14 Replication](https://github.com/colmeabhi/sparecycles/issues/14) | 11-28 | Add `replication.Quorum` and leader catch-up | Terms and `BeforeLead` from #13 (backup side can start) |
| [#15 AWS with Terraform](https://github.com/colmeabhi/sparecycles/issues/15) | 11-29 | Add `deploy/terraform` | Failover test on AWS needs #13 and #14 |

Suggested order: #13, #14, #15.

### Kushagra (`Kushagrabainsla`)

All issues done (#10, #11, #12). Next:

- Run the "after merge" steps below when #7, #13 and #14 land.
- Report: your sections, diagrams D3, D6, I1, the 8.2a screenshot, and your ID and email are done. Rerun `make load-sweep` after #7 and update 9.3.
- Presentation: slides 4 (your part), 9, 11 and 12 are drafted in `docs/presentation/slides.md`. After #7, update slide 12 with the new load numbers.

### All three

| Work | Due | Next step |
| --- | --- | --- |
| Final report (IEEE, 10 to 15 pages) | 12-04 | The LaTeX report is in `docs/report/` (`make report`). Fill your red `\todo` parts; first drafts by 11-30. Each person adds their student ID and email. |
| UML diagrams (6 design, 3 implementation) | 12-04 | 3 of 9 done (D3, D6, I1). Tejas: D1, D4, I3. Abhishek: D2, D5, I2. PlantUML in `docs/report/diagrams/`. |
| Recorded presentation (about 15 min) and slides | 12-03 | Agree on [docs/presentation/outline.md](presentation/outline.md) (14 slides, about 5 min each) and its slide tool by 11-24. Slides drafted by 11-30 in [slides.md](presentation/slides.md) (Kushagra's are done; Tejas: 1, 2, 3, 4, 8, 14; Abhishek: 4, 5, 6, 7, 10, 13); run-through and backup demo clips on 12-01. Demo steps: [demo-script.md](presentation/demo-script.md). |
| Submission README (setup and division of work) | 12-03 | Drafted in [docs/submission.md](submission.md). Each person checks their rows. On 12-03, do its "Before you submit" list. |

## Waiting on

| This | Needs |
| --- | --- |
| `make fail-faulty`, test case TC-03 | #7 |
| `make fail-leader`, test case TC-04 | #13 and #14 |
| Trust scores in #8 | #7 (`Decision.Wrong`) |
| #14 leader catch-up | #13 (`BeforeLead` hook, terms) |
| Election-time metric (grading checklist, part 2) | #13 |
| Failover test on AWS | #13, #14, #15 |

## After merge

Do these right after the named PR merges, in the same day:

| After | Do |
| --- | --- |
| #7 merges | Run `make fail-faulty`. Record TC-03 in `docs/test-results.md`. |
| #13 and #14 merge | Run `make fail-leader`. Record TC-04. Check the "failure scripts" and "metrics" items in `docs/grading-checklist.md`. |
| #7 merges | Run `make load-sweep` again: "full" mode then waits for 2 of 3 results. Update TC-08, report 9.3 and slide 12 (`docs/presentation/visuals/s12-evaluation.html`). |
| Any algorithm issue merges | Update its card and the roadmap on the product page (`site/index.html`). |

## Done

| Date | PR | What |
| --- | --- | --- |
| 2026-11-22 | [#31](https://github.com/colmeabhi/sparecycles/pull/31) | Lease expiry: `lease.Timeout`; a lost unit moves to another volunteer when its lease ends (TC-13; closes #6) |
| 2026-11-21 | [#30](https://github.com/colmeabhi/sparecycles/pull/30) | Docker Compose cluster tested with gossip and Prometheus (TC-12); Kushagra's ID and email in the report |
| 2026-11-21 | [#29](https://github.com/colmeabhi/sparecycles/pull/29) | Slide text and speaker notes (`slides.md`); Kushagra's slides 4, 9, 11, 12 with 2 images |
| 2026-11-21 | [#28](https://github.com/colmeabhi/sparecycles/pull/28) | Report 8.2a: terminal screenshot of the volunteer crash; `docs/report/screenshots/` for section 8 |
| 2026-11-21 | [#27](https://github.com/colmeabhi/sparecycles/pull/27) | Submission README draft: setup steps and division of work |
| 2026-11-21 | [#26](https://github.com/colmeabhi/sparecycles/pull/26) | Load data measured again in the foreground; client sweep (TC-11); report 9.3 and test results updated |
| 2026-11-21 | [#25](https://github.com/colmeabhi/sparecycles/pull/25) | Presentation plan (14 slides, speakers, timing) and demo script |
| 2026-11-21 | [#24](https://github.com/colmeabhi/sparecycles/pull/24) | LaTeX report skeleton; Kushagra's sections and diagrams D3, D6, I1; `make report` |
| 2026-11-21 | [#23](https://github.com/colmeabhi/sparecycles/pull/23) | Report outline with owners, page budget and UML plan; `AGENTS.md` state column fixed |
| 2026-11-21 | [#22](https://github.com/colmeabhi/sparecycles/pull/22) | This shared status file |
| 2026-11-21 | [#21](https://github.com/colmeabhi/sparecycles/pull/21) | Load generator, failure scripts, load results, test results, observability docs (closes #12) |
| 2026-11-20 | [#20](https://github.com/colmeabhi/sparecycles/pull/20) | Per-volunteer tokens and Ed25519 signed tasks (closes #11) |
| 2026-11-20 | [#19](https://github.com/colmeabhi/sparecycles/pull/19) | SWIM gossip; dead volunteers lose their leases (closes #10) |
| 2026-11-20 | [#18](https://github.com/colmeabhi/sparecycles/pull/18) | Pages deploys on every push to `site/` |
| 2026-11-20 | [#17](https://github.com/colmeabhi/sparecycles/pull/17) | Product page and lean README |
| 2026-11-20 | [#16](https://github.com/colmeabhi/sparecycles/pull/16) | Module READMEs and code quality rules |
| 2026-11-20 | [#5](https://github.com/colmeabhi/sparecycles/pull/5) | Base version: gRPC contract, task table, stubs, CI |
| 2026-11-20 | [#1](https://github.com/colmeabhi/sparecycles/pull/1) to [#4](https://github.com/colmeabhi/sparecycles/pull/4) | Project guide, work split, design decisions, grading checklist |

## How to update this file

- In your PR: move your issue to "Done" with the date and PR link, change "Next step" and "Blocked by" if they changed, and update "Updated" and "Days left".
- Add new dependencies to "Waiting on", and new follow-ups to "After merge".
- Keep it to about one screen. Summarize; link to issues for details.
- Use simple language and real dates (YYYY-MM-DD).
