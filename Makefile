MODULE := github.com/colmeabhi/sparecycles
GOBIN  := $(shell go env GOPATH)/bin

.PHONY: help proto build test vet keys run-coordinator run-volunteer run-client up down logs clean \
	load load-sweep fail-volunteer fail-faulty fail-leader evidence report

help:
	@echo "make build            Build all binaries into ./bin"
	@echo "make test             Run all tests with the race detector"
	@echo "make vet              Run go vet"
	@echo "make proto            Regenerate gRPC code (needs protoc and plugins)"
	@echo "make keys             Print a new task signing key pair for .env"
	@echo "make run-coordinator  Run one coordinator on this machine"
	@echo "make run-volunteer    Run one volunteer on this machine"
	@echo "make run-client       Submit a fractal job and save result.png"
	@echo "make up               Start the local cluster with Docker Compose"
	@echo "make down             Stop the local cluster"
	@echo "make load             One load test (2 volunteers, 8 jobs, full mode)"
	@echo "make load-sweep       Load tests for 1, 2, 4 volunteers, baseline and full"
	@echo "make fail-volunteer   Failure demo: kill a volunteer during a job"
	@echo "make fail-faulty      Failure demo: a volunteer sends wrong results"
	@echo "make fail-leader      Failure demo: kill the leader during a job"
	@echo "make evidence         All 3 failure demos"
	@echo "make report           Render diagrams and build docs/report/build/main.pdf"

proto:
	protoc \
		--plugin=protoc-gen-go=$(GOBIN)/protoc-gen-go \
		--plugin=protoc-gen-go-grpc=$(GOBIN)/protoc-gen-go-grpc \
		--go_out=. --go_opt=module=$(MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(MODULE) \
		proto/sparecycles/v1/sparecycles.proto

build:
	go build -o bin/ ./cmd/...

test:
	go test -race ./...

vet:
	go vet ./...

keys:
	@go run ./cmd/keygen

run-coordinator:
	go run ./cmd/coordinator

run-volunteer:
	go run ./cmd/volunteer

run-client:
	go run ./cmd/client -out result.png

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

load:
	bash scripts/load.sh

load-sweep:
	bash scripts/load-sweep.sh

fail-volunteer:
	bash scripts/fail-volunteer.sh

fail-faulty:
	bash scripts/fail-faulty.sh

fail-leader:
	bash scripts/fail-leader.sh

evidence: fail-volunteer fail-faulty fail-leader

report:
	plantuml -tpng docs/report/diagrams/*.puml
	mkdir -p docs/report/build
	tectonic --outdir docs/report/build docs/report/main.tex

clean:
	rm -rf bin result.png runs
