# Project guide

Read this file before you start work on SpareCycles.

## The project

SpareCycles is a fault-tolerant volunteer computing platform. It is the final project for a Distributed Systems course.

- [docs/proposal.md](docs/proposal.md) is the proposal that the team submitted to the professor. It defines the scope.
- [docs/project-instructions.md](docs/project-instructions.md) is the assignment from the professor. It defines the requirements for the code, the report and the presentation.
- [README.md](README.md) is a short note for the project folder. It is not the course submission README.

- [docs/grading-checklist.md](docs/grading-checklist.md) lists each graded requirement. Check it before you plan or finish work.

If this file and the proposal disagree on scope, the proposal is correct.

## Team and ownership

The team divides the work evenly across all 3 people. When you create or assign issues, keep the amount of work for each person approximately equal.

| Person | Papers | Algorithms | Other work |
| --- | --- | --- | --- |
| Kushagra Bainsla | SWIM [2], VECA [6] | Gossip membership, failure detection | Base version, security |
| Tejas Chakkarwar | BOINC [1], [5] | Leases and reassignment, majority voting, resource discovery | Second workload |
| Abhishek Ahirrao | Bully [3], VECTrust [4] | Leader election, task-table replication | AWS and Terraform setup |

All 3 people share the report, the presentation and the testing.

Resource discovery is based on VECTrust [4]. Tejas writes the code, and Abhishek reviews those PRs.

Do not change code in an area that another person owns. If a change is necessary, say so in the issue or the PR.

## Workflow

1. One person builds a base version first. It contains the full `.proto` file, the Go interfaces with simple stub implementations, one end-to-end run and CI.
2. The team creates one GitHub issue for each stub. Each issue names the owner, the package to change and the acceptance test.
3. Each person works on the issues assigned to them, on a separate branch.
4. Each PR closes one issue and changes one package where possible.
5. A teammate reviews each PR. CI must pass before the merge.

### Definition of done

A feature is done only when it has all of these:

- Tests that pass, including a failure case where one applies.
- Prometheus metrics for its main events, for example lease expiry or election time.
- Clear log lines that show the feature work in a demo.
- A way to trigger its failure scenario, if it has one.
- An update to `docs/grading-checklist.md` if it completes an item.

## Rules

- Do not mention AI or agents anywhere in the project. This includes commit messages, PR descriptions, code comments, documents and the README.
- Do not add a `Co-Authored-By` line for an AI tool to commits.
- Do not add a "Generated with" line to PRs.
- Do not change `docs/proposal.md` or `docs/project-instructions.md`. They are the submitted and assigned texts.
- Do not commit secrets, AWS keys or Terraform state.

## Design decisions

The team agreed on these decisions. Follow them. If a change is necessary, discuss it with the team first.

| Topic | Decision |
| --- | --- |
| Split brain | Each leader has a term number. A leader must reach 2 of 3 coordinators to act. A node rejects messages from an older term. |
| Replication | The leader confirms a task-table change after 1 backup has it (2 of 3 copies). |
| Gossip | Volunteers and coordinators gossip together, in SWIM style. |
| No majority | If no 2 of 3 results agree, the leader sends the unit to 2 more volunteers. |
| Lease length | 3 minutes for a 60-second unit. |
| Work assignment | Pull. Volunteers ask the leader for work. |
| Workloads | Fractal tiles first. Monte Carlo second. |
| Security | Each volunteer has a token. The leader signs each task. |

## Stack

Go, gRPC, Docker, Terraform, AWS EC2 and Prometheus. The team writes all distributed algorithms itself. Do not use a library for election, gossip, consensus or replication.

## Deadlines

| Item | Due |
| --- | --- |
| Code submission | 2026-12-03 |
| Recorded presentation | 2026-12-03 |
| Final report | 2026-12-04 |
