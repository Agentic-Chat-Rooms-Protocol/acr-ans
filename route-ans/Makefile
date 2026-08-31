# ANS Resolution Server Makefile

.PHONY: all build run test lint clean docker docker-build docker-run docker-tag docker-push docker-release help

# Variables
BINARY_NAME=route-ans
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"
DOCKER_IMAGE=route-ans
DOCKER_TAG?=1.0.0
DOCKER_REGISTRY?=ghcr.io
DOCKER_USERNAME?= route-ans

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Default target
all: lint test build

## Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/resolver

## Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	$(GORUN) ./cmd/resolver --config configs/resolver.yaml

## Run with hot reload (requires air)
dev:
	@echo "Starting development server with hot reload..."
	air -c .air.toml

## Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

## Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## Run linter
lint:
	@echo "Running linter..."
	$(GOLINT) run ./...

## Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

## Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

## Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

## Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

## Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-f deployments/docker/Dockerfile .

## Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 -p 9091:9091 $(DOCKER_IMAGE):$(DOCKER_TAG)

## Tag Docker image for registry
docker-tag:
	@echo "Tagging Docker image for $(DOCKER_REGISTRY)/$(DOCKER_USERNAME)/$(DOCKER_IMAGE)..."
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_REGISTRY)/$(DOCKER_USERNAME)/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_REGISTRY)/$(DOCKER_USERNAME)/$(DOCKER_IMAGE):$(VERSION)
	@if [ "$(DOCKER_TAG)" = "latest" ]; then \
		docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_REGISTRY)/$(DOCKER_USERNAME)/$(DOCKER_IMAGE):latest; \
	fi

## Push Docker image to registry
docker-push: docker-tag
	@echo "Pushing Docker image to $(DOCKER_REGISTRY)/$(DOCKER_USERNAME)/$(DOCKER_IMAGE)..."
	docker push $(DOCKER_REGISTRY)/$(DOCKER_USERNAME)/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_USERNAME)/$(DOCKER_IMAGE):$(VERSION)
	@if [ "$(DOCKER_TAG)" = "latest" ]; then \
		docker push $(DOCKER_REGISTRY)/$(DOCKER_USERNAME)/$(DOCKER_IMAGE):latest; \
	fi

## Build and push Docker image
docker-release: docker-build docker-push
	@echo "Docker image released: $(DOCKER_REGISTRY)/$(DOCKER_USERNAME)/$(DOCKER_IMAGE):$(VERSION)"

## Generate API docs
api-docs:
	@echo "Generating API documentation..."
	@which swag > /dev/null || go install github.com/swaggo/swag/cmd/swag@latest
	swag init -g cmd/resolver/main.go -o docs/swagger

## Generate mocks for testing
mocks:
	@echo "Generating mocks..."
	@which mockgen > /dev/null || go install go.uber.org/mock/mockgen@latest
	go generate ./...

## Verify dependencies
verify:
	@echo "Verifying dependencies..."
	$(GOMOD) verify

## Show help
help:
	@echo "ANS Resolution Server - Available targets:"
	@echo ""
	@grep -E '^##' Makefile | sed 's/## /  /'
	@echo ""
	@echo "Usage: make [target]"

# Serve configuration docs
.PHONY: docs-serve
docs-serve:
	mkdocs serve

# Build configuration docs
.PHONY: docs-build
docs-build:
	mkdocs build

# Install doc dependencies
.PHONY: docs-install
docs-install:
	pip install -r requirements.txt
