# Project status

Read this file right after `AGENTS.md`. It says what is left, who does it next, and what blocks it.
GitHub issues hold the task details. This file only summarizes and links.

**Updated:** 2026-11-25 · **Update it in every PR** (see the definition of done in `AGENTS.md`).

## Now

| Deadline | Date | Days left |
| --- | --- | ---: |
| Issues #6, #13 | 2026-11-27 | 2 |
| Issues #14, #35 | 2026-11-28 | 3 |
| Issues #9, #15 | 2026-11-29 | 4 |
| Code and recorded presentation | 2026-12-03 | 8 |
| Final report | 2026-12-04 | 9 |

**State:** the base platform, gossip, security (now with session secrets, #35), scripts and the product page are done. Majority voting (#7, PR #32) and resource discovery (#8, PR #33) are merged. Tejas has PRs open for his other issues, with the review comments fixed. Bully election (#37), replication (#38) and the Terraform files (#39) are merged; the first real AWS run is next.

## Per person

### Tejas (`Tejas-Chakkarwar`)

| Issue | Due | Next step | Blocked by |
| --- | --- | --- | --- |
| [#6 Lease expiry](https://github.com/colmeabhi/sparecycles/issues/6) | 11-27 | PR [#31](https://github.com/colmeabhi/sparecycles/pull/31): fix the review comments, then merge | — |
| [#9 Monte Carlo](https://github.com/colmeabhi/sparecycles/issues/9) | 11-29 | PR [#34](https://github.com/colmeabhi/sparecycles/pull/34): fix the review comments, then merge | — |

#7 and #8 are done (#32, #33). Merge order: #31, #34.

### Abhishek (`colmeabhi`)

| Issue | Due | Next step | Blocked by |
| --- | --- | --- | --- |
| [#15 AWS with Terraform](https://github.com/colmeabhi/sparecycles/issues/15) | 11-29 | Terraform is on `main` (#39). Next: one real `make deploy-up` with the team account, a job from a laptop, the leader failover (`kill_leader_command` output), then `make deploy-down`. Paste the outputs in #15 and close it. | Team AWS account |

#13 and #14 are done (#37, #38).

### Kushagra (`Kushagrabainsla`)

Issues #10, #11, #12 and #35 (session secrets) are done. Next:

- Run the "after merge" steps below when #31 to #34, #13 and #14 land.
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
| Failover test on AWS | First `make deploy-up` (#15) |

## After merge

Do these right after the named PR merges, in the same day:

| After | Do |
| --- | --- |
| #7 merges | Kushagra: change `scripts/load-sweep.sh` so that "full" mode uses 2 or more volunteers (1 volunteer never reaches 2 of 3). Then run `make load-sweep` again, and update TC-08, report 9.3 and slide 12 (`docs/presentation/visuals/s12-evaluation.html`). |
| #9 merges | Kushagra: give `cmd/loadgen` an empty `-params` default, as in the client, so `-workload montecarlo` works. |
| #6 to #9 merge | Kushagra: update diagram I1 (the stubs that were replaced), and the test count on slide 12. |
| #35 and any PR with new tests merge | A test that calls `RequestWork` or `SubmitResult` directly must send the `session_secret` from its `RegisterResponse`. Whoever merges second fixes the tests. |
| #14 merges | Kushagra: the new leader does not know the old volunteer IDs. Their leases end through gossip (`member dead`) or, after #6, the lease timeout. Check this in `make fail-leader` logs. |
| Any algorithm issue merges | Update its card and the roadmap on the product page (`site/index.html`). |

## Done

| Date | PR | What |
| --- | --- | --- |
| 2026-11-22 | [#33](https://github.com/colmeabhi/sparecycles/pull/33) | Resource discovery: `scheduler.Matcher` with VECTrust-style trust, speed and memory rules; faulty volunteer blocked after 2 wrong results (TC-14; closes #8; stacked on #32) |
| 2026-11-22 | [#32](https://github.com/colmeabhi/sparecycles/pull/32) | Majority voting: `verify.Majority`, `SC_VOTE_QUORUM`, faulty mode `SC_FAULTY_RATE`, late copies checked, `make fail-faulty` passes (TC-03; closes #7) |
| 2026-11-25 | [#40](https://github.com/colmeabhi/sparecycles/pull/40) | Pages workflow split into build and deploy jobs; the deploy no longer fails when GitHub lists the artifact late |
| 2026-11-25 | [#39](https://github.com/colmeabhi/sparecycles/pull/39) | Terraform for AWS: 3 coordinators in 3 zones, spot volunteers, Prometheus; `make deploy-up` and `deploy-down`; CI checks it (refs #15; first apply to do) |
| 2026-11-25 | [#38](https://github.com/colmeabhi/sparecycles/pull/38) | Task-table replication (2 of 3 copies) and leader catch-up; `make fail-leader` passes (closes #14, TC-04) |
| 2026-11-25 | [#37](https://github.com/colmeabhi/sparecycles/pull/37) | Bully election with terms, majority and leader lease; 3 coordinators in Docker Compose (closes #13) |
| 2026-11-23 | [#36](https://github.com/colmeabhi/sparecycles/pull/36) | Session secrets: nobody can act under another volunteer's ID (closes #35, TC-16) |
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
