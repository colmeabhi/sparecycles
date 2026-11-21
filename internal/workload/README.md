# workload

The types of jobs that SpareCycles can run. A workload cuts a job into units, runs one unit, and joins the outputs.

**Owner:** Tejas (Monte Carlo) · **State:** fractal complete · **Issue:** #9 (Monte Carlo)

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

## Rules

- `Run` must be deterministic: the same payload gives the same bytes on every volunteer. Voting compares hashes.
- Floating point: Go can fuse `a*b + c` on arm64 but not on amd64. Wrap each product in `float64(...)`, as `mandelbrot` does, or return integers.
- Check `ctx.Err()` in long loops. The volunteer cancels the context when its lease ends.
- Validate parameters in `Split` and return a clear error. The client sees it as `InvalidArgument`.

## Add a workload

1. Add a type that implements `Workload` in a new file.
2. Register it in `Default()`.
3. Add tests: split, run, merge, determinism, bad parameters.
4. Add a row to the table above and an example command to `docs/setup.md`.
