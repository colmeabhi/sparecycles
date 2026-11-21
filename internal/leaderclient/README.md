# leaderclient

Sends RPCs to the current leader. The client and the volunteers use it.

**Owner:** all · **State:** complete

## How it works

- A `Pool` holds one gRPC connection to each coordinator.
- `Do(ctx, fn)` calls `fn` with the connection that worked last.
- If the answer is `FailedPrecondition` (not the leader) or `Unavailable` (down), it tries the next coordinator.
- It tries each coordinator once per call. The caller decides when to try again.

This also handles a leader failover: the old leader is down, the next coordinator answers, and the pool keeps using it.

## Use

```go
pool, err := leaderclient.Dial([]string{"host1:7400", "host2:7400"})
err = pool.Do(ctx, func(conn *grpc.ClientConn) error {
    _, err := pb.NewClientServiceClient(conn).GetJob(ctx, req)
    return err
})
```

Tests use `FromConns` with `bufconn` connections.

## Rules

- Connections use no TLS today. Add TLS here if the team adds it.
