# ─────────────────────────────────────────────────────────────────────────────
# Starter Go API — Makefile
# ─────────────────────────────────────────────────────────────────────────────

APP_NAME      := starter-go
BIN_DIR       := bin
BIN           := $(BIN_DIR)/$(APP_NAME)
CMD           := ./cmd/api
PKG           := ./...

# Build metadata
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS       := -s -w -X main.version=$(VERSION)
GOFLAGS       := -trimpath

# Docker
DOCKER_IMAGE  ?= $(APP_NAME):$(VERSION)
DOCKERFILE    := docker/Dockerfile
COMPOSE       ?= docker compose

# Tools
SWAG          := $(shell go env GOPATH)/bin/swag

.DEFAULT_GOAL := help

# ─────────────────────────────────────────────────────────────────────────────
# Help
# ─────────────────────────────────────────────────────────────────────────────
.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Development

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

.PHONY: fmt
fmt: ## gofmt the codebase
	gofmt -s -w .

.PHONY: vet
vet: ## go vet
	go vet $(PKG)

.PHONY: lint
lint: ## golangci-lint (requires the binary on PATH)
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found. Install: https://golangci-lint.run"; exit 1; }
	golangci-lint run

.PHONY: test
test: ## Run unit tests with -race
	go test -race -count=1 $(PKG)

.PHONY: cover
cover: ## Test with coverage report
	go test -race -count=1 -coverprofile=coverage.out $(PKG)
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report -> coverage.html"

##@ Build & run

.PHONY: build
build: ## Build the static binary into bin/
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BIN) $(CMD)
	@echo "Built $(BIN) ($(VERSION))"

.PHONY: run
run: ## Run the API locally (uses envs/.env)
	go run $(CMD)

.PHONY: dev
dev: ## Hot reload via 'air' (auto-installs if missing)
	@command -v air >/dev/null 2>&1 || go install github.com/air-verse/air@latest
	air

.PHONY: clean
clean: ## Remove build artefacts
	rm -rf $(BIN_DIR) coverage.out coverage.html

##@ Swagger

.PHONY: swag-install
swag-install: ## Install the swag CLI
	go install github.com/swaggo/swag/cmd/swag@latest

.PHONY: swag
swag: ## Generate Swagger docs into docs/
	@command -v swag >/dev/null 2>&1 || $(MAKE) swag-install
	swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -f $(DOCKERFILE) --build-arg VERSION=$(VERSION) -t $(DOCKER_IMAGE) .

.PHONY: docker-run
docker-run: ## Run the Docker image with envs/.env mounted
	docker run --rm -p 4000:4000 \
		--env-file envs/.env \
		--name $(APP_NAME) \
		$(DOCKER_IMAGE)

##@ Compose

.PHONY: up
up: ## Start the full stack (postgres + redis + minio + api)
	$(COMPOSE) up -d --build

.PHONY: up-deps
up-deps: ## Start only the dependencies (postgres + redis + minio)
	$(COMPOSE) up -d postgres redis minio minio-init

.PHONY: down
down: ## Stop the stack
	$(COMPOSE) down

.PHONY: down-volumes
down-volumes: ## Stop the stack and remove volumes (WIPES DATA)
	$(COMPOSE) down -v

.PHONY: logs
logs: ## Tail logs for all services
	$(COMPOSE) logs -f --tail=200

.PHONY: ps
ps: ## Show running services
	$(COMPOSE) ps
