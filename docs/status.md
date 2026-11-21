# Project status

Read this file right after `AGENTS.md`. It says what is left, who does it next, and what blocks it.
GitHub issues hold the task details. This file only summarizes and links.

**Updated:** 2026-11-21 · **Update it in every PR** (see the definition of done in `AGENTS.md`).

## Now

| Deadline | Date | Days left |
| --- | --- | ---: |
| Issues #6, #7, #13 | 2026-11-27 | 6 |
| Issues #8, #14 | 2026-11-28 | 7 |
| Issues #9, #15 | 2026-11-29 | 8 |
| Code and recorded presentation | 2026-12-03 | 12 |
| Final report | 2026-12-04 | 13 |

**State:** the base platform, gossip, security, scripts and the product page are done. 7 of 10 issues are open, and none has a PR yet. Only 1 of the 6 course algorithms (gossip) is real today; the other 5 are stubs.

## Per person

### Tejas (`Tejas-Chakkarwar`)

| Issue | Due | Next step | Blocked by |
| --- | --- | --- | --- |
| [#6 Lease expiry](https://github.com/colmeabhi/sparecycles/issues/6) | 11-27 | Add `lease.Timeout` and its tests | — |
| [#7 Majority voting](https://github.com/colmeabhi/sparecycles/issues/7) | 11-27 | Add `verify.Majority` and `SC_FAULTY_RATE` | — |
| [#8 Resource discovery](https://github.com/colmeabhi/sparecycles/issues/8) | 11-28 | Add `scheduler.Matcher` with trust scores | Trust input needs #7 (can start before) |
| [#9 Monte Carlo](https://github.com/colmeabhi/sparecycles/issues/9) | 11-29 | Add the `montecarlo` workload | — |

Suggested order: #6, #7, #8, #9.

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
- Test Docker Compose with gossip (`make up`). It is not tested yet.
- Write your report sections and diagrams D3, D6, I1 (see `docs/report/outline.md`).

### All three

| Work | Due | Next step |
| --- | --- | --- |
| Final report (IEEE, 10 to 15 pages) | 12-04 | Agree on [docs/report/outline.md](report/outline.md) and its 2 decisions by 11-24. First drafts of all sections by 11-30. |
| UML diagrams (6 design, 3 implementation) | 12-04 | Owners are set in the outline: 3 diagrams each |
| Recorded presentation (about 15 min) and slides | 12-03 | Plan the demo around `make fail-volunteer`, `fail-faulty`, `fail-leader` |
| Submission README (setup and division of work) | 12-03 | Decide where it lives (the root README stays lean) |

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
| #7 merges | Run `make load-sweep` again: "full" mode then waits for 2 of 3 results. Update TC-08. |
| Any algorithm issue merges | Update its card and the roadmap on the product page (`site/index.html`). |

## Done

| Date | PR | What |
| --- | --- | --- |
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
