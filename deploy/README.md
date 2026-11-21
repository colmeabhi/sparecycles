# deploy

Files to run SpareCycles outside a single `go run`.

**Owner:** Abhishek (Terraform) · **State:** Prometheus only · **Issue:** #15 (Terraform)

## Files

| Path | Use |
| --- | --- |
| `prometheus.yml` | Prometheus scrape config for Docker Compose (`docker-compose.yml` in the repo root) |
| `terraform/` | AWS EC2 deployment (to come, issue #15) |

## Rules

- Never commit secrets, Terraform state or real `.tfvars` files. `.gitignore` blocks them.
- When you add a coordinator, add it to `prometheus.yml`.
- Document every deploy command in `docs/setup.md`.
