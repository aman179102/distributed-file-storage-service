.PHONY: all build run test lint clean docker docker-build docker-up docker-down dev help

APP_NAME := filestore
BUILD_DIR := build
CMD_DIR := cmd/server
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"

all: clean lint test build

build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./$(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

run:
	@echo "Starting $(APP_NAME)..."
	@go run ./$(CMD_DIR)

test:
	@echo "Running tests..."
	@go test ./... -v -count=1 -race -timeout 120s 2>&1 | grep -v "no test files"

test-cover:
	@echo "Running tests with coverage..."
	@go test ./... -count=1 -race -coverprofile=coverage.out -covermode=atomic -timeout 120s
	@go tool cover -func=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-unit:
	@echo "Running unit tests..."
	@go test ./tests/unit/... -v -count=1 -timeout 60s

test-integration:
	@echo "Running integration tests..."
	@go test ./tests/integration/... -v -count=1 -timeout 60s

lint:
	@echo "Running linters..."
	@command -v golangci-lint >/dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@golangci-lint run ./... --timeout=5m || true

vet:
	@echo "Running go vet..."
	@go vet ./...

fmt:
	@echo "Formatting code..."
	@go fmt ./...

tidy:
	@echo "Tidying modules..."
	@go mod tidy
	@go mod verify

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf tmp/
	@rm -f coverage.out coverage.html
	@rm -rf /tmp/test-filestore-*
	@echo "Clean complete"

docker-build:
	@echo "Building Docker image..."
	@docker build -t $(APP_NAME):latest -f deploy/Dockerfile .

docker-up:
	@echo "Starting services..."
	@docker-compose -f deploy/docker-compose.yml up -d --build

docker-down:
	@echo "Stopping services..."
	@docker-compose -f deploy/docker-compose.yml down

docker-logs:
	@docker-compose -f deploy/docker-compose.yml logs -f

dev:
	@echo "Starting development environment..."
	@docker-compose -f deploy/docker-compose.yml up -d postgres redis
	@sleep 3
	@echo "Starting application..."
	@DB_HOST=localhost go run ./$(CMD_DIR)

migration:
	@echo "Creating new migration..."
	@read -p "Migration name: " name; \
	touch src/infrastructure/database/migrations/$$(date +%Y%m%d%H%M%S)_$${name}.sql

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  all              Run clean, lint, test, build"
	@echo "  build            Build the binary"
	@echo "  run              Run the application"
	@echo "  test             Run all tests"
	@echo "  test-cover       Run tests with coverage report"
	@echo "  test-unit        Run unit tests"
	@echo "  test-integration Run integration tests"
	@echo "  lint             Run linters"
	@echo "  vet              Run go vet"
	@echo "  fmt              Format code"
	@echo "  tidy             Tidy go modules"
	@echo "  clean            Clean build artifacts"
	@echo "  docker-build     Build Docker image"
	@echo "  docker-up        Start all services with Docker Compose"
	@echo "  docker-down      Stop all services"
	@echo "  dev              Start development environment"
	@echo "  help             Show this help"
