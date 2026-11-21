# logging

Creates the structured logger (`log/slog`) for every binary.

**Owner:** all · **State:** complete

## Use

```go
log := logging.New("coordinator")
log.Info("job submitted", "job", jobID, "units", n)
```

Each line has `time`, `level`, `msg`, `component` and your key-value pairs.

## Settings

| Variable | Default | Values |
| --- | --- | --- |
| `SC_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Rules

- Use short messages in lower case, for example `lease expired`. The demo and the report search for them.
- Put data in key-value pairs, not in the message.
- Use `info` for events that a demo shows, `debug` for details, `warn` for problems the system handles, and `error` for problems it does not.
