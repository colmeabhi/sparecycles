# client (binary)

Submits one job, shows the progress, and saves the result to a file.

**Owner:** all · **State:** complete

## Run

```bash
make run-client
```

Or with your own values:

```bash
go run ./cmd/client -workload fractal -params '{"width":2048,"height":2048,"tile_size":256}' -out big.png
```

```bash
go run ./cmd/client -workload montecarlo -params '{"units":8,"samples_per_unit":1000000}' -out pi.json
```

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-workload` | `fractal` | Workload name (see `internal/workload`) |
| `-params` | empty: the workload's defaults (fractal: 1024×1024, tiles of 256, 1000 iterations) | Workload parameters as JSON |
| `-out` | `result.png` | File for the result |
| `-poll` | `1s` | Time between progress checks |
| `-timeout` | `30m` | Maximum time to wait |

## Settings

`SC_COORDINATORS` (default `localhost:7400`). The client finds the leader itself.
