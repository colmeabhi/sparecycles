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

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-workload` | `fractal` | Workload name (see `internal/workload`) |
| `-params` | 1024×1024 fractal | Workload parameters as JSON |
| `-out` | `result.png` | File for the result |
| `-poll` | `1s` | Time between progress checks |
| `-timeout` | `30m` | Maximum time to wait |

## Settings

`SC_COORDINATORS` (default `localhost:7400`). The client finds the leader itself.
