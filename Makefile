.PHONY: build run clean test docker-up docker-down install

# Build variables
APP_NAME := rag-cli
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS := -ldflags "-X 'rag-cli/pkg/version.Version=$(VERSION)' \
                    -X 'rag-cli/pkg/version.GitCommit=$(GIT_COMMIT)' \
                    -X 'rag-cli/pkg/version.BuildDate=$(BUILD_DATE)'"

# Build the CLI tool
build:
	go build $(LDFLAGS) -o $(APP_NAME)

# Build with race detector and debug info
build-dev:
	go build -race $(LDFLAGS) -o $(APP_NAME)

# Run the CLI tool
run:
	go run main.go

# Clean build artifacts
clean:
	rm -f rag-cli

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Start Docker services (ChromaDB only for native Ollama setup)
docker-up:
	cd docker && docker-compose -f docker-compose-chroma-only.yaml up -d

# Stop Docker services
docker-down:
	cd docker && docker-compose -f docker-compose-chroma-only.yaml down

# Install dependencies
deps:
	go mod tidy
	go mod download

# Install the CLI tool globally
install:
	go install $(LDFLAGS) .

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Generate documentation
docs:
	go run main.go --help > docs/help.txt

# Pull required models
models:
	./scripts/pull-models.sh

# Setup development environment
setup:
	@echo "Setting up development environment..."
	@echo "1. Installing dependencies..."
	@make deps
	@echo "2. Starting Docker services..."
	@make docker-up
	@echo "3. Pulling models..."
	@make models
	@echo "4. Building CLI tool..."
	@make build
	@echo "Setup complete! Run 'make run' to start."

# Help command
help:
	@echo "Available commands:"
	@echo "  build       - Build the CLI tool"
	@echo "  build-dev   - Build with race detector and debug info"
	@echo "  run         - Run the CLI tool"
	@echo "  clean       - Clean build artifacts"
	@echo "  test        - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  docker-up   - Start Docker services (Ollama + ChromaDB)"
	@echo "  docker-down - Stop Docker services"
	@echo "  models      - Pull required models into Docker containers"
	@echo "  deps        - Install dependencies"
	@echo "  install     - Install CLI tool globally"
	@echo "  fmt         - Format code"
	@echo "  lint        - Lint code"
	@echo "  docs        - Generate documentation"
	@echo "  setup       - Full setup (Docker + models + build)"
	@echo "  help        - Show this help message"
