# volunteer (binary)

Runs one volunteer. It registers with the leader, pulls units, runs them and sends the results.

**Owner:** all · **State:** complete

## Run

```bash
make run-volunteer
```

Run the command in more terminals to add more volunteers.

## What `main` does

1. Loads `.env` and the volunteer settings (`internal/config`).
2. Connects to all coordinators in `SC_COORDINATORS` (`internal/leaderclient`).
3. Runs the loop in `internal/volunteer` until Ctrl+C or `SIGTERM`.

## Settings

`SC_COORDINATORS`, `SC_VOLUNTEER_TOKEN`, `SC_VOLUNTEER_NAME`, `SC_CPU_CORES`, `SC_MEMORY_MB`, `SC_SPEED_SCORE`. See `.env.example`.
