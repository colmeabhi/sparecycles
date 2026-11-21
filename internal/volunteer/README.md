# volunteer

The volunteer loop: register, ask for work, run it, send the result, and repeat.

**Owner:** all · **State:** complete

## How it fits

`cmd/volunteer` builds a `Volunteer` and calls `Run`. It talks to the coordinators through `internal/leaderclient`, so it follows the leader after a failover.

## Loop

1. Register with the token, resources and gossip address. Keep the volunteer ID, and call `Config.OnRegister` with it.
2. Ask for work (`RequestWork`). If there is no unit, wait for `retry_after_ms`.
3. Check the signature of `security.TaskMessage(unit, workload, lease expiry, payload)`. Run the workload with a deadline at the lease expiry.
4. Send the result (`SubmitResult`).

## Error handling

| Error | Action |
| --- | --- |
| `Unauthenticated` on register | Stop. A wrong token does not fix itself. |
| Bad task signature | Stop and run nothing. A wrong key or a forged task does not fix itself, and the volunteer must not keep taking units. |
| `NotFound` (leader does not know the ID) | Register again. This happens after a failover, or after gossip wrongly marked it dead. |
| Any other error | Wait with exponential backoff (250 ms up to 10 s), then try again. |
| Unit fails or lease ends | Send nothing. The lease expires and another volunteer gets the unit. |

## Settings

Read in `internal/config` (volunteer section): `SC_COORDINATORS`, `SC_VOLUNTEER_TOKEN`, `SC_VOLUNTEER_NAME`, `SC_CPU_CORES`, `SC_MEMORY_MB`, `SC_SPEED_SCORE`, `SC_SIGNING_PUBLIC_KEY`, and the `SC_GOSSIP_*` settings.
