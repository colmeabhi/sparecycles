# security

Checks volunteer tokens and signs tasks.

**Owner:** Kushagra · **State:** shared token, no signatures · **Issue:** #11

## How it fits

- `Register` in the coordinator calls `Authenticate(token)`. A failure returns gRPC `Unauthenticated`, and the volunteer stops.
- `RequestWork` in the coordinator calls `Sign(payload)` and sends the signature with the unit.
- The volunteer calls `Verify(payload, signature)` before it runs a unit, and skips the unit on failure.

## Interfaces

```go
type Authenticator interface { Authenticate(token string) error }
type Signer interface {
    Sign(payload []byte) []byte
    Verify(payload, signature []byte) error
}
```

## Implementations

| Type | Behavior |
| --- | --- |
| `SharedToken` | One token for all volunteers. An empty token accepts everyone. |
| `NoSign` | No signature. Every payload passes. Base stub. |

## Settings

| Variable | Default | Meaning |
| --- | --- | --- |
| `SC_VOLUNTEER_TOKEN` | empty | Shared volunteer token |

## Rules

- Design decision: each volunteer has a token, and the leader signs each task.
- With no keys or tokens set, the repo must still run after a clone.
- Compare secrets in constant time. Use the standard library for crypto.
- Never commit keys or tokens.
