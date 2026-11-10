SHELL := /bin/bash

GO ?= go
IMAGE_REGISTRY ?= mall
IMAGE_TAG ?= dev
K8S_REMOTE ?= root@10.211.55.29
GOPROXY ?= https://proxy.golang.org,direct

COMPOSE_DIR := deploy/compose
COMPOSE_ENV_ARG := $(if $(wildcard $(COMPOSE_DIR)/.env),--env-file $(COMPOSE_DIR)/.env,)
COMPOSE := docker compose $(COMPOSE_ENV_ARG) \
	-f $(COMPOSE_DIR)/infra.yml \
	-f $(COMPOSE_DIR)/services.yml

SERVICES := gateway admin identity catalog inventory cart pricing order payment fulfillment aftersales
INFRA_SERVICES := mysql redis nats etcd dtm
OBS_SERVICES := prometheus grafana
K8S_DIR := deploy/k8s
K8S_MALL_VALUES := $(K8S_DIR)/charts/mall/values-single-node.yaml
K8S_INFRA_VALUES := $(K8S_DIR)/charts/infra/values-single-node.yaml

.DEFAULT_GOAL := help

.PHONY: help build test fmt fmt-check vet run-infra run-observability \
	run-gateway run-admin run-service run-all db-reset dtm-smoke dtm-contract e2e-phase15 concurrency-phase15 test-race verify wire arch clean \
	k8s-images k8s-import k8s-render k8s-lint k8s-deploy mesh-smoke

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
	@echo "  e2e-phase15                       Phase 1.5 B-end/C-end/return E2E (gateway/admin URLs)"
	@echo "  concurrency-phase15               prove two simultaneous checkouts do not oversell"
	@echo "  test-race                         unit tests with the race detector"
	@echo "  arch                              architecture/import guard"
	@echo "  wire                              regenerate Wire injectors for gateway/admin/order/payment"
	@echo "  verify                            fmt-check + vet + test + build + arch + dtm-contract"
	@echo "  k8s-images                        build service and migration images for Kubernetes"
	@echo "  k8s-import                        import images into a remote k3s containerd"
	@echo "  k8s-render                        render Helm charts for the single-node values"
	@echo "  k8s-lint                          helm lint Kubernetes charts"
	@echo "  k8s-deploy                        install/upgrade charts into an existing cluster"
	@echo "  mesh-smoke                        run the Kubernetes/Ambient smoke script"
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
	$(GO) run github.com/google/wire/cmd/wire ./cmd/gateway ./cmd/admin ./cmd/order ./cmd/payment

verify: fmt-check vet test test-race build arch dtm-contract
	@echo "verify: OK"

dtm-smoke:
	./scripts/dtm-smoke.sh

e2e-phase15:
	./scripts/e2e-phase15.sh

concurrency-phase15:
	./scripts/concurrency-phase15.sh

dtm-contract:
	./scripts/check-dtm-contract.sh

test-race:
	$(GO) test -race ./internal/services/... ./internal/platform/...

db-reset:
	@echo ">> removing containers and named volumes for project mall"
	$(COMPOSE) down -v --remove-orphans
	@echo ">> starting mysql; init scripts re-apply databases and migrations"
	$(COMPOSE) up -d --wait mysql

k8s-images:
	@set -e; for svc in $(SERVICES); do \
		echo ">> docker build --build-arg SERVICE=$$svc --build-arg GOPROXY=$(GOPROXY) -t $(IMAGE_REGISTRY)/$$svc:$(IMAGE_TAG) ."; \
		docker build --build-arg SERVICE=$$svc --build-arg GOPROXY=$(GOPROXY) -t $(IMAGE_REGISTRY)/$$svc:$(IMAGE_TAG) .; \
	done
	docker build -f $(K8S_DIR)/images/migrate/Dockerfile -t $(IMAGE_REGISTRY)/migrate:$(IMAGE_TAG) .

k8s-import:
	@set -e; for image in $(addprefix $(IMAGE_REGISTRY)/,$(SERVICES)) $(IMAGE_REGISTRY)/migrate; do \
		echo ">> import $$image:$(IMAGE_TAG)"; \
		docker save "$$image:$(IMAGE_TAG)" | ssh $(K8S_REMOTE) "k3s ctr -n k8s.io images import -"; \
	done

k8s-render:
	helm template mall $(K8S_DIR)/charts/mall -n mall -f $(K8S_MALL_VALUES) > /tmp/mall-rendered.yaml
	helm template mall-infra $(K8S_DIR)/charts/infra -n mall-data -f $(K8S_INFRA_VALUES) > /tmp/mall-infra-rendered.yaml
	@echo "rendered /tmp/mall-rendered.yaml and /tmp/mall-infra-rendered.yaml"

k8s-lint:
	helm lint $(K8S_DIR)/charts/mall -f $(K8S_MALL_VALUES)
	helm lint $(K8S_DIR)/charts/infra -f $(K8S_INFRA_VALUES)

k8s-deploy:
	kubectl apply -f $(K8S_DIR)/manifests/namespaces.yaml
	helm upgrade --install mall-infra $(K8S_DIR)/charts/infra -n mall-data -f $(K8S_INFRA_VALUES)
	helm upgrade --install mall $(K8S_DIR)/charts/mall -n mall -f $(K8S_MALL_VALUES)

mesh-smoke:
	./scripts/k8s-smoke.sh

clean:
	rm -rf bin
