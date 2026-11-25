# deploy

Files to run SpareCycles outside a single `go run`.

**Owner:** Abhishek (Terraform) · **State:** Prometheus config and Terraform (validated; first apply to do) · **Issue:** #15 (Terraform)

## Files

| Path | Use |
| --- | --- |
| `prometheus.yml` | Prometheus scrape config for Docker Compose (`docker-compose.yml` in the repo root). It scrapes `coordinator-1` to `coordinator-3` on port 9400. |
| `terraform/` | AWS EC2 deployment: 3 coordinators in 3 zones, spot volunteers, Prometheus. `make deploy-up` and `make deploy-down`. See [terraform/README.md](terraform/README.md). |

## Rules

- Never commit secrets, Terraform state or real `.tfvars` files. `.gitignore` blocks them.
- When you add a coordinator, add it to `prometheus.yml`.
- Document every deploy command in `docs/setup.md`.
