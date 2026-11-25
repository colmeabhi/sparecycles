# Observability

How to watch SpareCycles during a demo, and the queries for the report screenshots.

## Where the data is

| Source | Where |
| --- | --- |
| Metrics | `http://localhost:9400/metrics` for `make run-coordinator`; ports 9401 to 9403 for the 3 Docker Compose coordinators (Prometheus text format) |
| Prometheus | `http://localhost:9090` with `make up` (Docker Compose) |
| Logs | stderr of each process. The scripts keep them in `runs/<script>-<time>/` |

## Prometheus queries

Paste these into the Prometheus web page (Graph tab). Each one makes one panel for the report.

| Panel | Query |
| --- | --- |
| Throughput (units per second) | `rate(sc_units_accepted_total[1m])` |
| Jobs finished per minute | `60 * rate(sc_jobs_finished_total{state="done"}[5m])` |
| Job time, p95 | `histogram_quantile(0.95, rate(sc_job_duration_seconds_bucket[5m]))` |
| Members by state | `sc_gossip_members` |
| Failure detection time, p95 | `histogram_quantile(0.95, rate(sc_gossip_detection_seconds_bucket[10m]))` |
| Units moved after a volunteer died | `increase(sc_leases_dead_volunteer_total[5m])` |
| Units moved after a lease expired | `increase(sc_leases_expired_total[5m])` |
| Wrong results caught | `increase(sc_wrong_results_total[5m])` |
| Votes with no majority | `increase(sc_votes_no_majority_total[5m])` |
| Units by volunteer speed | `sum by (speed) (increase(sc_scheduler_picks_total[5m]))` |
| Requests refused, by reason | `sum by (reason) (increase(sc_scheduler_refusals_total[5m]))` |
| Volunteers blocked for low trust | `sc_volunteers_blocked_total` |
| Units that wait for a new volunteer | `sc_units_without_volunteer` |
| Rejected registrations | `increase(sc_registrations_rejected_total[5m])` |
| Requests with a bad session secret | `increase(sc_requests_rejected_total[5m])` |
| Which node leads | `sc_coordinator_is_leader` |
| Election term | `sc_election_term` |
| Elections started and won | `increase(sc_elections_started_total[5m])`, `increase(sc_elections_won_total[5m])` |
| Failover time (old leader silent to new leader acts), p95 | `histogram_quantile(0.95, rate(sc_election_failover_seconds_bucket[10m]))` |
| Replication time per change, p95 | `histogram_quantile(0.95, rate(sc_replication_seconds_bucket[1m]))` |
| Backup lag (changes behind the leader) | `sc_replication_backup_lag` |
| Snapshots served and restored | `increase(sc_replication_snapshots_served_total[5m])`, `increase(sc_replication_snapshots_restored_total[5m])` |
| Wasted copies (assigned but not needed) | `rate(sc_units_assigned_total[1m]) - rate(sc_results_received_total[1m])` |
| Coordinator memory (the task table is most of it) | `process_resident_memory_bytes` |

The report figure in section 9.3 (`docs/report/screenshots/s5-prometheus.png`) shows throughput, `sc_gossip_members{state="alive"}` and memory. To make it again, see TC-12 part 2 in `docs/test-results.md`.

## Log lines for screenshots

`grep` these in a coordinator log. Each line has a time, a level and key-value data.

| Event | Log message |
| --- | --- |
| Job lifecycle | `job submitted`, `job finished` |
| Gossip | `member suspect`, `member dead`, `member alive again`, `refuted suspicion` |
| Lost work | `lease released: volunteer dead`, `lease expired` |
| Security | `registration rejected` and `request rejected: bad session` (coordinator), `bad signature` and `leader rejected the session` (volunteer) |
| Voting | `wrong result` (`late=true` for a late copy), `no majority, more copies`, `unit stuck: every volunteer already ran it` |
| Trust | `volunteer blocked: low trust` |
| Election | `election started`, `became leader`, `new leader`, `stepped down` (each with the term) |
| Replication | `leader continues from table index`, `snapshot restored`, `backup behind` |

## Quick view without Prometheus

```bash
curl -s localhost:9400/metrics | grep '^sc_'
```
