# volunteer

The volunteer loop: register, ask for work, run it, send the result, and repeat.

**Owner:** all · **State:** complete

## How it fits

`cmd/volunteer` builds a `Volunteer` and calls `Run`. It talks to the coordinators through `internal/leaderclient`, so it follows the leader after a failover.

## Loop

1. Register with the token and resources. Keep the volunteer ID.
2. Ask for work (`RequestWork`). If there is no unit, wait for `retry_after_ms`.
3. Check the signature. Run the workload with a deadline at the lease expiry.
4. Send the result (`SubmitResult`).

## Error handling

| Error | Action |
| --- | --- |
| `Unauthenticated` on register | Stop. A wrong token does not fix itself. |
| `NotFound` (leader does not know the ID) | Register again. This happens after a failover. |
| Any other error | Wait with exponential backoff (250 ms up to 10 s), then try again. |
| Unit fails or lease ends | Send nothing. The lease expires and another volunteer gets the unit. |

## Settings

Read in `internal/config` (volunteer section): `SC_COORDINATORS`, `SC_VOLUNTEER_TOKEN`, `SC_VOLUNTEER_NAME`, `SC_CPU_CORES`, `SC_MEMORY_MB`, `SC_SPEED_SCORE`.
