# One-Shot Prompt: Create tcz-proxy

Create a complete Go application called `tcz-proxy` with the following specification:

## Core Functionality
Build an HTTP proxy server that:
- Listens on a TCP port (default 8080, configurable via `-port` flag)
- Replaces the host part of incoming request URLs based on configuration
- Supports regex-based path mapping to redirect specific requests to different destinations
- Can handle HTTPS destinations even when clients connect via HTTP

## Configuration
The proxy reads from a YAML config file (default `config.yaml`, override with `-config` flag):
```yaml
default_host: https://repo.example.com
path_mappings:
  - from: .*/(\d+)\.x/(aarch64|armhf)/tcz/watchdog\.tcz
    to: https://github.com/asssaf/picore-watchdog/releases/download/$1/watchdog-$2.zip
```

## Path Mapping Logic
1. Check incoming request path against all regex patterns in `path_mappings`
2. If a pattern matches, construct the destination URL using capture groups ($1, $2, etc.)
3. If no pattern matches, replace the host with `default_host`
4. Forward the request, preserving headers, query parameters, and request body

## Command Line Flags
- `-config string`: Path to config file (default "config.yaml")
- `-port string`: Port to listen on (default "8080")
- `-host string`: Override default_host from config

## Requirements
1. **main.go**: Complete proxy implementation with:
   - `Config` struct for YAML parsing
   - `Proxy` struct with compiled regex patterns
   - `ServeHTTP` method implementing the proxy logic
   - `loadConfig` function for YAML parsing
   - Proper error handling and logging

2. **main_test.go**: Comprehensive unit tests covering:
   - Proxy initialization with valid/invalid regex
   - Path mapping with capture groups
   - Default host fallback
   - Header and query parameter preservation
   - Config file loading (valid/invalid cases)
   - End-to-end request/response flow using httptest

3. **config.yaml**: Example configuration with the watchdog.tcz pattern

4. **go.mod**: Module file with `gopkg.in/yaml.v3` dependency

5. **README.md**: Complete documentation with installation, configuration, usage examples, and testing instructions

## Example Usage
```bash
# Start proxy
./tcz-proxy -config config.yaml -port 8080

# Request gets redirected based on pattern
curl -x http://localhost:8080 http://repo.example.com/14.x/aarch64/tcz/watchdog.tcz
# → https://github.com/asssaf/picore-watchdog/releases/download/14/watchdog-aarch64.zip
```

Create all files with production-quality code, proper error handling, and comprehensive test coverage.
