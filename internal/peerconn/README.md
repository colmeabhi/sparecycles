# peerconn

Keeps one gRPC connection to each other coordinator, with a message limit that fits table snapshots.

**Owner:** Abhishek · **State:** complete · **Issue:** #14

## How it fits

- `cmd/coordinator` makes one `Pool` and passes it to the election and replication transports.
- `cmd/coordinator` starts its gRPC server with `ServerOptions()`.

## API

| Name | Use |
| --- | --- |
| `New()` | An empty pool |
| `Pool.Conn(addr)` | The connection to `addr`, made on first use. It reconnects within about a second after the peer comes back. |
| `Pool.Close()` | Closes all connections |
| `ServerOptions()` | Server options for messages up to `MaxMessageBytes` |
| `MaxMessageBytes` | 256 MB. A snapshot holds every result, and a finished job holds the merged image, so both can pass the 4 MB gRPC default. |

## Rules

- Do not add retry or election logic here. This package only manages connections.
