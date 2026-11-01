GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always 2>/dev/null || echo dev)
WIRE ?= $(shell command -v wire 2>/dev/null || echo $(GOPATH)/bin/wire)
export PATH := $(GOPATH)/bin:$(PATH)

ifneq (,$(findstring windows,$(GOHOSTOS)))
	INTERNAL_PROTO_FILES=$(shell find internal -name "*.proto")
	API_PROTO_FILES=$(shell find api -name "*.proto")
else
	INTERNAL_PROTO_FILES=$(shell find internal -name "*.proto")
	API_PROTO_FILES=$(shell find api -name "*.proto")
endif

.PHONY: help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-24s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.PHONY: init
# Install protoc plugins, wire and helper tools
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest

.PHONY: api
# Generate protobuf/gRPC/HTTP/OpenAPI code
api:
	protoc --proto_path=./api \
	       --proto_path=./third_party \
	       --go_out=paths=source_relative:./api \
	       --go-http_out=paths=source_relative:./api \
	       --go-grpc_out=paths=source_relative:./api \
	       --openapi_out=fq_schema_naming=true,default_response=false:. \
	       $(API_PROTO_FILES)

.PHONY: config
# Generate internal protobuf config
config:
	protoc --proto_path=./internal \
	       --proto_path=./third_party \
	       --go_out=paths=source_relative:./internal \
	       $(INTERNAL_PROTO_FILES)

.PHONY: wire
# Regenerate Wire dependency injection code
wire:
	cd internal/bootstrap && $(WIRE)
	cd cmd/order && $(WIRE)
	cd cmd/product && $(WIRE)

.PHONY: generate
# Run go generate and tidy modules
generate:
	go generate ./...
	go mod tidy

.PHONY: all
# Generate API, config and Wire code
all: api config wire generate

.PHONY: build
# Build all commands into ./bin
build:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./cmd/server ./cmd/order ./cmd/product

.PHONY: run
# Run the modular monolith with configs/config.yaml
run:
	go run ./cmd/server -conf configs/config.yaml

.PHONY: run-local
# Run the monolith with SQLite + local saga (no MySQL/etcd/DTM)
run-local:
	go run ./cmd/server -conf configs/config.local.yaml

.PHONY: run-order
# Run the split order service (local compensation saga)
run-order:
	go run ./cmd/order -conf configs/order.yaml

.PHONY: run-product
# Run the split product service
run-product:
	go run ./cmd/product -conf configs/product.yaml

.PHONY: test
# Run unit + integration tests (SQLite, no external infra)
test:
	go test ./...

.PHONY: test-race
# Run tests with the race detector
test-race:
	go test -race ./...

.PHONY: vet
# Run go vet
vet:
	go vet ./...

.PHONY: fmt
# Format Go sources
fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './third_party/*')

.PHONY: migrate
# Apply MySQL schema (override MYSQL_* env vars as needed)
migrate:
	mysql -h $${MYSQL_HOST:-127.0.0.1} -P $${MYSQL_PORT:-3306} \
	      -u $${MYSQL_USER:-root} -p$${MYSQL_PASSWORD:-root} \
	      $${MYSQL_DATABASE:-gin_kratos} < migrations/001_init.sql

.PHONY: compose-up
# Start MySQL/Redis/etcd only (DTM is intentionally optional)
compose-up:
	docker compose -f docker/docker-compose.yml up -d

.PHONY: compose-down
# Stop local infrastructure
compose-down:
	docker compose -f docker/docker-compose.yml down

.PHONY: gen_code
# Generate GORM query code from an existing MySQL schema
gen_code:
	go run cmd/gorm_gen.go

.PHONY: clean
# Remove local build/test artifacts
clean:
	rm -rf bin .tmp dev.db order.db product.db

.DEFAULT_GOAL := help
