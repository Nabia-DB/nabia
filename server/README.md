# Nabia

[![pipeline status](https://gitlab.com/Nabia-DB/nabia/server/badges/main/pipeline.svg)](https://gitlab.com/Nabia-DB/nabia/server/-/commits/main)
[![coverage report](https://gitlab.com/Nabia-DB/nabia/server/badges/main/coverage.svg)](https://gitlab.com/Nabia-DB/nabia/server/-/commits/main)
[![Latest Release](https://gitlab.com/Nabia-DB/nabia/server/-/badges/release.svg)](https://gitlab.com/Nabia-DB/nabia/server/-/releases)
[![Go Report Card](https://goreportcard.com/badge/gitlab.com/Nabia-DB/nabia/server)](https://goreportcard.com/report/gitlab.com/Nabia-DB/nabia/server)

In-memory HTTP API for the Nabia library.

## Quick Start

### Prerequisites
- Go 1.22 or higher
- Make (optional, for using Makefile commands)

### Installation

```bash
# Clone the repository
git clone https://gitlab.com/Nabia-DB/nabia/server.git
cd server

# Install dependencies
go mod download

# Build the application
make build
# or without make:
go build -o nabia .
```

### Running

```bash
# Run with default configuration
make run
# or without make:
./nabia

# Run in development mode (clean, build, and run)
make dev
```

### Testing

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Run tests with coverage report
make test-coverage
```

### Configuration

Nabia looks for a `config.yaml` file in the following locations:
- `/etc/nabia/`
- `$HOME/.nabia`
- Current directory

Example configuration:
```yaml
db_location: "server.db"
port: "5380"
```

## API Documentation

Nabia provides a simple REST API for key-value storage:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET    | `/{key}` | Retrieve value for a key |
| POST   | `/{key}` | Create a new key-value pair (fails if exists) |
| PUT    | `/{key}` | Create or update a key-value pair |
| DELETE | `/{key}` | Delete a key-value pair |
| HEAD   | `/{key}` | Check if a key exists |
| OPTIONS| `/{key}` | Get allowed methods for a key |

All requests that send data (POST, PUT) must include a valid `Content-Type` header.

## Development

### Available Make Commands

```bash
make help              # Show all available commands
make build            # Build the application
make test             # Run tests
make test-verbose     # Run tests in verbose mode
make test-coverage    # Run tests with coverage report
make clean            # Clean build artifacts and test files
make run              # Build and run the application
make dev              # Clean, build, and run (for development)
make fmt              # Format Go code
make lint             # Run linter (requires golangci-lint)
make deps             # Download and tidy dependencies
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Merge Request

## License

See [LICENSE](LICENSE) file for details.

# FAQ

**Q: What does Nabia stand for?**

[Nabia](https://en.wikipedia.org/wiki/Nabia) is the goddess of rivers and water in Celtiberian mythology. Nabia allows your data to quickly flow in and out (like water) of RAM through a simple REST API. Nabia keeps your data secure and protected, free from data loss and using advanced parallelization and sharding techniques, allows you to have a highly reliable, high throughput database.
