.PHONY: all build test test-verbose test-coverage clean run help

# Default target
all: clean build

# Build the application
build:
	go build -o nabia .

# Run tests
test:
	go test ./...

# Run tests in verbose mode
test-verbose:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@go tool cover -func=coverage.out | tail -1

# Clean build artifacts and test files
clean:
	rm -f nabia
	rm -f *.db
	rm -f test*.db
	rm -f coverage.out coverage.html

# Run the application
run: build
	./nabia

# Development run (builds and runs)
dev: clean build run

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

# Show help
help:
	@echo "Available targets:"
	@echo "  make build         - Build the application"
	@echo "  make test          - Run tests"
	@echo "  make test-verbose  - Run tests in verbose mode"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make clean         - Clean build artifacts and test files"
	@echo "  make run           - Build and run the application"
	@echo "  make dev           - Clean, build, and run (for development)"
	@echo "  make fmt           - Format Go code"
	@echo "  make lint          - Run linter (requires golangci-lint)"
	@echo "  make deps          - Download and tidy dependencies"
	@echo "  make help          - Show this help message"