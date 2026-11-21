# Setup and run

These steps work on a fresh clone. No `.env` file is necessary to start.

## 1. Install the tools

| Tool | Version | Needed for |
| --- | --- | --- |
| Go | 1.27 or later | Build, test, run |
| Docker Desktop | Any recent | Local cluster with Docker Compose (optional) |
| protoc and plugins | protoc 30 or later | Only if you change the `.proto` file |

On macOS:

```bash
brew install go
```

Only if you change `proto/sparecycles/v1/sparecycles.proto`:

```bash
brew install protobuf
```

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

```bash
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

The generated code in `gen/` is committed. You do not need protoc to build or run.

## 2. Clone and test

```bash
git clone https://github.com/colmeabhi/sparecycles.git
```

```bash
make -C sparecycles test
```

## 3. Settings (optional)

All settings are environment variables that start with `SC_`. The defaults work on one machine.

To change a setting, copy the example file and edit it:

```bash
cp .env.example .env
```

- Each binary reads `.env` from the current directory. Variables that are already set in the shell win.
- Docker Compose also reads `.env`.
- Do not commit `.env`. It is in `.gitignore`.
- For CI, add the same names as GitHub secrets or variables.

`.env.example` explains each variable.

## 4. Run on one machine

Open 3 terminals in the repo folder.

Terminal 1, the coordinator:

```bash
make run-coordinator
```

Terminal 2, a volunteer. Run this in more terminals to add more volunteers:

```bash
make run-volunteer
```

Terminal 3, the client. It submits a 1024×1024 fractal job and saves `result.png`:

```bash
make run-client
```

## 5. Run with Docker Compose

This starts 1 coordinator, 3 volunteers and Prometheus. Start Docker Desktop first.

```bash
make up
```

```bash
make run-client
```

```bash
make down
```

## Ports

| Port | Use |
| --- | --- |
| 7400 | Coordinator gRPC |
| 9400 | Coordinator `/metrics` and `/healthz` |
| 9090 | Prometheus web page (Docker Compose only) |

Port 7000 is not used, because macOS uses it for AirPlay.
