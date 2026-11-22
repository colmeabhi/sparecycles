# volunteer (binary)

Runs one volunteer. It registers with the leader, pulls units, runs them and sends the results. It also takes part in SWIM gossip.

**Owner:** all · **State:** complete

## Run

```bash
make run-volunteer
```

Run the command in more terminals to add more volunteers. Each one picks a free gossip port.

## What `main` does

1. Loads `.env` and the volunteer settings (`internal/config`).
2. Connects to all coordinators in `SC_COORDINATORS` (`internal/leaderclient`).
3. Unless `SC_GOSSIP_ADDR=off`: starts a gRPC gossip listener and a SWIM member (`internal/membership`), and sends its address in `Register`. After each registration, it gives the new volunteer ID to SWIM.
4. Checks task signatures with `SC_SIGNING_PUBLIC_KEY` if it is set (`internal/security`).
5. Runs the loop in `internal/volunteer` until Ctrl+C or `SIGTERM`, or until a task has a bad signature.

## Settings

| Variable | Default | Meaning |
| --- | --- | --- |
| `SC_COORDINATORS` | `localhost:7400` | Coordinator addresses |
| `SC_VOLUNTEER_TOKEN` | empty | Registration token |
| `SC_SIGNING_PUBLIC_KEY` | empty (no check) | Coordinators' public key, from `make keys` |
| `SC_VOLUNTEER_NAME` | host name | Name in logs |
| `SC_CPU_CORES`, `SC_MEMORY_MB`, `SC_SPEED_SCORE` | detected | Reported resources |
| `SC_GOSSIP_ADDR` | `:0` (free port) | Gossip listen address, or `off` |
| `SC_GOSSIP_HOST` | `127.0.0.1` | Host that others use to reach this volunteer. `hostname` uses the machine name. |
| `SC_GOSSIP_*` timings | see `.env.example` | Same as the coordinator |
| `SC_FAULTY_RATE` | `0` | Demo only: share of units (0 to 1) with a wrong output on purpose. The volunteer logs a warning at start when it is above 0. |

## Behind a home router

Coordinators must be able to connect to the gossip address. A laptop behind a home router usually can not accept those connections. Run it with `SC_GOSSIP_ADDR=off`: it then works normally, and only the lease finds its failure.
