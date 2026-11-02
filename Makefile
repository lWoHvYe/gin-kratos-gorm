SHELL := /bin/bash

GO ?= go

COMPOSE_DIR := deploy/compose
COMPOSE_ENV_ARG := $(if $(wildcard $(COMPOSE_DIR)/.env),--env-file $(COMPOSE_DIR)/.env,)
COMPOSE := docker compose $(COMPOSE_ENV_ARG) \
	-f $(COMPOSE_DIR)/infra.yml \
	-f $(COMPOSE_DIR)/services.yml

SERVICES := gateway admin identity catalog inventory cart pricing order payment fulfillment aftersales
INFRA_SERVICES := mysql redis nats etcd dtm
OBS_SERVICES := prometheus grafana

.DEFAULT_GOAL := help

.PHONY: help build test fmt fmt-check vet run-infra run-observability \
	run-gateway run-admin run-service run-all db-reset dtm-smoke dtm-contract test-race verify wire arch clean

help:
	@echo "mall - Go microservice mall (Kratos + Gin + database/sql + DTM)"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build                             build every service into ./bin"
	@echo "  test                              run unit tests (go test ./...)"
	@echo "  fmt                               gofmt all Go sources"
	@echo "  fmt-check                         fail when gofmt would change files"
	@echo "  vet                               run go vet ./..."
	@echo "  run-infra                         start mysql, redis, nats, etcd, dtm"
	@echo "  run-observability                 start prometheus + grafana (profile)"
	@echo "  run-gateway                       build and start gateway + dependencies"
	@echo "  run-admin                         build and start admin + dependencies"
	@echo "  run-service SERVICE=identity      build and start one service + dependencies"
	@echo "  run-all                           build and start the full application stack"
	@echo "  dtm-smoke                         manual happy-path DTM smoke (stack must be up)"
	@echo "  dtm-contract                      static DTM wiring guard"
	@echo "  test-race                         unit tests with the race detector"
	@echo "  arch                              architecture/import guard"
	@echo "  wire                              regenerate Wire injectors for gateway/admin/order"
	@echo "  verify                            fmt-check + vet + test + build + arch + dtm-contract"
	@echo "  clean                             remove ./bin"
	@echo ""
	@echo "Compose files: $(COMPOSE_DIR)/infra.yml + $(COMPOSE_DIR)/services.yml"
	@echo "Optional: cp $(COMPOSE_DIR)/.env.example $(COMPOSE_DIR)/.env"

build:
	@mkdir -p bin
	@set -e; for svc in $(SERVICES); do \
		echo ">> go build -o bin/$$svc ./cmd/$$svc"; \
		$(GO) build -o bin/$$svc ./cmd/$$svc; \
	done

test:
	$(GO) test ./...

fmt:
	@gofmt -l -w $$(find cmd internal -name '*.go')

fmt-check:
	@out=$$(gofmt -l cmd internal); \
	if [ -n "$$out" ]; then \
		echo "gofmt required for:"; echo "$$out"; exit 1; \
	fi

vet:
	$(GO) vet ./...

run-infra:
	$(COMPOSE) up -d --wait $(INFRA_SERVICES)

run-observability:
	$(COMPOSE) --profile observability up -d --wait $(OBS_SERVICES)

run-gateway:
	$(COMPOSE) up -d --build gateway

run-admin:
	$(COMPOSE) up -d --build admin

run-service:
	@if [ -z "$(SERVICE)" ]; then \
		echo "usage: make run-service SERVICE=<name>"; \
		echo "services: $(SERVICES)"; \
		exit 1; \
	fi
	@case " $(SERVICES) " in \
		*" $(SERVICE) "*) ;; \
		*) echo "unknown SERVICE=$(SERVICE)"; echo "services: $(SERVICES)"; exit 1 ;; \
	esac
	$(COMPOSE) up -d --build $(SERVICE)

run-all:
	$(COMPOSE) up -d --build

arch:
	./scripts/check-architecture.sh

wire:
	$(GO) run github.com/google/wire/cmd/wire ./cmd/gateway ./cmd/admin ./cmd/order

verify: fmt-check vet test build arch dtm-contract
	@echo "verify: OK"

dtm-smoke:
	./scripts/dtm-smoke.sh

dtm-contract:
	./scripts/check-dtm-contract.sh

test-race:
	$(GO) test -race ./internal/services/... ./internal/platform/...

db-reset:
	@echo ">> removing containers and named volumes for project mall"
	$(COMPOSE) down -v --remove-orphans
	@echo ">> starting mysql; init scripts re-apply databases and migrations"
	$(COMPOSE) up -d --wait mysql

clean:
	rm -rf bin
