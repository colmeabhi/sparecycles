# SpareCycles: submission README

This file is the README for the course submission. It has 2 parts, as the course asks:

1. [How to set up, compile and run the project](#1-set-up-compile-and-run), step by step.
2. [Division of work](#2-division-of-work): which parts each student was responsible for.

**Team:** Kushagra Bainsla, Tejas Chakkarwar, Abhishek Ahirrao. **Course:** Distributed Systems, final project.
**Code:** https://github.com/colmeabhi/sparecycles · **Product page:** https://colmeabhi.github.io/sparecycles/

## What is in the submission

| Item | Where |
| --- | --- |
| Final report (IEEE, PDF) | `docs/report/build/main.pdf` (build it with step 7 below) — *link to the uploaded PDF: to do* |
| Recorded presentation and demo | *Link: to do* |
| Presentation slides (PDF) | *Link: to do* |
| Code | This repository |
| This README | `docs/submission.md` |

## 1. Set up, compile and run

All commands run from the repository folder on macOS or Linux. The defaults work on one machine, so no settings file is necessary.

### Step 1: Install the tools

| Tool | Version | Needed for |
| --- | --- | --- |
| Go | 1.27 or later | Compile, test and run (required) |
| Git, Make, Bash | Any recent | Clone, shortcuts and scripts (required) |
| Docker Desktop | Any recent | Local cluster with Prometheus (optional, step 6) |
| PlantUML and Tectonic | Any recent | Build the report PDF (optional, step 7) |

On macOS:

```bash
brew install go
```

You do not need `protoc`. The generated gRPC code in `gen/` is committed.

### Step 2: Get the code

```bash
git clone https://github.com/colmeabhi/sparecycles.git
```

```bash
cd sparecycles
```

### Step 3: Compile and test

Compile the 5 binaries (`coordinator`, `volunteer`, `client`, `keygen`, `loadgen`) into `bin/`:

```bash
make build
```

Run all tests with the Go race detector:

```bash
make test
```

**Expected:** each package prints `ok` or `no test files`, and no package prints `FAIL`.

### Step 4: Run one job on one machine

Open 4 terminals in the repository folder.

Terminal 1, the coordinator (gRPC on port 7400, metrics on port 9400):

```bash
make run-coordinator
```

Terminals 2 and 3, two volunteers. Each unit runs on 3 volunteers and needs 2 equal results, so start at least 2. Run the same command in more terminals to add more volunteers:

```bash
make run-volunteer
```

Terminal 4, the client. It submits a 1024×1024 fractal job of 16 units:

```bash
make run-client
```

**Expected:** the client prints `16/16 units done` and saves `result.png`. Open it to see the fractal.

Stop each process with Ctrl+C.

### Step 5: Run the failure demos and the load tests

Each script starts its own coordinators and volunteers, prints the important log lines, and ends with `PASS`, `FAIL` or `SKIP`. Logs go to `runs/`.

| Command | What it shows |
| --- | --- |
| `make fail-volunteer` | A volunteer is killed with `kill -9`. Gossip marks it dead, its unit moves to another volunteer, and the job finishes. |
| `make fail-faulty` | A volunteer sends wrong results. Majority voting rejects them. *Prints `SKIP` until issue #7 merges.* |
| `make fail-leader` | The leader coordinator is killed. A backup wins the election and the job finishes. *Prints `SKIP` until issues #13 and #14 merge.* |
| `make load-sweep` | Throughput and latency for 1, 2 and 4 volunteers, with and without redundancy. Appends to `results/load.csv`. |

Run the load tests from a terminal in the foreground. A background run on macOS can measure about half the real throughput.

### Step 6: Run the local cluster with Docker Compose (optional)

This starts 1 coordinator, 3 volunteers and Prometheus. Start Docker Desktop first.

```bash
make up
```

```bash
make run-client
```

Open http://localhost:9090 for Prometheus. The queries are in [observability.md](observability.md). Stop the cluster:

```bash
make down
```

### Step 7: Build the report PDF (optional)

```bash
brew install plantuml tectonic
```

```bash
make report
```

The PDF is `docs/report/build/main.pdf`. You can also upload the `docs/report/` folder to Overleaf and set the main file to `main.tex`.

### Settings

All settings are environment variables that start with `SC_`. To change one, copy the example file and edit it:

```bash
cp .env.example .env
```

`.env.example` explains each variable. To turn on volunteer tokens and signed tasks, follow "Tokens and signed tasks" in [setup.md](setup.md).

### Ports

| Port | Use |
| --- | --- |
| 7400 | Coordinator gRPC |
| 9400 | Coordinator `/metrics` and `/healthz` |
| 9090 | Prometheus (Docker Compose only) |

## 2. Division of work

The team divided the work evenly. Each student owns 2 papers, 2 or 3 algorithms with their tests, metrics and failure scenario, and an equal share of the report and the presentation. All 3 students share the testing.

### Summary

| Student | Papers | Algorithms and code | Other work |
| --- | --- | --- | --- |
| Kushagra Bainsla | SWIM [2], VECA [6] | Gossip membership and failure detection; volunteer tokens and signed tasks | Base version, failure and load scripts, product page |
| Tejas Chakkarwar | BOINC [1], [5] | Leases and reassignment; majority voting; resource discovery | Second workload (Monte Carlo) |
| Abhishek Ahirrao | Bully [3], VECTrust [4] | Leader election; task-table replication | AWS deployment with Terraform; review of resource discovery |

### Code

Each algorithm is a Go package behind an interface. The base version has a simple stub for each package. Each owner replaces their stubs with the real algorithm, its tests and its metrics.

| Part | Package or folder | Student | Issue | Pull request |
| --- | --- | --- | --- | --- |
| Base version: gRPC contract, task table, coordinator, volunteer, client, CI | `proto/`, `internal/tasktable/`, `internal/coordinator/`, `internal/volunteer/`, `cmd/` | Kushagra | — | [#5](https://github.com/colmeabhi/sparecycles/pull/5) |
| SWIM gossip and failure detection | `internal/membership/` | Kushagra | [#10](https://github.com/colmeabhi/sparecycles/issues/10) | [#19](https://github.com/colmeabhi/sparecycles/pull/19) |
| Volunteer tokens and Ed25519 signed tasks | `internal/security/`, `cmd/keygen/` | Kushagra | [#11](https://github.com/colmeabhi/sparecycles/issues/11) | [#20](https://github.com/colmeabhi/sparecycles/pull/20) |
| Session secrets: no requests under another volunteer's ID | `internal/security/`, `internal/coordinator/`, `internal/volunteer/`, `proto/` | Kushagra | [#35](https://github.com/colmeabhi/sparecycles/issues/35) | [#36](https://github.com/colmeabhi/sparecycles/pull/36) |
| Failure scripts, load generator and baseline mode | `scripts/`, `cmd/loadgen/`, `results/` | Kushagra | [#12](https://github.com/colmeabhi/sparecycles/issues/12) | [#21](https://github.com/colmeabhi/sparecycles/pull/21), [#26](https://github.com/colmeabhi/sparecycles/pull/26) |
| Lease expiry and reassignment | `internal/lease/` | Tejas | [#6](https://github.com/colmeabhi/sparecycles/issues/6) | *In progress* |
| Majority voting | `internal/verify/` | Tejas | [#7](https://github.com/colmeabhi/sparecycles/issues/7) | [#32](https://github.com/colmeabhi/sparecycles/pull/32) |
| Resource discovery with trust scores | `internal/scheduler/` | Tejas (Abhishek reviews) | [#8](https://github.com/colmeabhi/sparecycles/issues/8) | [#33](https://github.com/colmeabhi/sparecycles/pull/33) |
| Monte Carlo workload | `internal/workload/` | Tejas | [#9](https://github.com/colmeabhi/sparecycles/issues/9) | *In progress* |
| Bully election with terms | `internal/election/` | Abhishek | [#13](https://github.com/colmeabhi/sparecycles/issues/13) | *In progress* |
| Task-table replication | `internal/replication/` | Abhishek | [#14](https://github.com/colmeabhi/sparecycles/issues/14) | *In progress* |
| AWS deployment with Terraform | `deploy/` | Abhishek | [#15](https://github.com/colmeabhi/sparecycles/issues/15) | *In progress* |
| Product page and documentation | `site/`, `docs/`, module READMEs | Kushagra | — | [#16](https://github.com/colmeabhi/sparecycles/pull/16), [#17](https://github.com/colmeabhi/sparecycles/pull/17) |

### Report

The full plan with page budgets is in [report/outline.md](report/outline.md). Each student writes about 4 pages.

| Section | Kushagra | Tejas | Abhishek |
| --- | --- | --- | --- |
| 1–3 Cover, goals, challenges | Cover page | Goals and motivation | Challenges addressed |
| 4 Literature review | SWIM, VECA | BOINC | Bully, VECTrust |
| 5.1 Design goals | Security, failure handling, openness | Heterogeneity, quality of service, scalability | Concurrency, transparency |
| 5.2, 6.1, 9 Algorithms | Gossip, tokens and signed tasks | Leases, voting, resource discovery | Election, replication |
| 6.2 Properties | Fault tolerance, security | Performance, scalability | Consistency, concurrency |
| 7 Implementation | 7.1 Architectural style, 7.3 Components and gRPC | — | 5.3 Architecture text |
| 8 Scenarios | 8.2a Volunteer crash | 8.1 Success, 8.2b Faulty volunteer | 8.2c Leader crash |
| 9–11 Results and conclusions | 9.2–9.3 Metrics and workloads | 10 Test cases | 9.1 Reflection, 11 Conclusions |
| UML diagrams | D3 job sequence, D6 SWIM states, I1 class | D1 use case, D4 unit states, I3 object | D2 deployment, D5 leader crash, I2 component |

### Presentation

The full plan is in [presentation/outline.md](presentation/outline.md). Each student speaks about 5 minutes.

| Student | Slides |
| --- | --- |
| Kushagra | 4 (SWIM, VECA), 9 Gossip and security, 11 Live demo (drives; narrates the volunteer crash and scaling), 12 Evaluation |
| Tejas | 1 Title, 2 Problem, 3 Goals, 4 (BOINC), 8 Leases, voting and resource discovery, 11 (narrates the faulty volunteer), 14 Summary |
| Abhishek | 4 (Bully, VECTrust), 5 Architecture, 6 Life of a job, 7 Design choices, 10 Election and replication, 11 (narrates the leader crash), 13 Next steps |

### Shared work

All 3 students share the test cases in [test-results.md](test-results.md) and the final editing of the report and the slides. Each student adds the test cases for their own algorithms.

## Before you submit

Remove this section after the last item is done.

- [ ] Each "In progress" row in "Code" has a merged pull request.
- [ ] `make fail-faulty` and `make fail-leader` print `PASS`.
- [ ] Add the links to the report PDF, the recording and the slides.
- [ ] Add an AWS run step from `deploy/` after issue #15 merges.
- [ ] Check that the report and slide owners match the final versions.
