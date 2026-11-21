MODULE := github.com/colmeabhi/sparecycles
GOBIN  := $(shell go env GOPATH)/bin

.PHONY: help proto build test vet keys run-coordinator run-volunteer run-client up down logs clean

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

clean:
	rm -rf bin result.png
