SHELL := /bin/bash
.DEFAULT_GOAL := help

GO        ?= go
COMPOSE   ?= docker compose
SERVICES  := counter persistency-controller writer reader api-gateway
PROTO_DIR := libs/proto
GEN_DIR   := libs/gen

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: tools
tools: ## Install build tooling (protoc plugins, golangci-lint)
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

.PHONY: proto
proto: ## Generate Go stubs from .proto files into libs/gen
	@mkdir -p $(GEN_DIR)
	@PATH="$$(go env GOPATH)/bin:$$PATH" bash -c '\
		protoc -I $(PROTO_DIR) \
			--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
			--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
			$$(find $(PROTO_DIR) -name "*.proto")'
	@echo "proto generation complete"

.PHONY: proto-check
proto-check: proto ## Fail if generated stubs drift from committed ones
	@git diff --exit-code -- $(GEN_DIR) || \
		(echo "generated proto code is out of date; run 'make proto'" && exit 1)

.PHONY: tidy
tidy: ## Sync go.mod/go.sum
	$(GO) mod tidy

.PHONY: build
build: ## Compile all Go packages
	$(GO) build ./...

.PHONY: lint
lint: ## Run golangci-lint
	@PATH="$$(go env GOPATH)/bin:$$PATH" golangci-lint run

.PHONY: test
test: ## Run unit tests
	$(GO) test ./... -count=1

.PHONY: up
up: ## Build images and start the full stack
	$(COMPOSE) up -d --build

.PHONY: down
down: ## Stop the stack and remove volumes
	$(COMPOSE) down -v

.PHONY: ps
ps: ## Show stack status
	$(COMPOSE) ps

.PHONY: logs
logs: ## Tail logs from all services
	$(COMPOSE) logs -f

.PHONY: e2e
e2e: ## Run the end-to-end smoke script against the running stack
	@bash scripts/e2e.sh

.PHONY: images
images: ## Build all service Docker images without starting them
	$(COMPOSE) build
