# Observability

How to watch SpareCycles during a demo, and the queries for the report screenshots.

## Where the data is

| Source | Where |
| --- | --- |
| Metrics | `http://localhost:9400/metrics` on each coordinator (Prometheus text format) |
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
| Rejected registrations | `increase(sc_registrations_rejected_total[5m])` |
| Which node leads | `sc_coordinator_is_leader` |
| Wasted copies (assigned but not needed) | `rate(sc_units_assigned_total[1m]) - rate(sc_results_received_total[1m])` |

## Log lines for screenshots

`grep` these in a coordinator log. Each line has a time, a level and key-value data.

| Event | Log message |
| --- | --- |
| Job lifecycle | `job submitted`, `job finished` |
| Gossip | `member suspect`, `member dead`, `member alive again`, `refuted suspicion` |
| Lost work | `lease released: volunteer dead`, `lease expired` |
| Security | `registration rejected` (coordinator), `bad signature` (volunteer) |
| Voting (after #7) | `wrong result`, `no majority, more copies` |
| Election (after #13) | `election started`, `became leader` |

## Quick view without Prometheus

```bash
curl -s localhost:9400/metrics | grep '^sc_'
```
