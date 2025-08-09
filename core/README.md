# nabia-core

[![Build Status](https://github.com/Nabia-DB/nabia/workflows/CI/badge.svg)](https://github.com/Nabia-DB/nabia/actions)
[![Coverage Status](https://codecov.io/gh/Nabia-DB/nabia/branch/main/graph/badge.svg)](https://codecov.io/gh/Nabia-DB/nabia)
[![Latest Release](https://img.shields.io/github/v/release/Nabia-DB/nabia)](https://github.com/Nabia-DB/nabia/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nabia-DB/nabia)](https://goreportcard.com/report/github.com/Nabia-DB/nabia)

Lightweight in-memory key-value database engine library used by Nabia.

## Quick Start

### Prerequisites
- Go 1.22 or higher

### Installation

```bash
# Clone the repository
git clone https://github.com/Nabia-DB/nabia.git
cd nabia/core

# Install dependencies
go mod download

# Build the library
go build ./...
```

### Usage as a Library

Add nabia-core to your Go project:

```bash
go get github.com/Nabia-DB/nabia/core
```

#### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/Nabia-DB/nabia/core/engine"
)

func main() {
    // Create a new database
    db, err := engine.NewNabiaDB("mydata.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Stop() // Saves to disk before closing

    // Write data
    err = db.Write("user:123", []byte("John Doe"))
    if err != nil {
        log.Fatal(err)
    }

    // Read data
    data, err := db.Read("user:123")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("User: %s\n", string(data))

    // Check if key exists
    if db.Exists("user:123") {
        fmt.Println("User exists!")
    }

    // Delete data
    db.Delete("user:123")
}
```

#### Loading Existing Database

```go
db, err := engine.LoadFromFile("existing.db")
if err != nil {
    log.Fatal(err)
}
defer db.Stop()
```

## Features

- **In-Memory Storage**: Lightning-fast read/write operations
- **Persistence**: Automatic saving to disk using Go's gob encoding
- **Concurrency**: Thread-safe operations using sync.Map
- **Metrics**: Built-in read/write/size tracking
- **Simple API**: Clean, intuitive interface

## API Reference

### Core Functions

- `NewNabiaDB(location string) (*NabiaDB, error)` - Create a new database
- `LoadFromFile(filename string) (*NabiaDB, error)` - Load existing database
- `Write(key string, value []byte) error` - Store key-value pair
- `Read(key string) ([]byte, error)` - Retrieve value by key
- `Exists(key string) bool` - Check if key exists
- `Delete(key string)` - Remove key-value pair
- `Stop()` - Save to disk and stop database
- `SaveToFile(filename string) error` - Manually save to disk

### Data Types

- `NabiaRecord` - Represents a value in the database
- `NabiaDB` - Main database structure

## Performance Characteristics

- **Read Operations**: O(1) average case using hash map
- **Write Operations**: O(1) average case with concurrent safety
- **Memory Usage**: Efficient storage with minimal overhead
- **Persistence**: Fast serialization using gob encoding

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...
```

### Benchmarking

The core includes comprehensive benchmarks:

```bash
go test -bench=. ./...
```

## Thread Safety

All operations are thread-safe and can be used concurrently from multiple goroutines. The engine uses `sync.Map` internally for optimal concurrent performance.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Merge Request

## License

See [LICENSE](LICENSE) file for details.

## About Nabia

[Nabia](https://en.wikipedia.org/wiki/Nabia) is the goddess of rivers and water in Celtiberian mythology. This core library allows your data to flow quickly in and out of memory, providing the foundation for high-performance applications.
