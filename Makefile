# Copyright 2025 KrakLabs
# SPDX-License-Identifier: AGPL-3.0-or-later

# CIE - Code Intelligence Engine
# Makefile for build, test, and development automation

# Build info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go settings
GOOS    ?= $(shell go env GOOS)
GOARCH  ?= $(shell go env GOARCH)
COZO_VERSION ?= 0.7.6

# Docker settings
DOCKER_REGISTRY ?= ghcr.io
DOCKER_IMAGE    ?= kraklabs/cie
DOCKER_TAG      ?= $(VERSION)

# Build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) -s -w"
CGO_LDFLAGS := -L$(shell pwd)/lib -lcozo_c -lstdc++ -lm
ifeq ($(GOOS),darwin)
	CGO_LDFLAGS += -framework Security
endif
ifeq ($(GOOS),windows)
	CGO_LDFLAGS += -lbcrypt -lwsock32 -lws2_32 -lshlwapi -lrpcrt4
endif

# Install directory (defaults to ~/go/bin, override with INSTALL_DIR=path)
INSTALL_DIR ?= $(HOME)/go/bin

.PHONY: all build test test-short test-coverage lint fmt fmt-check clean docker-build docker-push tools run help deps install

# Default target
all: lint test build

build: deps ## Build the cie binary
	@echo "Building cie $(VERSION) for $(GOOS)/$(GOARCH)..."
	@mkdir -p bin
	CGO_ENABLED=1 CGO_LDFLAGS="$(CGO_LDFLAGS)" go build $(LDFLAGS) -o bin/cie ./cmd/cie
	@echo "✓ Built bin/cie"

install: build ## Install cie binary globally (default: ~/go/bin, override: INSTALL_DIR=path)
	@mkdir -p $(INSTALL_DIR)
	@cp bin/cie $(INSTALL_DIR)/cie
	@echo "✓ Installed cie to $(INSTALL_DIR)/cie"

deps: ## Download dependencies (CozoDB)
	@mkdir -p lib
	@if [ ! -f lib/libcozo_c.a ]; then \
		echo "Downloading CozoDB $(COZO_VERSION)..."; \
		ARCH=$(GOARCH); \
		OS=$(GOOS); \
		if [ "$$OS" = "darwin" ]; then \
			if [ "$$ARCH" = "arm64" ]; then PLATFORM="aarch64-apple-darwin"; else PLATFORM="x86_64-apple-darwin"; fi; \
		elif [ "$$OS" = "linux" ]; then \
			if [ "$$ARCH" = "arm64" ]; then PLATFORM="aarch64-unknown-linux-gnu"; else PLATFORM="x86_64-unknown-linux-gnu"; fi; \
		else \
			echo "Unsupported OS: $$OS"; exit 1; \
		fi; \
		URL="https://github.com/cozodb/cozo/releases/download/v$(COZO_VERSION)/libcozo_c-$(COZO_VERSION)-$${PLATFORM}.a.gz"; \
		curl -L $$URL -o lib/libcozo_c.a.gz; \
		gunzip -f lib/libcozo_c.a.gz; \
		if [ -f lib/libcozo_c-$(COZO_VERSION)-$${PLATFORM}.a ]; then \
			mv lib/libcozo_c-$(COZO_VERSION)-$${PLATFORM}.a lib/libcozo_c.a; \
		fi \
	fi

test: deps ## Run all tests with race detection and coverage
	@echo "Running tests..."
	CGO_ENABLED=1 CGO_LDFLAGS="$(CGO_LDFLAGS)" go test -race -cover -coverprofile=coverage.out ./...
	@echo "✓ Coverage report: coverage.out"

test-short: ## Run tests without integration tests (no CozoDB required)
	@echo "Running short tests..."
	CGO_ENABLED=1 CGO_LDFLAGS="$(CGO_LDFLAGS)" go test -short -race ./...

test-coverage: test ## View coverage in browser
	go tool cover -html=coverage.out

lint: ## Run golangci-lint
	@echo "Running linter..."
	golangci-lint run ./...

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	@command -v goimports >/dev/null 2>&1 && goimports -w . || echo "⚠ goimports not installed (run: make tools)"

fmt-check: ## Check formatting (for CI)
	@echo "Checking formatting..."
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)

docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build -t $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):latest
	@echo "✓ Built $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)"

docker-push: ## Push Docker image to registry
	@echo "Pushing Docker image..."
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):latest
	@echo "✓ Pushed $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/ coverage.out
	@echo "✓ Cleaned"

tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "✓ Tools installed"

run: ## Run the application
	go run ./cmd/cie $(ARGS)

help: ## Show help
	@echo "CIE - Code Intelligence Engine"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
