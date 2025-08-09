# nabia-client

[![Build Status](https://github.com/Nabia-DB/nabia/workflows/CI/badge.svg)](https://github.com/Nabia-DB/nabia/actions)
[![Coverage Status](https://codecov.io/gh/Nabia-DB/nabia/branch/main/graph/badge.svg)](https://codecov.io/gh/Nabia-DB/nabia)
[![Latest Release](https://img.shields.io/github/v/release/Nabia-DB/nabia)](https://github.com/Nabia-DB/nabia/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nabia-DB/nabia)](https://goreportcard.com/report/github.com/Nabia-DB/nabia)

Nabia-compatible HTTP client for interacting with Nabia servers.

## Quick Start

### Prerequisites

- Go 1.22 or higher
- A running Nabia server instance

### Installation

```bash
# Clone the repository
git clone https://github.com/Nabia-DB/nabia.git
cd nabia/client

# Install dependencies
go mod download

# Build the application
go build -o nabia-client .
```

### Usage

The nabia-client supports all HTTP methods that the Nabia server implements:

#### Configuration

The client can be configured using command-line flags or environment variables:

```bash
# Command-line flags
./nabia-client --host localhost --port 5380 GET mykey

# Environment variables
export NABIA_HOST=localhost
export NABIA_PORT=5380
./nabia-client GET mykey
```

#### Basic Operations

**GET - Retrieve a value:**

```bash
./nabia-client GET mykey
```

**POST - Create a new key-value pair:**

```bash
# From command line
./nabia-client POST mykey "Hello, World!"

# From file
./nabia-client POST mykey --file /path/to/file.txt
```

**PUT - Create or update a key-value pair:**

```bash
# From command line
./nabia-client PUT mykey "Updated value"

# From file
./nabia-client PUT mykey --file /path/to/file.txt
```

**DELETE - Remove a key:**

```bash
./nabia-client DELETE mykey
```

**HEAD - Check if a key exists:**

```bash
./nabia-client HEAD mykey
```

**OPTIONS - Get allowed methods for a key:**

```bash
./nabia-client OPTIONS mykey
```

#### Working with Files

The client can handle file uploads and downloads:

```bash
# Upload a file
./nabia-client POST image.jpg --file /path/to/image.jpg

# The client automatically detects MIME types for files
```

#### Content Types

The client automatically handles content types:

- Text data is sent as `text/plain; charset=utf-8`
- File uploads use automatic MIME type detection
- Binary data defaults to `application/octet-stream`

## API Compatibility

This client is compatible with the Nabia server API. [Read the full specification](spec.md).

## Development

### Running Tests

```bash
go test ./...
```

### Building from Source

```bash
go build -o nabia-client .
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Merge Request

## License

See [LICENSE](LICENSE) file for details.
