# tcz-proxy

A flexible HTTP proxy server written in Go that can redirect requests based on regex pattern matching and host replacement.

## Features

- **Default Host Replacement**: Proxy all requests to a configured default host
- **Path-Based Routing**: Use regex patterns to redirect specific paths to different destinations
- **HTTPS Support**: Handle HTTPS destinations even when clients connect via HTTP
- **Capture Groups**: Use regex capture groups to construct dynamic destination URLs
- **Command Line Overrides**: Override configuration via command line flags
- **Comprehensive Testing**: Full test suite included

## Installation

```bash
# Clone the repository
git clone <repository-url>
cd tcz-proxy

# Build the binary
go build -o tcz-proxy

# Or run directly
go run main.go
```

## Dependencies

```bash
go get gopkg.in/yaml.v3
```

## Configuration

Create a `config.yaml` file:

```yaml
default_host: https://repo.example.com

path_mappings:
  - from: .*/(\d+)\.x/(aarch64|armhf)/tcz/watchdog\.tcz
    to: https://github.com/asssaf/picore-watchdog/releases/download/$1/watchdog-$2.zip
  
  - from: /api/v(\d+)/(.+)
    to: https://api.example.com/v$1/$2
```

### Configuration Options

- **default_host**: The default host to proxy requests to when no path mapping matches
- **path_mappings**: Array of path mapping rules
  - **from**: Regex pattern to match against the request path
  - **to**: Destination URL with support for capture group references ($1, $2, etc.)

## Usage

### Basic Usage

```bash
# Start with default config.yaml on port 8080
./tcz-proxy

# Specify a custom config file
./tcz-proxy -config /path/to/config.yaml

# Use a different port
./tcz-proxy -port 9090

# Override the default host
./tcz-proxy -host https://mirror.example.com
```

### Command Line Flags

- `-config`: Path to configuration file (default: "config.yaml")
- `-port`: Port to listen on (default: "8080")
- `-host`: Override default host from config file

### Making Requests

Once the proxy is running, configure your client to use it:

```bash
# Using curl
curl -x http://localhost:8080 http://example.com/some/path

# Using wget
wget -e use_proxy=yes -e http_proxy=localhost:8080 http://example.com/some/path

# Set environment variable
export http_proxy=http://localhost:8080
curl http://example.com/some/path
```

## How It Works

1. Client sends HTTP request to the proxy
2. Proxy checks if the request path matches any regex pattern in `path_mappings`
3. If a pattern matches:
   - The destination URL is constructed using the matched pattern and capture groups
   - The request is forwarded to the constructed URL
4. If no pattern matches:
   - The host part of the URL is replaced with `default_host`
   - The request is forwarded to the default host
5. Proxy returns the response to the client

## Examples

### Example 1: Redirecting TinyCore Linux Packages

**Config:**
```yaml
default_host: http://tinycorelinux.net
path_mappings:
  - from: .*/(\d+)\.x/(aarch64|armhf)/tcz/watchdog\.tcz
    to: https://github.com/asssaf/picore-watchdog/releases/download/$1/watchdog-$2.zip
```

**Request:**
```bash
curl -x http://localhost:8080 http://repo.tinycorelinux.net/14.x/aarch64/tcz/watchdog.tcz
```

**Result:** Request is redirected to:
```
https://github.com/asssaf/picore-watchdog/releases/download/14/watchdog-aarch64.zip
```

### Example 2: API Versioning

**Config:**
```yaml
path_mappings:
  - from: /api/v(\d+)/(.+)
    to: https://api-v$1.example.com/$2
```

**Request:**
```bash
curl -x http://localhost:8080 http://myapp.com/api/v2/users
```

**Result:** Request is redirected to:
```
https://api-v2.example.com/users
```

## Testing

Run the complete test suite:

```bash
# Run all tests
go test -v

# Run with coverage
go test -v -cover

# Run specific test
go test -v -run TestServeHTTP_WithPathMapping

# Generate coverage report
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Project Structure

```
tcz-proxy/
├── main.go           # Main proxy implementation
├── main_test.go      # Comprehensive unit tests
├── config.yaml       # Configuration file
├── README.md         # This file
└── go.mod           # Go module file
```

## Error Handling

- If no default host is configured and no path mapping matches, returns 502 Bad Gateway
- If the target server is unreachable, returns 502 Bad Gateway
- If the regex pattern is invalid, the proxy fails to start with an error message
- All errors are logged to stdout

## Security Considerations

- The proxy does not perform authentication
- All headers from the client are forwarded to the target
- X-Forwarded-For header is added to identify the client
- Not recommended for production use without additional security measures

## License

[Your License Here]

## Contributing

Contributions are welcome! Please submit pull requests or open issues for bugs and feature requests.
