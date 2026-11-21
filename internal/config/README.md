# config

Reads all settings from environment variables. Each binary first loads `.env` from the current directory, if it exists. Variables that are already set in the shell win.

**Owner:** all · **State:** complete

## Functions

| Function | Used by |
| --- | --- |
| `LoadDotEnv()` | Every `main` |
| `LoadCoordinator()` | `cmd/coordinator` |
| `LoadVolunteer()` | `cmd/volunteer` |
| `LoadClient()` | `cmd/client` |

## Variables

`.env.example` in the repo root lists every variable with a comment and its default.

## Add a variable

1. Add a field to the right struct and read it in the right `Load...` function with `stringVar`, `intVar`, `floatVar`, `durationVar` or `listVar`.
2. Give it a default that works on one machine. The repo must run after a clone with no `.env`.
3. Return a clear error for a bad value, with the variable name.
4. Add it to `.env.example` with a comment.
5. Add a test in `config_test.go` for the default and for a bad value.
6. If CI needs it, add it as a GitHub secret or variable with the same name.
