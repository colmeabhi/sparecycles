# Project guide

Read this file before you start work on SpareCycles.

## The project

SpareCycles is a fault-tolerant volunteer computing platform. It is the final project for a Distributed Systems course.

- [docs/proposal.md](docs/proposal.md) is the proposal that the team submitted to the professor. It defines the scope.
- [docs/project-instructions.md](docs/project-instructions.md) is the assignment from the professor. It defines the requirements for the code, the report and the presentation.
- [README.md](README.md) is a short note for the project folder. It is not the course submission README.

If this file and the proposal disagree on scope, the proposal is correct.

## Team and ownership

Each person owns the algorithms from the papers that they study in the proposal.

| Person | Papers | Areas |
| --- | --- | --- |
| Tejas Chakkarwar | BOINC [1], [5] | Work units, leases and reassignment, majority voting |
| Kushagra Bainsla | SWIM [2], VECA [6] | Gossip membership, failure detection, volunteer churn |
| Abhishek Ahirrao | Bully [3], VECTrust [4] | Leader election, task-table replication, resource discovery |

Do not change code in an area that another person owns. If a change is necessary, say so in the issue or the PR.

## Workflow

1. One person builds a base version first. It contains the full `.proto` file, the Go interfaces with simple stub implementations, one end-to-end run and CI.
2. The team creates one GitHub issue for each stub. Each issue names the owner, the package to change and the acceptance test.
3. Each person works on the issues assigned to them, on a separate branch.
4. Each PR closes one issue and changes one package where possible.
5. A teammate reviews each PR. CI must pass before the merge.

## Rules

- Do not mention AI or agents anywhere in the project. This includes commit messages, PR descriptions, code comments, documents and the README.
- Do not add a `Co-Authored-By` line for an AI tool to commits.
- Do not add a "Generated with" line to PRs.
- Do not change `docs/proposal.md` or `docs/project-instructions.md`. They are the submitted and assigned texts.
- Do not commit secrets, AWS keys or Terraform state.

## Stack

Go, gRPC, Docker, Terraform, AWS EC2 and Prometheus. The team writes all distributed algorithms itself. Do not use a library for election, gossip, consensus or replication.

## Deadlines

| Item | Due |
| --- | --- |
| Code submission | 2026-12-03 |
| Recorded presentation | 2026-12-03 |
| Final report | 2026-12-04 |
