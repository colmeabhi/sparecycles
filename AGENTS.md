# Project guide

Read this file before you start work on SpareCycles. Then read [docs/status.md](docs/status.md): it says what is left, who does it next, and what blocks it.

## The project

SpareCycles is a fault-tolerant volunteer computing platform. It is the final project for a Distributed Systems course.

- [docs/proposal.md](docs/proposal.md) is the proposal that the team submitted to the professor. It defines the scope.
- [docs/project-instructions.md](docs/project-instructions.md) is the assignment from the professor. It defines the requirements for the code, the report and the presentation.
- [README.md](README.md) is lean: a one-line pitch, quick start, doc links and the team. Do not add product detail there. It is not the course submission README.
- [site/](site/) is the product page (GitHub Pages). Product detail goes there. Keep it true: mark unfinished features as in progress.
- [docs/grading-checklist.md](docs/grading-checklist.md) lists each graded requirement. Check it before you plan or finish work.
- [docs/status.md](docs/status.md) is the shared status: deadlines, next step per person, dependencies, steps to run after a merge, and a log of merged PRs.

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

## Code layout

Each part is a Go interface with a simple stub. Replace the stub with the real algorithm in the same package. Keep the interface, or agree on a change with the team first.

Each folder in the table has a `README.md`. Read it before you change the folder. It explains what the module is for, how other code uses it, its settings, its metrics and its rules.

| Path | What it does | State | Owner |
| --- | --- | --- | --- |
| `proto/sparecycles/v1/` | gRPC contract for all services | Complete | All (discuss changes) |
| `gen/sparecycles/v1/` | Generated code. Run `make proto` after a `.proto` change. Do not edit by hand. | Complete | All |
| `internal/tasktable/` | Jobs and units. Changes only through `Change` and `Apply`. | Complete | All (discuss changes) |
| `internal/coordinator/` | Client and volunteer RPCs. Connects all parts. | Complete | All (discuss changes) |
| `internal/lease/` | Lease length and expiry | Done: `Timeout` (`NoExpiry` kept for tests) | Tejas |
| `internal/verify/` | Majority voting | `FirstResult`: accepts the first result | Tejas |
| `internal/scheduler/` | Resource discovery: which unit goes to which volunteer | `FIFO`: oldest unit first | Tejas |
| `internal/workload/` | Workloads: split, run, merge | Fractal complete | Tejas (Monte Carlo) |
| `internal/membership/` | SWIM gossip and failure detection | Done: `SWIM` (`Static` kept for tests) | Kushagra |
| `internal/security/` | Volunteer tokens and task signatures | Done: `TokenList`, Ed25519 (`NoSign` when no key) | Kushagra |
| `internal/election/` | Bully election with terms | `Static`: highest node ID | Abhishek |
| `internal/replication/` | Copy table changes to backups | `Noop`: copies nothing | Abhishek |
| `deploy/` | Prometheus config, later Terraform | Prometheus only | Abhishek (Terraform) |
| `cmd/` | Binaries: coordinator, volunteer, client, keygen, loadgen | Complete | All |
| `scripts/` | One-command failure demos and load tests | Complete | Kushagra |
| `results/` | Load test results (`load.csv`), kept in git for the report | Data | All |
| `site/` | Product page for GitHub Pages (one HTML file, pixel tile style) | Complete | All |

To replace a stub, add the new type in the package and use it in `cmd/coordinator/main.go` (or `cmd/volunteer/main.go`).

## Run and configure

- [docs/setup.md](docs/setup.md) has the setup steps. The repo must build, test and run right after a clone.
- All settings are environment variables that start with `SC_`. Defaults must work on one machine.
- When you add a variable, add it to `.env.example` with a comment. Read it in `internal/config/`.
- `.env` is local and is not committed. CI uses GitHub secrets or variables with the same names.
- Before you push, run `make test` and `make vet`. Run `gofmt` on changed files.

## Code quality

Follow these principles in all code:

- **One job per package, type and function.** If a function does two things, split it.
- **Small, clear interfaces.** Depend on interfaces, not on concrete types, between packages (see `coordinator.Deps`).
- **Clear names.** Use full words. A name says what the thing is or does. Avoid abbreviations except common ones (`ctx`, `err`, `id`).
- **Short functions.** If a function does not fit on one screen, split it.
- **No duplicate logic.** Put shared logic in one place and call it.
- **Handle every error.** Return it with context (`fmt.Errorf("replicate %s: %w", kind, err)`) or log it. Never ignore an error without a comment that says why.
- **No global state.** Pass dependencies in. This keeps code testable.
- **Safe for concurrency.** Protect shared state with a mutex, and run `go test -race`.
- **Comments explain why, not what.** Every exported name has a doc comment.
- **Simple language** in comments, logs, docs and READMEs.
- **Tests for each behavior,** including failure cases. Tests must not depend on timing luck: use short timeouts with a clear deadline, or a fake clock.
- **No dead code.** Delete unused code. Git keeps the history.

## Module READMEs

Each module folder (each package in `internal/`, each binary in `cmd/`, and `proto/`, `gen/`, `deploy/`) has a `README.md`.

- Keep it up to date in the same PR as the code change. A reviewer must reject a PR that changes a module but not its README when the README is now wrong.
- Use this order: one-line purpose; owner, state and issue; how it fits (who calls it); interface or main types; implementations; settings; metrics; rules; how to test.
- Use simple language and short sentences. Show tables and small code blocks, not long text.
- A new folder needs a `README.md` in the same PR.

## Workflow

1. One person builds a base version first. It contains the full `.proto` file, the Go interfaces with simple stub implementations, one end-to-end run and CI. (Done.)
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
- An updated `README.md` in each module folder it changed (state, implementations, settings, metrics).
- An updated product page (`site/index.html`) if users can see the change, for example a finished algorithm or a new workload. Update the roadmap section too.
- An update to `docs/grading-checklist.md` if it completes an item.
- An update to `docs/status.md` in the same PR: a "Done" line with the date and PR link, the new next step, and any new "Waiting on" or "After merge" entry.

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
