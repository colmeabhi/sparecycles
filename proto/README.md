# proto

The gRPC contract between all nodes. `gen/` holds the Go code made from it.

**Owner:** all (discuss changes with the team) · **State:** complete

## Services

| Service | Caller → server | RPCs |
| --- | --- | --- |
| `ClientService` | client → leader | `SubmitJob`, `GetJob` |
| `VolunteerService` | volunteer → leader | `Register`, `RequestWork`, `SubmitResult` |
| `ElectionService` | coordinator → coordinator | `Election`, `Coordinator`, `Heartbeat` |
| `ReplicationService` | leader → backup | `Replicate`, `Snapshot` |
| `GossipService` | any node → any node | `Ping`, `PingReq` |

A follower answers client and volunteer RPCs with gRPC `FAILED_PRECONDITION`. The caller then tries the next coordinator.

## Change the contract

1. Discuss the change with the team. Every part depends on this file.
2. Only add fields with new numbers. Never reuse or renumber a field.
3. Run `make proto` (needs `protoc` and the 2 plugins, see `docs/setup.md`).
4. Commit the `.proto` file and the new files in `gen/` together.
5. Update this README if you add a service or an RPC.
