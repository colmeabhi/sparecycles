# SpareCycles

A fault-tolerant volunteer computing platform for the Distributed Systems final project.

A client submits a large compute job and the platform divides it into ~60-second work units that volunteers run in Docker containers. Three replicated coordinators lease work units to volunteers, verify results by majority vote, and recover from coordinator failure via leader election.

**Team:** Tejas Chakkarwar, Kushagra Bainsla, Abhishek Ahirrao

## Documentation

- [Project Proposal](./docs/proposal.md) ([PDF](./docs/SpareCycles_Proposal.pdf))
- [Course Project Instructions](./docs/project-instructions.md) ([PDF](./docs/finalProjectDistributedSys.pdf))

## Planned Stack

Go, gRPC, Docker, Terraform, AWS EC2, Prometheus

## Milestones

| Milestone | Dates | Result |
| --- | --- | --- |
| M1 Core pipeline | Oct 9 – Oct 19 | One coordinator and volunteers run the fractal workload with gRPC |
| M2 Leases, voting, AWS | Oct 20 – Nov 2 | Lease reassignment, majority voting, AWS deployment; working prototype |
| M3 Election, gossip | Nov 3 – Nov 16 | Three coordinators, Bully election, gossip, resource discovery |
| M4 Security, workloads | Nov 17 – Nov 25 | Authentication, signed tasks, malicious volunteer test, more workloads |
| M5 Evaluation | Nov 26 – Dec 4 | Benchmarks, failure tests, final report, presentation |
