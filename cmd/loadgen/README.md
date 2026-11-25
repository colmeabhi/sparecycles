# loadgen (binary)

Submits many jobs with several clients at the same time, waits for all of them, and reports throughput and job time.

**Owner:** Kushagra · **State:** complete · **Issue:** #12

## Run

```bash
go run ./cmd/loadgen -jobs 8 -concurrency 2 -label full -volunteers 2 -csv results/load.csv
```

Usually `scripts/load.sh` runs it for you.

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-jobs` | `8` | Jobs to submit |
| `-concurrency` | `2` | Clients that submit at the same time |
| `-workload` | `fractal` | Workload name |
| `-params` | empty (the workload's defaults) | Workload parameters as JSON. `scripts/load.sh` sends a fixed fractal job (1024×1024, tile 128, 2000 iterations, 64 units), so the results stay comparable. |
| `-label` | `full` | Name of the setup in the CSV |
| `-volunteers` | `0` | Volunteer count, only recorded in the CSV |
| `-csv` | empty | Append the result to this file |
| `-timeout` | `30m` | Maximum time for the whole run |

It reads `SC_COORDINATORS` like the client, and follows the leader through `internal/leaderclient`.

## Output

```text
setup        full, 2 volunteers, 2 clients
jobs         8 (0 failed)
units        512
total time   ...
throughput   ... jobs/min, ... units/s
job time     mean ..., p95 ...
```

CSV columns: `time, label, volunteers, clients, jobs, failed, units, total_s, jobs_per_min, units_per_s, mean_job_s, p95_job_s, cpus, arch`.

## Rules

- Job time is measured from submit to the "done" state, with a 100 ms poll. Shorter jobs have a larger relative error.
- p95 uses the nearest-rank method. `stats_test.go` checks it.
