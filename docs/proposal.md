# SpareCycles: A Fault-Tolerant Volunteer Computing Platform

**Team Members:** Tejas Chakkarwar, Kushagra Bainsla, Abhishek Ahirrao

> Source: [`SpareCycles_Proposal.pdf`](./SpareCycles_Proposal.pdf)

## 1. Problem Description

SpareCycles is a volunteer computing platform. A client submits a large compute job. The platform divides the job into work units of approximately 60 seconds each. Volunteer computers run the work units and send back the results. Example workloads are fractal image tiles, Monte Carlo simulations, and text embeddings.

The platform must give correct results when three problems occur:

- Volunteers can stop at any time.
- Volunteers can send incorrect results because of a fault or on purpose.
- The coordinator that assigns the work can fail.

## 2. Motivation

Most computers are idle for a large part of the day. SETI@home and Folding@home use this idle capacity for research. Recent systems such as VECA [6] run scientific workflows on volunteer edge resources.

The problem is challenging because it has many distributed systems problems at the same time. The nodes have different hardware, the membership changes continuously, failures are frequent, and some nodes are malicious. Students and researchers benefit from a solution because they get a large quantity of compute power at a low cost.

## 3. Previous Work

BOINC [1], [5] is the standard volunteer computing platform. A central server sends each work unit to more than one volunteer and compares the results. SWIM [2] uses gossip and indirect probes to find failed nodes with a constant message load per node. The Bully algorithm [3] elects a new coordinator when the current coordinator fails. VECTrust [4] allocates volunteer resources based on trust, and VECA [6] uses fail-over to handle volunteer nodes that leave.

BOINC depends on a central project server. SpareCycles replicates the coordinator on three nodes and elects a new leader automatically. Tejas studies [1] and [5]. Kushagra studies [2] and [6]. Abhishek studies [3] and [4].

## 4. Methodology and Plan

### Architecture

A client submits a job to a cluster of three coordinators in three AWS availability zones (Figure 1). The leader divides the job into work units, leases each unit to three volunteers, and verifies the results by majority vote. The leader replicates its task table to two backups. Volunteers run the work units in Docker containers.

### Algorithms

We write all algorithms ourselves. The project uses six algorithms from the course list.

| Algorithm | Function in SpareCycles |
| --- | --- |
| Leader election, Bully [3] | Selects a new leader when the leader fails |
| Gossip, SWIM style [2] | Finds failed volunteers |
| Resource discovery [4] | Matches work units to volunteer CPU, memory, and speed |
| Resource allocation, leases | Gives lost work to a different volunteer when a lease expires |
| Replication and consistency | Copies each task table change to the backups before confirmation |
| Agreement, majority voting [5] | Accepts a result when two of three volunteers agree |

### Implementation

We use Go with gRPC. The leader compares hashes for deterministic results. It uses a small tolerance for floating-point results.

### Deployment

Coordinators and volunteers run on AWS EC2. Volunteers use spot instances, on-demand instances, and our laptops. Terraform creates and removes the cluster.

## 5. Milestones and Schedule

| Milestone | Dates | Result |
| --- | --- | --- |
| M1 Core pipeline | Oct 9 to Oct 19 | One coordinator and volunteers run the fractal workload with gRPC |
| M2 Leases, voting, AWS | Oct 20 to Nov 2 | Lease reassignment, majority voting, AWS deployment. Working prototype. |
| M3 Election, gossip | Nov 3 to Nov 16 | Three coordinators, Bully election, gossip, resource discovery |
| M4 Security, workloads | Nov 17 to Nov 25 | Authentication, signed tasks, malicious volunteer test, more workloads |
| M5 Evaluation | Nov 26 to Dec 4 | Benchmarks, failure tests, final report, presentation |

## 6. Resources

- **Cloud:** AWS EC2, with three coordinator instances and spot and on-demand volunteer instances
- **Hardware:** our laptops, as additional volunteers
- **Software:** Go, gRPC, Docker, Terraform, Prometheus, and GitHub

## 7. References

1. D. P. Anderson, "BOINC: A System for Public-Resource Computing and Storage," in *Proc. 5th IEEE/ACM Int. Workshop on Grid Computing*, 2004, pp. 4–10. <https://doi.org/10.1109/GRID.2004.14>
2. A. Das, I. Gupta, and A. Motivala, "SWIM: Scalable Weakly-consistent Infection-style Process Group Membership Protocol," in *Proc. IEEE DSN*, 2002, pp. 303–312. <https://doi.org/10.1109/DSN.2002.1028914>
3. H. Garcia-Molina, "Elections in a Distributed Computing System," *IEEE Trans. Computers*, vol. C-31, no. 1, pp. 48–59, 1982. <https://doi.org/10.1109/TC.1982.1675885>
4. A. Pandey, P. Calyam, S. Debroy, S. Wang, and M. Lemus Alarcon, "VECTrust: Trusted Resource Allocation in Volunteer Edge-Cloud Computing Workflows," in *Proc. 14th IEEE/ACM Int. Conf. on Utility and Cloud Computing (UCC)*, 2021, pp. 19:1–19:10. <https://par.nsf.gov/servlets/purl/10314474>
5. D. P. Anderson, "BOINC: A Platform for Volunteer Computing," *Journal of Grid Computing*, vol. 18, pp. 99–122, 2020. <https://doi.org/10.1007/s10723-019-09497-9>
6. H. S. Yeddulapalli, M. Lemus Alarcon, U. Roy, R. L. Neupane, D. Gafurov, M. Mounesan, S. Debroy, and P. Calyam, "VECA: Reliable and Confidential Resource Clustering for Volunteer Edge-Cloud Computing," in *Proc. IEEE Int. Conf. on Cloud Engineering (IC2E)*, 2024, pp. 152–159. <https://doi.org/10.1109/IC2E61754.2024.00024>
