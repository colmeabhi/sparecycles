# security

Checks volunteer tokens and signs tasks.

**Owner:** Kushagra · **State:** complete · **Issue:** #11

## How it fits

- `Register` in the coordinator calls `Authenticate(token)`. A failure returns gRPC `Unauthenticated`, counts `sc_registrations_rejected_total`, and the volunteer stops.
- `RequestWork` in the coordinator signs `TaskMessage(unitID, workload, leaseExpiry, payload)` and sends the signature with the unit.
- The volunteer checks the signature before it runs a unit. On a bad signature it runs nothing and stops, because every later task would fail too. Gossip then marks it dead, and its unit moves on.

## Interfaces

```go
type Authenticator interface { Authenticate(token string) (label string, err error) }
type Signer interface { Sign(msg []byte) []byte }              // coordinator
type Verifier interface { Verify(msg, signature []byte) error } // volunteer

func TaskMessage(unitID, workload string, leaseExpiryUnixMs int64, payload []byte) []byte
```

`TaskMessage` binds the payload to its unit, workload and lease expiry, with a length prefix on each field. A signed task can not be replayed for another unit or after its lease.

## Implementations

| Type | Role | Behavior |
| --- | --- | --- |
| `TokenList` | Authenticator | Per-volunteer tokens, `name=token` or `token`. Constant-time compare of every entry. |
| `SharedToken` | Authenticator | One token for all. An empty token accepts everyone (label `open`). |
| `Ed25519Signer` | Signer | Signs with the private key seed |
| `Ed25519Verifier` | Verifier | Checks with the public key |
| `NoSign` | Signer and Verifier | No signature, accepts everything. Used when no key is set. |

## Settings

| Variable | Used by | Meaning |
| --- | --- | --- |
| `SC_VOLUNTEER_TOKENS` | coordinator | Comma-separated `name=token` list. Wins over `SC_VOLUNTEER_TOKEN`. |
| `SC_VOLUNTEER_TOKEN` | both | Shared token (coordinator), or this volunteer's token |
| `SC_SIGNING_KEY` | coordinator | Base64 Ed25519 seed. Every coordinator needs the same key. |
| `SC_SIGNING_PUBLIC_KEY` | volunteer | Base64 Ed25519 public key |

With no tokens and no keys, everything stays open, so the repo runs after a clone. Each binary logs which mode is active at start.

## Make keys

```bash
make keys
```

It prints the 2 `.env` lines. Put `SC_SIGNING_KEY` on the coordinators only, and `SC_SIGNING_PUBLIC_KEY` on the volunteers.

## Demo

Coordinator with `SC_VOLUNTEER_TOKENS=alice=...,bob=...` and `SC_SIGNING_KEY`:

```text
volunteer side:     msg="volunteer stopped" err="rpc error: code = Unauthenticated desc = invalid volunteer token"
coordinator side:   msg="registration rejected" name=intruder reason="invalid volunteer token"

wrong public key:   msg="bad signature: check SC_SIGNING_PUBLIC_KEY" unit=job-9243...-0
coordinator side:   member dead → lease released: volunteer dead → job finished state=done
```

## Rules

- Never commit keys or tokens. Use `.env` locally and GitHub secrets in CI.
- Use the Go standard library for crypto. Do not write your own.
- Compare secrets in constant time.
