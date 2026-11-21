<div align="center">

# SpareCycles

**Your spare computers, working as one.**

Fault-tolerant volunteer computing: big jobs split into small units, run on volunteer machines, and every answer checked by majority vote.

[![CI](https://github.com/colmeabhi/sparecycles/actions/workflows/ci.yml/badge.svg)](https://github.com/colmeabhi/sparecycles/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)

**[Product page](https://colmeabhi.github.io/sparecycles/)** · [Setup guide](docs/setup.md) · [Project guide](AGENTS.md)

</div>

## Quick start

```bash
make test              # run all tests
make run-coordinator   # terminal 1
make run-volunteer     # terminal 2
make run-client        # terminal 3: saves result.png
```

You need Go 1.27 or later. No `.env` file is needed to start.

## Documentation

| Document | Content |
| --- | --- |
| [Setup guide](docs/setup.md) | Install, settings, local and Docker runs |
| [Project guide](AGENTS.md) | Rules, design decisions, code layout, definition of done |
| [Status](docs/status.md) | What is left, who does it next, and what blocks it |
| [Proposal](docs/proposal.md) · [Instructions](docs/project-instructions.md) | Course documents |
| [Grading checklist](docs/grading-checklist.md) | Every graded requirement and its status |
| [Test results](docs/test-results.md) · [Observability](docs/observability.md) | Test cases with real results; metrics queries and log lines |

Each folder has a `README.md` that explains what it does.

## Team

[Tejas Chakkarwar](https://github.com/Tejas-Chakkarwar) · [Kushagra Bainsla](https://github.com/Kushagrabainsla) · [Abhishek Ahirrao](https://github.com/colmeabhi)
