# URL Expander

[![Go Reference](https://pkg.go.dev/badge/github.com/YOURUSERNAME/url-expander.svg)](https://pkg.go.dev/github.com/YOURUSERNAME/url-expander)
[![Go Report Card](https://goreportcard.com/badge/github.com/YOURUSERNAME/url-expander)](https://goreportcard.com/report/github.com/YOURUSERNAME/url-expander)

A Go library and CLI tool for expanding short URLs from services like **TinyURL, bit.ly, shorturl.at, t.co**, and any other URL shortening service.

## Features

 **Universal Support**: Works with any URL shortening service (TinyURL, Bitly, t.co, shorturl.at, is.gd, etc.) **Detailed Redirect Chain**: Track every redirect hop with status codes and timing **Batch Processing**: Expand multiple URLs concurrently **Configurable**: Customize timeout, max redirects, user agent, retry logic **Context Support**: Full support for Go contexts (cancellation, timeout) **Loop Detection**: Detects and handles redirect loops **Retry Logic**: Built-in retry mechanism for transient failures **JSON Output**: Machine-readable JSON output for integration **Production Ready**: Comprehensive test suite, proper error handling

## Installation

### As a Library

```bash
go get github.com/kapilsingh421/url-expander
```

### As a CLI Tool

```bash
git clone https://github.com/kapilsingh421/url-expander.git
cd url-expander
go build -o url-expander .
```

## Quick Start

### Library Usage

```go
package main

import (
    "context"
    "fmt"

    "github.com/kapilsingh421/url-expander/expander"
)

func main() {
    // Create expander with default config
    exp := expander.New(nil)
    defer exp.Close()

    // Simple expansion - just get the final URL
    finalURL, err := exp.ExpandSimple(context.Background(), "https://shorturl.at/g0iU7")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Expanded: %s\n", finalURL)
    // Output: Expanded: https://www.icc-cricket.com/
}
```

### CLI Usage

```bash
# Expand a single URL
./url-expander -url "https://shorturl.at/g0iU7"

# Expand with verbose output (show redirect chain)
./url-expander -url "https://shorturl.at/g0iU7" -verbose

# Get JSON output
./url-expander -url "https://shorturl.at/g0iU7" -json

# Expand multiple URLs concurrently
./url-expander -urls "https://bit.ly/url1,https://tinyurl.com/url2" -concurrency 10

# Custom timeout
./url-expander -url "https://example.com" -timeout 60s
```

## Examples

### Basic Expansion

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/kapilsingh421/url-expander/expander"
)

func main() {
    exp := expander.New(nil)
    defer exp.Close()

    finalURL, err := exp.ExpandSimple(context.Background(), "https://bit.ly/example")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(finalURL)
}
```

### Detailed Expansion with Redirect Chain

```go
package main

import (
    "context"
    "fmt"

    "github.com/kapilsingh421/url-expander/expander"
)

func main() {
    exp := expander.New(nil)
    defer exp.Close()

    result := exp.Expand(context.Background(), "https://shorturl.at/g0iU7")

    fmt.Printf("Original: %s\n", result.OriginalURL)
    fmt.Printf("Final:    %s\n", result.FinalURL)
    fmt.Printf("Hops:     %d\n", result.Hops)
    fmt.Printf("Duration: %s\n", result.TotalTime)

    // Print each redirect
    for i, r := range result.RedirectChain {
        fmt.Printf("  [%d] %d: %s -> %s\n", i+1, r.StatusCode, r.FromURL, r.ToURL)
    }
}
```

### Batch Processing

```go
package main

import (
    "context"
    "fmt"

    "github.com/kapilsingh421/url-expander/expander"
)

func main() {
    exp := expander.New(nil)
    defer exp.Close()

    urls := []string{
        "https://bit.ly/url1",
        "https://tinyurl.com/url2",
        "https://t.co/url3",
        "https://shorturl.at/url4",
    }

    // Expand all URLs with concurrency of 5
    results := exp.ExpandBatch(context.Background(), urls, 5)

    for _, result := range results {
        fmt.Printf("%s -> %s\n", result.OriginalURL, result.FinalURL)
    }
}
```

### Custom Configuration

```go
package main

import (
    "context"
    "time"

    "github.com/kapilsingh421/url-expander/expander"
)

func main() {
    config := &expander.Config{
        Timeout:       60 * time.Second,  // Request timeout
        MaxRedirects:  30,                 // Max redirects to follow
        UserAgent:     "MyApp/1.0",        // Custom user agent
        RetryCount:    3,                  // Retry failed requests
        RetryDelay:    1 * time.Second,    // Delay between retries
        SkipTLSVerify: false,              // Keep TLS verification
        CustomHeaders: map[string]string{
            "X-Custom-Header": "value",
        },
    }

    exp := expander.New(config)
    defer exp.Close()

    result := exp.Expand(context.Background(), "https://bit.ly/example")
    // Use result...
}
```

### With Context Timeout

```go
package main

import (
    "context"
    "time"

    "github.com/kapilsingh421/url-expander/expander"
)

func main() {
    exp := expander.New(nil)
    defer exp.Close()

    // Create context with 5 second timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    result := exp.Expand(ctx, "https://bit.ly/example")
    if result.Error != nil {
        // Handle timeout or other error
    }
}
```

### Detect Short URLs

```go
package main

import (
    "fmt"

    "github.com/kapilsingh421/url-expander/expander"
)

func main() {
    urls := []string{
        "https://bit.ly/abc123",       // Short URL
        "https://www.google.com",      // Regular URL
        "https://tinyurl.com/xyz",     // Short URL
        "https://shorturl.at/abc",     // Short URL
    }

    for _, url := range urls {
        if expander.IsShortURL(url) {
            fmt.Printf("✓ %s is a short URL\n", url)
        } else {
            fmt.Printf("✗ %s is NOT a short URL\n", url)
        }
    }
}
```

## API Reference

### Types

#### `Expander`
Main expander struct. Create with `New(config)`.

#### `Config`
```go
type Config struct {
    Timeout           time.Duration         // Request timeout (default: 30s)
    MaxRedirects      int                   // Max redirects (default: 20)
    UserAgent         string                // User agent string
    FollowMetaRefresh bool                  // Follow HTML meta refresh
    SkipTLSVerify     bool                  // Skip TLS verification
    CustomHeaders     map[string]string     // Custom HTTP headers
    RetryCount        int                   // Retry count (default: 2)
    RetryDelay        time.Duration         // Delay between retries
}
```

#### `ExpansionResult`
```go
type ExpansionResult struct {
    OriginalURL   string          // The input URL
    ExpandedURL   string          // Alias for FinalURL
    FinalURL      string          // The final destination URL
    RedirectChain []RedirectInfo  // List of all redirects
    TotalTime     time.Duration   // Total time taken
    Hops          int             // Number of redirect hops
    Error         error           // Any error that occurred
}
```

#### `RedirectInfo`
```go
type RedirectInfo struct {
    FromURL    string        // Source URL
    ToURL      string        // Destination URL
    StatusCode int           // HTTP status code (301, 302, etc.)
    Duration   time.Duration // Time for this hop
}
```

### Functions

| Function | Description |
|----------|-------------|
| `New(config *Config) *Expander` | Create new expander instance |
| `DefaultConfig() *Config` | Get default configuration |
| `IsShortURL(url string) bool` | Check if URL is likely a short URL |

### Methods

| Method | Description |
|--------|-------------|
| `Expand(ctx, url) *ExpansionResult` | Expand URL with full details |
| `ExpandSimple(ctx, url) (string, error)` | Expand URL, return just final URL |
| `ExpandBatch(ctx, urls, concurrency) []*ExpansionResult` | Expand multiple URLs concurrently |
| `Close()` | Release resources |

## Supported URL Shorteners

Works with **any** URL shortening service, including:

| Service | Domain |
|---------|--------|
| Bitly | bit.ly, j.mp |
| TinyURL | tinyurl.com |
| ShortURL | shorturl.at |
| Twitter | t.co |
| Google | goo.gl |
| Hootsuite | ow.ly |
| is.gd | is.gd, v.gd |
| Rebrandly | rebrand.ly |
| Short.io | short.io |
| Cutt.ly | cutt.ly |
| Tiny.cc | tiny.cc |
| And many more... | |

## Running Tests

```bash
# Run all tests
go test ./expander/... -v

# Run with race detection
go test ./expander/... -race

# Run benchmarks
go test ./expander/... -bench=.

# Run with coverage
go test ./expander/... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Project Structure

```
url-expander/
├── expander/
│   ├── expander.go       # Core library
│   └── expander_test.go  # Unit tests
├── main.go               # CLI application
├── go.mod
└── README.md
```

## Contributing

Fork the repositoryCreate your feature branch (`git checkout -b feature/amazing-feature`)Commit your changes (`git commit -m 'Add amazing feature'`)Push to the branch (`git push origin feature/amazing-feature`)Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Setup for Publishing

To publish this package:

Replace `YOURUSERNAME` with your GitHub username in:`go.mod``main.go`All files in `examples/`This README

Create a GitHub repository named `url-expander`

Push your code:   ```bash
   git init
   git add .
   git commit -m "Initial commit"
   git remote add origin https://github.com/YOURUSERNAME/url-expander.git
   git push -u origin main
   ```

Create a release tag:   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

Users can then install via:   ```bash
   go get github.com/YOURUSERNAME/url-expander@latest
   ```