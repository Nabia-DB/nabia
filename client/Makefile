.PHONY: all build test test-verbose test-coverage coverage clean run help

# Default target
all: clean build

# Build the application
build:
	go build -o nabia-client .

# Run tests
test:
	go test ./...

# Run tests in verbose mode
test-verbose:
	go test -v ./...

# Run tests with coverage (only covers main package, not test utilities)
test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./... -coverpkg=github.com/Nabia-DB/nabia/client
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@go tool cover -func=coverage.out | tail -1

# Alias for test-coverage
coverage: test-coverage

# Clean build artifacts and test files
clean:
	rm -f nabia-client
	rm -f *.db test*.db
	rm -f coverage.out coverage.html
	rm -f *.test
	rm -rf vendor/ dist/ build/
	rm -rf testing/testdata/

# Run the application (requires arguments)
run: build
	@echo "Usage: ./nabia-client [OPTIONS] METHOD KEY [DATA]"
	@echo "Run './nabia-client --help' for more information"

# Format code
fmt:
	go fmt ./...

# Run linter (requires golangci-lint to be installed)
lint:
	golangci-lint run

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  make build         - Build the application"
	@echo "  make test          - Run tests"
	@echo "  make test-verbose  - Run tests in verbose mode"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make coverage      - Alias for test-coverage"
	@echo "  make clean         - Clean build artifacts and test files"
	@echo "  make run           - Build and show usage (client requires arguments)"
	@echo "  make fmt           - Format Go code"
	@echo "  make lint          - Run linter (requires golangci-lint)"
	@echo "  make deps          - Download and tidy dependencies"
	@echo "  make bench         - Run benchmarks"
	@echo "  make help          - Show this help message"