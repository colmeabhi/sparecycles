# metrics

Defines the Prometheus metrics. The coordinator serves them on `SC_METRICS_ADDR` (default `:9400`) at `/metrics`.

**Owner:** all · **State:** coordinator metrics complete

## Coordinator metrics

| Metric | Type | Meaning |
| --- | --- | --- |
| `sc_coordinator_is_leader` | gauge | 1 on the leader, else 0 |
| `sc_jobs_submitted_total` | counter | Jobs submitted |
| `sc_jobs_finished_total{state}` | counter | Jobs finished, by `done` or `failed` |
| `sc_job_duration_seconds` | histogram | Time from submit to result |
| `sc_volunteers_registered_total` | counter | Volunteer registrations |
| `sc_units_assigned_total` | counter | Leases given out |
| `sc_results_received_total` | counter | Results recorded |
| `sc_units_accepted_total` | counter | Units accepted |
| `sc_wrong_results_total` | counter | Results that lost the vote |
| `sc_leases_expired_total` | counter | Leases released after expiry |
| `sc_replication_errors_total` | counter | Changes that could not be replicated |

## Add a metric

1. Add a field to `Coordinator` and create it in `NewCoordinator` with a name that starts with `sc_` and a clear `Help` text.
2. Register it in the same function.
3. Update it where the event happens.
4. Add a row to the table above.

Each `NewCoordinator` call makes its own registry, so tests can create many coordinators.
