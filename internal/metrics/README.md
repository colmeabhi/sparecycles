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
| `sc_registrations_rejected_total` | counter | Registrations rejected for a bad token |
| `sc_units_assigned_total` | counter | Leases given out |
| `sc_results_received_total` | counter | Results recorded |
| `sc_units_accepted_total` | counter | Units accepted |
| `sc_wrong_results_total` | counter | Results that lost the vote |
| `sc_leases_expired_total` | counter | Leases released after expiry |
| `sc_leases_dead_volunteer_total` | counter | Leases released because gossip marked the volunteer dead |
| `sc_replication_errors_total` | counter | Changes that could not be replicated |

## Gossip metrics (`Gossip`, part of `Coordinator`)

| Metric | Type | Meaning |
| --- | --- | --- |
| `sc_gossip_members{state}` | gauge | Known members, by `alive`, `suspect` or `dead` |
| `sc_gossip_pings_total` | counter | Direct pings sent |
| `sc_gossip_ping_failures_total` | counter | Direct pings with no valid ack |
| `sc_gossip_indirect_probes_total` | counter | Indirect ping requests sent |
| `sc_gossip_refutations_total` | counter | Times this node refuted a suspicion about itself |
| `sc_gossip_detection_seconds` | histogram | Time from the last contact with a member to marking it dead |

`NewGossip(reg)` creates them in any registry. Volunteers create them in a private registry, because they serve no metrics endpoint.

## Add a metric

1. Add a field to `Coordinator` and create it in `NewCoordinator` with a name that starts with `sc_` and a clear `Help` text.
2. Register it in the same function.
3. Update it where the event happens.
4. Add a row to the table above.

Each `NewCoordinator` call makes its own registry, so tests can create many coordinators.
