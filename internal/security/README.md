# security

Checks volunteer tokens and session secrets, and signs tasks.

**Owner:** Kushagra · **State:** complete · **Issues:** #11, #35 (session secrets)

## How it fits

- `Register` in the coordinator calls `Authenticate(token)`. A failure returns gRPC `Unauthenticated`, counts `sc_registrations_rejected_total`, and the volunteer stops.
- On success, `Register` calls `NewSession()`. It sends the secret to the volunteer once and keeps only its hash.
- `RequestWork` and `SubmitResult` call `SessionHash.Check(secret)` before anything else. A missing or wrong secret returns `Unauthenticated`, counts `sc_requests_rejected_total`, and changes nothing. Gossip shares every volunteer ID, so the ID alone does not prove who sends a request.
- `RequestWork` in the coordinator signs `TaskMessage(unitID, workload, leaseExpiry, payload)` and sends the signature with the unit.
- The volunteer checks the signature before it runs a unit. On a bad signature it runs nothing and stops, because every later task would fail too. Gossip then marks it dead, and its unit moves on.

## Interfaces

```go
type Authenticator interface { Authenticate(token string) (label string, err error) }
type Signer interface { Sign(msg []byte) []byte }              // coordinator
type Verifier interface { Verify(msg, signature []byte) error } // volunteer

func TaskMessage(unitID, workload string, leaseExpiryUnixMs int64, payload []byte) []byte

func NewSession() (secret []byte, hash SessionHash) // 32 random bytes and their SHA-256
func (h SessionHash) Check(secret []byte) error      // nil, or ErrBadSession
```

`TaskMessage` binds the payload to its unit, workload and lease expiry, with a length prefix on each field. A signed task can not be replayed for another unit or after its lease.

Signed tasks prove the leader to the volunteer. Session secrets prove the volunteer to the leader. Both are needed: without the secret, a volunteer that knows another volunteer's ID (from gossip) could take its work, send results in its name, or lower its trust score.

## Implementations

| Type | Role | Behavior |
| --- | --- | --- |
| `TokenList` | Authenticator | Per-volunteer tokens, `name=token` or `token`. Constant-time compare of every entry. |
| `SharedToken` | Authenticator | One token for all. An empty token accepts everyone (label `open`). |
| `Ed25519Signer` | Signer | Signs with the private key seed |
| `Ed25519Verifier` | Verifier | Checks with the public key |
| `NoSign` | Signer and Verifier | No signature, accepts everything. Used when no key is set. |
| `SessionHash` | Session check | SHA-256 of a session secret. `Check` compares in constant time. A zero hash rejects every secret. |

Session secrets are always on. They need no setting and no key, so the repo still runs after a clone. They are leader-local: after a failover, volunteers get `NotFound` from the new leader, register again, and get a new secret.

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

another volunteer's ID, no secret:
coordinator side:   msg="request rejected: bad session" volunteer=vol-...
```

The session case has no script, because a real volunteer always sends its own secret. `TestRequestsUnderAnotherVolunteersIDAreRejected` in `internal/coordinator/session_test.go` plays the attacker: it sends requests under the victim's ID, and checks that all are rejected and the victim's result still finishes the job.

## Limits

- Without TLS, an attacker on the network can read a secret from the traffic. TLS is out of scope, as in the proposal.
- A secret lasts as long as the registration. A volunteer that registers again gets a new one, and the old one stops working.

## Rules

- Never commit keys or tokens. Use `.env` locally and GitHub secrets in CI.
- Use the Go standard library for crypto. Do not write your own.
- Compare secrets in constant time.
- Never log a session secret. The coordinator keeps only its hash.

## How to test

```bash
go test -race ./internal/security/ ./internal/coordinator/ ./internal/volunteer/
```
