.PHONY: all test test-verbose test-coverage coverage clean fmt lint deps help
.SILENT: clean

# Default target
all: test

# Run tests for all modules
test:
	@echo "Running tests for all modules..."
	@echo "\n=== Testing Core ==="
	@cd core && go test ./...
	@echo "\n=== Testing Client ==="
	@cd client && go test ./...
	@echo "\n=== Testing Server ==="
	@cd server && go test ./...

# Run tests in verbose mode for all modules
test-verbose:
	@echo "Running verbose tests for all modules..."
	@echo "\n=== Testing Core (verbose) ==="
	@cd core && go test -v ./...
	@echo "\n=== Testing Client (verbose) ==="
	@cd client && go test -v ./...
	@echo "\n=== Testing Server (verbose) ==="
	@cd server && go test -v ./...

# Run tests with coverage for all modules
test-coverage:
	@echo "Running tests with coverage for all modules..."
	@echo "\n=== Testing Core with coverage ==="
	@cd core && go test -coverprofile=coverage.out ./...
	@cd core && go tool cover -func=coverage.out | tail -1
	@echo "\n=== Testing Client with coverage ==="
	@cd client && go test -coverprofile=coverage.out -covermode=atomic ./... -coverpkg=github.com/Nabia-DB/nabia/client
	@cd client && go tool cover -func=coverage.out | tail -1
	@echo "\n=== Testing Server with coverage ==="
	@cd server && go test -coverprofile=coverage.out ./...
	@cd server && go tool cover -func=coverage.out | tail -1
	@echo "\n=== Combined Coverage Summary ==="
	@echo "Core:   $$(cd core && go tool cover -func=coverage.out 2>/dev/null | tail -1 | awk '{print $$3}')"
	@echo "Client: $$(cd client && go tool cover -func=coverage.out 2>/dev/null | tail -1 | awk '{print $$3}')"
	@echo "Server: $$(cd server && go tool cover -func=coverage.out 2>/dev/null | tail -1 | awk '{print $$3}')"

# Alias for test-coverage
coverage: test-coverage

# Clean all build artifacts and test files
clean:
	@echo "Cleaning all modules..."
	# Core module
	@cd core && rm -f coverage.out coverage.html *.test *.db test*.db
	@cd core && rm -rf vendor/ dist/ build/
	# Client module
	@cd client && rm -f nabia-client coverage.out coverage.html *.test *.db test*.db
	@cd client && rm -rf vendor/ dist/ build/ testing/testdata/
	# Server module
	@cd server && rm -f nabia coverage.out coverage.html *.test *.db test*.db
	@cd server && rm -rf vendor/ dist/ build/
	# Root level
	@rm -f coverage.out coverage.html *.test
	@echo "Clean complete"

# Format code in all modules
fmt:
	@echo "Formatting code in all modules..."
	@cd core && go fmt ./...
	@cd client && go fmt ./...
	@cd server && go fmt ./...

# Run linter on all modules (requires golangci-lint)
lint:
	@echo "Running linter on all modules..."
	@echo "\n=== Linting Core ==="
	@cd core && golangci-lint run
	@echo "\n=== Linting Client ==="
	@cd client && golangci-lint run
	@echo "\n=== Linting Server ==="
	@cd server && golangci-lint run

# Download and tidy dependencies for all modules
deps:
	@echo "Managing dependencies for all modules..."
	@echo "\n=== Core dependencies ==="
	@cd core && go mod download && go mod tidy
	@echo "\n=== Client dependencies ==="
	@cd client && go mod download && go mod tidy
	@echo "\n=== Server dependencies ==="
	@cd server && go mod download && go mod tidy

# Run specific module tests
test-core:
	@cd core && go test ./...

test-client:
	@cd client && go test ./...

test-server:
	@cd server && go test ./...

# Run specific module coverage
coverage-core:
	@cd core && go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1

coverage-client:
	@cd client && go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1

coverage-server:
	@cd server && go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1

# Show help
help:
	@echo "Nabia Monorepo Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  make test           - Run tests for all modules"
	@echo "  make test-verbose   - Run tests in verbose mode for all modules"
	@echo "  make test-coverage  - Run tests with coverage for all modules"
	@echo "  make coverage       - Alias for test-coverage"
	@echo "  make clean          - Clean build artifacts and test files in all modules"
	@echo "  make fmt            - Format Go code in all modules"
	@echo "  make lint           - Run linter on all modules (requires golangci-lint)"
	@echo "  make deps           - Download and tidy dependencies for all modules"
	@echo ""
	@echo "Module-specific targets:"
	@echo "  make test-core      - Run tests for core module only"
	@echo "  make test-client    - Run tests for client module only"
	@echo "  make test-server    - Run tests for server module only"
	@echo "  make coverage-core  - Run coverage for core module only"
	@echo "  make coverage-client- Run coverage for client module only"
	@echo "  make coverage-server- Run coverage for server module only"
	@echo ""
	@echo "  make help           - Show this help message"