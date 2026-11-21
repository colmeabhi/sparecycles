# keygen (binary)

Prints a new Ed25519 key pair for task signing, as 2 `.env` lines.

**Owner:** Kushagra · **State:** complete

## Run

```bash
make keys
```

## Output

```text
# Coordinators only. Keep this secret.
SC_SIGNING_KEY=<base64 seed>
# Volunteers.
SC_SIGNING_PUBLIC_KEY=<base64 public key>
```

- Put `SC_SIGNING_KEY` in the `.env` of every coordinator. All coordinators need the same key, because any of them can become the leader.
- Put `SC_SIGNING_PUBLIC_KEY` in the `.env` of every volunteer.
- Never commit the private key. For CI or AWS, store it as a secret.
