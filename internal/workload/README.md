# workload

The types of jobs that SpareCycles can run. A workload cuts a job into units, runs one unit, and joins the outputs.

**Owner:** Tejas (Monte Carlo) · **State:** fractal and Monte Carlo complete · **Issue:** #9 (Monte Carlo)

## How it fits

- `SubmitJob` in the coordinator calls `Split(params)`. Each payload becomes one unit.
- A volunteer calls `Run(ctx, payload)` for each unit it gets.
- When all units are accepted, the coordinator calls `Merge(params, outputs)` with the outputs in unit order.

## Interface

```go
type Workload interface {
    Name() string
    Split(params []byte) ([][]byte, error)
    Run(ctx context.Context, payload []byte) ([]byte, error)
    Merge(params []byte, outputs [][]byte) ([]byte, error)
}
```

## Workloads

| Name | Unit | Result | Parameters (JSON) |
| --- | --- | --- | --- |
| `fractal` | One square tile of a Mandelbrot image | PNG image | `width`, `height`, `tile_size`, `max_iter`, `x_min`, `x_max`, `y_min`, `y_max` |
| `montecarlo` | Random points in a square; count those inside a quarter circle | JSON: `estimate` of pi, `inside`, `total`, `units` | `units` (default 16, up to 10,000), `samples_per_unit` (default 50,000,000, up to 5,000,000,000), `seed` (default 1; unit *i* uses `seed + i`) |

### Monte Carlo

```bash
go run ./cmd/client -workload montecarlo -params '{"units":8,"samples_per_unit":1000000}' -out pi.json
```

```json
{"estimate":3.1419955,"inside":6283991,"total":8000000,"units":8}
```

- Each unit draws its points with `math/rand/v2` `NewPCG(seed + i, constant)`. One 64-bit number gives one point with two 31-bit integer coordinates. The point is inside when `x² + y² < 2⁶²`, which fits in a `uint64`.
- **Only integer math in `Run`**, so every CPU gives the same bytes. `TestMonteCarloGoldenCounts` pins the exact counts for one seed; they are the same on arm64 (laptops) and amd64 (CI). Only `Merge`, which runs once on the leader, uses floating point.
- A unit's output is 16 bytes: `inside` and `total` as big-endian `uint64`. `Merge` checks the count and size of the outputs, and that each unit ran `samples_per_unit` samples.
- Speed: 3 to 8 ns per sample on a laptop, so the limit of 5 billion samples is 15 to 40 seconds per unit. It stays well under the 3-minute lease (`SC_LEASE_DURATION`) on a slower machine too. A unit that takes longer than its lease is given out again and again and never finishes.
- Seeds: unit *i* uses `seed + i`, so 2 jobs with nearby seeds share units (seed 1 and seed 2 with 16 units share 15 of them). Their results are not independent. To average several jobs, give them seeds that differ by at least `units`.
- Every volunteer must run a build that knows `montecarlo`. An older volunteer fails each Monte Carlo unit with `unknown workload`, sends nothing, and the unit waits for its lease.
- Unknown parameters are an error, so a typo such as `sample_per_unit` does not silently run the default size.

## Rules

- `Run` must be deterministic: the same payload gives the same bytes on every volunteer. Voting compares hashes.
- Floating point: Go can fuse `a*b + c` on arm64 but not on amd64. Wrap each product in `float64(...)`, as `mandelbrot` does, or use only integers, as `montecarlo` does.
- Check `ctx.Err()` in long loops. The volunteer cancels the context when its lease ends.
- Validate parameters in `Split` and return a clear error. The client sees it as `InvalidArgument`.

## How to test

```bash
go test -race ./internal/workload/ ./internal/coordinator/ -run 'Fractal|MonteCarlo'
```

`montecarlo_test.go`: split and seeds, defaults, the same bytes on each run, golden counts, the estimate within 0.01 of pi for 10 million samples, bad parameters, bad payloads, bad outputs in merge, and a cancelled context stops `Run`. `internal/coordinator/montecarlo_test.go` runs a job through the coordinator and 3 volunteers and compares it with the same job run in the test.

## Add a workload

1. Add a type that implements `Workload` in a new file.
2. Register it in `Default()`.
3. Add tests: split, run, merge, determinism, bad parameters.
4. Add a row to the table above and an example command to `docs/setup.md`.
