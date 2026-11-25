# Project status

Read this file right after `AGENTS.md`. It says what is left, who does it next, and what blocks it.
GitHub issues hold the task details. This file only summarizes and links.

**Updated:** 2026-11-25 · **Update it in every PR** (see the definition of done in `AGENTS.md`).

## Now

| Deadline | Date | Days left |
| --- | --- | ---: |
| Issue #15 (first AWS run) | 2026-11-29 | 4 |
| Report review by all three | 2026-12-02 | 7 |
| Code and recorded presentation | 2026-12-03 | 8 |
| Final report | 2026-12-04 | 9 |

**State:** the base platform, gossip, security (now with session secrets, #35), scripts and the product page are done. All 4 of Tejas's issues are merged: lease expiry (#6, PR #31), majority voting (#7, PR #32), resource discovery (#8, PR #33) and the Monte Carlo workload (#9, PR #34). Bully election (#37), replication (#38) and the Terraform files (#39) are merged; the first real AWS run is next. The full report draft is in `docs/report/` (PR #42): every section, all 9 diagrams and 5 screenshots, with the cover page complete. The slides are in a shared Google Slides deck (#46).

## Per person

### Tejas (`Tejas-Chakkarwar`)

All 4 issues are done (#6, #7, #8, #9; PRs #31 to #34). Next: review the report sections on your algorithms, and read your slides and notes in the shared deck (see "All three").

### Abhishek (`colmeabhi`)

| Issue | Due | Next step | Blocked by |
| --- | --- | --- | --- |
| [#15 AWS with Terraform](https://github.com/colmeabhi/sparecycles/issues/15) | 11-29 | Terraform is on `main` (#39). Use `main` with #45: before it, a leader crash after a few minutes of load ran the coordinators out of memory. Next: one real `make deploy-up` with the team account, a job from a laptop, the leader failover (`kill_leader_command` output), then `make deploy-down`. Paste the outputs in #15 and close it. | Team AWS account |

#13 and #14 are done (#37, #38). Also: review the report sections on election and replication, and read your slides and notes in the shared deck.

### Kushagra (`Kushagrabainsla`)

Issues #10, #11, #12 and #35 (session secrets) are done. Next:

- The "after merge" steps for #31 to #34, #37 and #38 are done (PR #41): new load sweep, `loadgen` default, short lease in `fail-leader.sh`, TC-03 and TC-04 rerun on `main`, diagram I1 and slide 12.
- Report: the full draft is done (PR #42). Every claim was checked against the code and the run records.
- Presentation: all 14 slides are drafted (PR #43), and all 5 demo parts were checked on `main` (PR #44). Next: the demo run-through on 12-01 (you drive the terminal).
- Prometheus figure for report 9.3 is done (PR #45). Its Docker run found and fixed unbounded task-table memory.

### All three

| Work | Due | Next step |
| --- | --- | --- |
| Final report (IEEE, 10 to 15 pages) | 12-04 | Full draft in `docs/report/` (`make report`): about 13 pages of text, 22 with figures. Each person reads all of it by 12-02, mainly the parts on their own algorithms, and puts the abstract, introduction and conclusions in the team's own words. All 3 IDs and emails are on the cover page; no `\todo` marks are left. If the AWS run (#15) happens, update the AWS sentences in sections I, IV-C, IX and X. |
| UML diagrams (6 design, 3 implementation) | 12-04 | All 9 done (D1 to D6, I1 to I3), drawn from the code. |
| Recorded presentation (about 15 min) and slides | 12-03 | All 14 slides are drafted in [slides.md](presentation/slides.md), with speaker notes and an image for each slide in `presentation/visuals/` (PR #43). The notes take about 10.5 min and the demo 3.5 min. The deck is in Google Slides ("SpareCycles presentation" in Kushagra's Drive, built 2026-11-25 with all 14 images and speaker notes; slide 11 got its own image). Tejas and Abhishek have edit access (2026-11-25). Next: each speaker reads their notes in the deck and changes the words to their own; run-through with a timer and backup demo clips on 12-01. Speakers: Tejas 1, 2, 3, 4 (BOINC), 8, 14; Kushagra 4 (SWIM, VECA), 9, 11, 12; Abhishek 4 (Bully, VECTrust), 5, 6, 7, 10, 13. Demo steps: [demo-script.md](presentation/demo-script.md). |
| Submission README (setup and division of work) | 12-03 | Drafted in [docs/submission.md](submission.md). Each person checks their rows. On 12-03, do its "Before you submit" list. |

## Waiting on

| This | Needs |
| --- | --- |
| Failover test on AWS | First `make deploy-up` (#15) |
| The scheduler skips a volunteer that just lost a lease on that unit (a slow volunteer can take back the unit it lost) | A follow-up to #8: needs a record of lost leases |
| Catch-up waits for a majority of snapshots (today a new leader uses the snapshots that arrive within `SC_REPLICATION_TIMEOUT`; with 2 snapshots at the same index it keeps the first) | A follow-up to #14 (Abhishek). The report lists it as an improvement. |
| Finished jobs leave the task table after the client fetched the result (today each keeps its merged result, about 50 KB for a fractal) | A follow-up to #45 (all; `internal/tasktable`). Only needed for a long-running cluster. The report lists it as an improvement. |

## After merge

Do these right after the named PR merges, in the same day:

| After | Do |
| --- | --- |
| Any PR that changes the coordinator's request path | Kushagra: run `make load-sweep` again from a terminal in the foreground, and update TC-07, TC-08, report 9.3 and slide 12 if the numbers change. |
| Any algorithm issue merges | Update its card and the roadmap on the product page (`site/index.html`). |

## Done

| Date | PR | What |
| --- | --- | --- |
| 2026-11-25 | [#47](https://github.com/colmeabhi/sparecycles/pull/47) | Report cover page: all 3 student IDs and emails; no `\todo` marks left. Deck shared with Tejas and Abhishek (edit access) |
| 2026-11-25 | [#46](https://github.com/colmeabhi/sparecycles/pull/46) | Slide tool: Google Slides. The deck has all 14 slides with titles, images and speaker notes; new image for slide 11 (live demo) |
| 2026-11-25 | [#45](https://github.com/colmeabhi/sparecycles/pull/45) | Task table drops unit outputs it no longer needs: before, a leader crash under load in Docker ran 2 coordinators out of memory and stopped the cluster; now memory stays below 270 MB (TC-12 part 2). Prometheus figure in report 9.3 |
| 2026-11-25 | [#44](https://github.com/colmeabhi/sparecycles/pull/44) | Demo script checked on `main`: all 5 parts pass; expected output, timings and narration updated; `make fail-faulty` now shows the `volunteer blocked` line |
| 2026-11-25 | [#43](https://github.com/colmeabhi/sparecycles/pull/43) | All 14 slides drafted: text, speaker notes and a 1600×900 image for each slide in the product page style |
| 2026-11-25 | [#42](https://github.com/colmeabhi/sparecycles/pull/42) | Full report draft: every section, diagrams D1, D2, D4, D5, I2 and I3, screenshots for 8.1, 8.2b and 8.2c; facts checked against the code and the run records |
| 2026-11-25 | [#41](https://github.com/colmeabhi/sparecycles/pull/41) | After-merge steps: load sweep on the final code (voting costs 32% at 4 volunteers), full mode needs 2 or more volunteers, `loadgen` uses workload defaults, short lease in `fail-leader.sh`, TC-03 and TC-04 rerun on `main`, diagram I1 and slide 12 updated |
| 2026-11-22 | [#34](https://github.com/colmeabhi/sparecycles/pull/34) | Monte Carlo workload: `montecarlo` estimates pi with integer-only sampling, the same bytes on arm64 and amd64 (TC-15; closes #9) |
| 2026-11-24 | [#31](https://github.com/colmeabhi/sparecycles/pull/31) | Lease expiry: `lease.Timeout`; a lost unit moves to another volunteer when its lease ends (TC-13; closes #6) |
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
