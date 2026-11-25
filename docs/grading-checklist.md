# Grading checklist

This checklist lists each requirement in [project-instructions.md](project-instructions.md). The instructions give no point values, so treat each item as required.

Mark an item `[x]` only when the evidence exists in the repo or in the submission.

## 1. Code

- [x] At least 3 algorithms from the course list. The plan has 6: leader election (5), gossip (3), resource allocation (8), resource discovery (10), agreement (11), replication and consistency (12). *(Real on `main`: gossip, leader election, replication, and agreement with majority voting.)*
- [ ] Each algorithm is written by the team, not taken from a library.
- [ ] The code handles fault tolerance, performance, scalability, availability, consistency and concurrency.
- [ ] The system runs on AWS EC2 with 3 coordinators and many volunteers.
- [x] At least 5 test cases, with recorded results. *(`docs/test-results.md`: all 15 cases have results.)*

## 2. Evidence that the code must produce

The report and the presentation need this evidence. Build it together with the features, not at the end.

- [ ] **Metrics** in Prometheus: jobs per minute, unit latency, lease expiries, reassignments, wrong results caught, election time. *(All are in. Election time is `sc_election_failover_seconds`; see `docs/observability.md`.)*
- [x] **Baseline mode** for comparison, for example 1 coordinator and no voting. The report must compare the system "ideally" to another system. *(`scripts/load.sh ... baseline`)*
- [x] **Load scripts** that increase the number of volunteers and the number of clients. The report needs throughput and latency as load grows. *(`scripts/load.sh`, `scripts/load-sweep.sh`, `results/load.csv`)*
- [x] **Failure scripts** for 3 failure scenarios: kill a volunteer, run a malicious volunteer, kill the leader. *(`fail-volunteer.sh`, `fail-faulty.sh` and `fail-leader.sh` all pass.)*
- [x] **Clear logs or a dashboard** for screenshots of 1 success scenario and the 3 failure scenarios. *(`docs/observability.md`: Prometheus queries and log lines)*

## 3. Final report (IEEE double column, 10 to 15 pages without images)

- [ ] 1. Cover page: names, student IDs and email addresses.
- [ ] 2. Project goals and motivation.
- [ ] 3. Distributed systems challenges addressed.
- [ ] 4. Detailed literature review.
- [ ] 5.1 Design goals for each challenge: heterogeneity, openness, security, failure handling, concurrency, quality of service, scalability and transparency.
- [ ] 5.2 Key components and algorithms.
- [ ] 5.3 Architecture with at least 3 design UML diagrams: use case, deployment or component, communication, sequence, activity or state.
- [ ] 6.1 Why the approach is good: intuitive description, correctness and complexity of each algorithm.
- [ ] 6.2 How the project handles fault tolerance, performance, scalability, consistency, concurrency and security.
- [ ] 7.1 Architectural style.
- [ ] 7.2 At least 3 implementation UML diagrams: class, object, interaction.
- [ ] 7.3 Software components and how they interact (gRPC).
- [ ] 8.1 1 success scenario, with screenshots.
- [ ] 8.2 3 failure scenarios, with screenshots and the system response.
- [ ] 9.1 Overall result and reflection on design decisions.
- [ ] 9.2 Metrics, ideally compared to another system.
- [ ] 9.3 Simulated workloads with measured throughput and latency.
- [ ] 9 Growth rate of the major algorithms.
- [ ] 10.1 At least 5 test cases, explained.
- [ ] 10.2 Test results and discussion.
- [ ] 11. Conclusions, lessons learned and possible improvements.

## 4. Recorded presentation (about 15 minutes)

- [ ] Introduce the project and the problem.
- [ ] Related work.
- [ ] Key design choices and how they relate to the distributed systems challenges.
- [ ] Components, architecture and features.
- [ ] Live demo: crash a node, scale concurrent access, show performance as users increase.
- [ ] Evaluation with metrics.
- [ ] Next steps.
- [ ] Summary of key points.

## 5. Submission (one per team)

- [ ] Final report.
- [ ] Recorded presentation and demo.
- [ ] Presentation slides.
- [ ] Code.
- [ ] Submission README: setup and compile steps, step-by-step run instructions, and the division of work for each student. *(Drafted in `docs/submission.md`; final after its "Before you submit" list.)*

## 6. Literature knowledge

Each student must explain their own papers in detail: each step of the algorithm, what fails if a step is removed, good and bad scenarios, and the main idea of the analysis.

- [ ] Kushagra: SWIM [2], VECA [6].
- [ ] Tejas: BOINC [1], [5].
- [ ] Abhishek: Bully [3], VECTrust [4].
