# terraform

Runs the full system on AWS EC2 with one command, and removes it with one command.

**Owner:** Abhishek · **State:** written and validated (`terraform validate`); first real apply still to do · **Issue:** #15

## What it creates

```
                 VPC 10.40.0.0/16, one public subnet per zone
   ┌────────────────────┬────────────────────┬────────────────────┐
   │ zone a             │ zone b             │ zone c             │
   │ coordinator-1      │ coordinator-2      │ coordinator-3      │
   │ 10.40.1.10         │ 10.40.2.10         │ 10.40.3.10         │
   │ prometheus         │                    │                    │
   │ volunteer 1, 4 …   │ volunteer 2, 5 …   │ volunteer 3, 6 …   │
   └────────────────────┴────────────────────┴────────────────────┘
   S3 bucket with the Linux binaries ──► each instance downloads its binary at boot
   laptops (allowed_cidrs) ──► :7400 gRPC, :9400 metrics, :9090 Prometheus, :22 SSH
```

| Resource | Details |
| --- | --- |
| 3 coordinators | One per availability zone, fixed private IPs, so `SC_PEERS` is known before boot. `SC_NODE_ID` 1 to 3. |
| Volunteers | `volunteer_count` instances, spread over the zones. Spot by default (`volunteer_spot`). |
| Prometheus | One small instance that scrapes all 3 coordinators |
| S3 bucket | The binaries from `make deploy-build`. Private; instances read it with an instance role. |
| Security groups | 7400 and 9400 inside the VPC; 7400, 9400, 9090 and 22 from `allowed_cidrs`; gossip (7500) between volunteers and coordinators |

Each binary runs as a `systemd` service that restarts on failure. Settings go to `/etc/sparecycles.env` as `SC_` variables. A new binary (a new S3 etag) replaces the instances that run it.

## Before you start

1. Install Terraform 1.6 or later. On macOS: `brew install hashicorp/tap/terraform`.
2. Log in to AWS (`aws configure`, or the AWS Academy credentials in `~/.aws/credentials`).
3. Copy the example settings and fill them in. Git ignores `terraform.tfvars`.

   ```bash
   cp deploy/terraform/terraform.tfvars.example deploy/terraform/terraform.tfvars
   ```

## Variables

| Variable | Default | Meaning |
| --- | --- | --- |
| `allowed_cidrs` | (required) | Laptop IPs with `/32`. Find yours: `curl -s https://checkip.amazonaws.com` |
| `volunteer_tokens` | (required) | `SC_VOLUNTEER_TOKENS` as `name=token`. EC2 volunteers use the first one. |
| `signing_key`, `signing_public_key` | empty | From `make keys`. Empty means tasks are not signed. |
| `region` | `us-east-1` | AWS Academy allows only `us-east-1` |
| `ssh_key_name` | empty | An existing key pair (AWS Academy: `vockey`) |
| `instance_profile_name` | empty | Empty: Terraform makes a role that reads the bucket. AWS Academy: `LabInstanceProfile` |
| `coordinator_instance_type` | `t3.small` | |
| `volunteer_count` | `3` | |
| `volunteer_instance_type` | `t3.small` | |
| `volunteer_spot` | `true` | `false` for on-demand volunteers |
| `monitor_instance_type` | `t3.micro` | Prometheus |
| `binaries_dir` | `../../dist` | Output of `make deploy-build` |
| `name` | `sparecycles` | Prefix of every name, and the `Project` tag |

## Outputs

| Output | Use |
| --- | --- |
| `sc_coordinators` | The `SC_COORDINATORS=...` line for a laptop `.env` |
| `coordinator_public_ips` | Public IP per node ID |
| `prometheus_url` | Prometheus web page |
| `volunteer_public_ips` | Public IPs of the EC2 volunteers |
| `ssh_commands` | SSH per coordinator; logs with `journalctl -u sparecycles -f` |
| `kill_leader_command` | Stops coordinator 3 (the first leader) for the failover test |

## Apply, test, destroy

```bash
make deploy-up
```

It builds the Linux binaries into `dist/`, runs `terraform init` and `terraform apply`. Copy the `sc_coordinators` line into your laptop `.env`, then:

```bash
make run-client
```

Failure test: run the `kill_leader_command` output during a job, then check the logs of coordinator 2 (`became leader`, `leader continues from table index`). Remove everything:

```bash
make deploy-down
```

## Cost

On-demand prices in `us-east-1`, November 2026. Spot prices change.

| Part | Each | Total per hour |
| --- | --- | --- |
| 3 coordinators, `t3.small` | $0.021 | $0.062 |
| 3 volunteers, `t3.small` spot | about $0.007 | about $0.021 |
| Prometheus, `t3.micro` | $0.010 | $0.010 |
| 7 public IPv4 addresses | $0.005 | $0.035 |
| **Total** | | **about $0.13 per hour** (about $3 per day) |

Run `make deploy-down` after each test session.

## Rules

- Never commit `terraform.tfvars`, state files or `.terraform/`. `.gitignore` blocks them. `.terraform.lock.hcl` is committed, so everyone uses the same provider version.
- Secrets reach the instances in `user_data`. Anyone who can call `ec2:DescribeInstanceAttribute` in the account can read them. For the course account this is fine; move them to SSM Parameter Store for a shared account.
- Laptop volunteers behind a home router should set `SC_GOSSIP_ADDR=off` (see `.env.example`).

## Test

```bash
make deploy-check
```

It runs `terraform fmt -check` and `terraform validate`. CI runs the same in the `terraform` job. Neither needs AWS credentials.
