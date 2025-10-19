package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewProxy(t *testing.T) {
	mappings := []PathMapping{
		{From: `/test/(\d+)`, To: `https://example.com/$1`},
	}

	proxy, err := NewProxy("https://default.com", mappings)
	if err != nil {
		t.Fatalf("NewProxy failed: %v", err)
	}

	if proxy == nil {
		t.Fatal("NewProxy returned nil")
	}

	if proxy.client == nil {
		t.Error("proxy.client is nil")
	}

	if proxy.defaultHost != "https://default.com" {
		t.Errorf("Expected default host 'https://default.com', got '%s'", proxy.defaultHost)
	}

	if len(proxy.compiledMappings) != 1 {
		t.Errorf("Expected 1 compiled mapping, got %d", len(proxy.compiledMappings))
	}
}

func TestNewProxy_InvalidRegex(t *testing.T) {
	mappings := []PathMapping{
		{From: `[invalid(regex`, To: `https://example.com`},
	}

	_, err := NewProxy("https://default.com", mappings)
	if err == nil {
		t.Error("Expected error for invalid regex, got nil")
	}

	if !strings.Contains(err.Error(), "invalid regex pattern") {
		t.Errorf("Expected error message about invalid regex, got: %v", err)
	}
}

func TestFindMapping(t *testing.T) {
	mappings := []PathMapping{
		{From: `.*/(\d+)\.x/(aarch64|armhf)/tcz/watchdog\.tcz`, To: `https://github.com/releases/download/$1/watchdog-$2.zip`},
		{From: `/api/v(\d+)/users`, To: `https://api.example.com/v$1/users`},
	}

	proxy, err := NewProxy("https://default.com", mappings)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected string
		found    bool
	}{
		{
			name:     "Match watchdog pattern with aarch64",
			path:     "/repo/14.x/aarch64/tcz/watchdog.tcz",
			expected: "https://github.com/releases/download/14/watchdog-aarch64.zip",
			found:    true,
		},
		{
			name:     "Match watchdog pattern with armhf",
			path:     "/mirror/15.x/armhf/tcz/watchdog.tcz",
			expected: "https://github.com/releases/download/15/watchdog-armhf.zip",
			found:    true,
		},
		{
			name:     "Match API pattern",
			path:     "/api/v2/users",
			expected: "https://api.example.com/v2/users",
			found:    true,
		},
		{
			name:     "No match",
			path:     "/some/other/path",
			expected: "",
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := proxy.findMapping(tt.path)

			if found != tt.found {
				t.Errorf("Expected found=%v, got found=%v", tt.found, found)
			}

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestBuildTargetURL_WithMapping(t *testing.T) {
	mappings := []PathMapping{
		{From: `/test/(\d+)`, To: `https://mapped.com/item/$1`},
	}

	proxy, _ := NewProxy("https://default.com", mappings)

	req := httptest.NewRequest("GET", "http://localhost/test/123", nil)
	targetURL, err := proxy.buildTargetURL(req)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := "https://mapped.com/item/123"
	if targetURL != expected {
		t.Errorf("Expected '%s', got '%s'", expected, targetURL)
	}
}

func TestBuildTargetURL_WithDefaultHost(t *testing.T) {
	proxy, _ := NewProxy("https://default.com", nil)

	req := httptest.NewRequest("GET", "http://localhost/some/path?key=value", nil)
	targetURL, err := proxy.buildTargetURL(req)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := "https://default.com/some/path?key=value"
	if targetURL != expected {
		t.Errorf("Expected '%s', got '%s'", expected, targetURL)
	}
}

func TestBuildTargetURL_NoDefaultNoMapping(t *testing.T) {
	proxy, _ := NewProxy("", nil)

	req := httptest.NewRequest("GET", "http://localhost/test", nil)
	_, err := proxy.buildTargetURL(req)

	if err == nil {
		t.Error("Expected error when no default host and no mapping match")
	}
}

func TestServeHTTP_WithDefaultHost(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success from backend"))
	}))
	defer backend.Close()

	proxy, _ := NewProxy(backend.URL, nil)

	req := httptest.NewRequest("GET", "http://localhost/test/path", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if string(body) != "success from backend" {
		t.Errorf("Expected body 'success from backend', got '%s'", string(body))
	}

	if resp.Header.Get("X-Test-Header") != "test-value" {
		t.Error("Expected X-Test-Header to be forwarded")
	}
}

func TestServeHTTP_WithPathMapping(t *testing.T) {
	var receivedPath string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mapped content"))
	}))
	defer backend.Close()

	mappings := []PathMapping{
		{From: `/test/(\d+)`, To: backend.URL + `/mapped/$1`},
	}

	proxy, _ := NewProxy("https://default.com", mappings)

	req := httptest.NewRequest("GET", "http://localhost/test/456", nil)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if string(body) != "mapped content" {
		t.Errorf("Expected body 'mapped content', got '%s'", string(body))
	}

	if receivedPath != "/mapped/456" {
		t.Errorf("Expected path '/mapped/456', got '%s'", receivedPath)
	}
}

func TestServeHTTP_HTTPSMapping(t *testing.T) {
	// Use a real HTTPS endpoint for testing
	mappings := []PathMapping{
		{From: `/github/(.+)`, To: `https://httpbin.org/status/$1`},
	}

	proxy, _ := NewProxy("http://localhost", mappings)

	req := httptest.NewRequest("GET", "http://localhost/github/200", nil)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Note: This test requires internet connectivity")
		t.Logf("Status: %d", resp.StatusCode)
	}
}

func TestServeHTTP_PreservesQueryParams(t *testing.T) {
	var receivedQuery string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy, _ := NewProxy(backend.URL, nil)

	req := httptest.NewRequest("GET", "http://localhost/path?key1=value1&key2=value2", nil)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if receivedQuery != "key1=value1&key2=value2" {
		t.Errorf("Expected query 'key1=value1&key2=value2', got '%s'", receivedQuery)
	}
}

func TestServeHTTP_PreservesHeaders(t *testing.T) {
	headerChecks := make(map[string]string)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerChecks["User-Agent"] = r.Header.Get("User-Agent")
		headerChecks["X-Custom"] = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy, _ := NewProxy(backend.URL, nil)

	req := httptest.NewRequest("GET", "http://localhost/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("X-Custom", "custom-value")
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if headerChecks["User-Agent"] != "test-agent" {
		t.Error("User-Agent header not preserved")
	}

	if headerChecks["X-Custom"] != "custom-value" {
		t.Error("Custom header not preserved")
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `default_host: https://example.com
path_mappings:
  - from: /test/(\d+)
    to: https://mapped.com/$1
  - from: /api/(.+)
    to: https://api.com/$1
`

	tmpfile, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	config, err := loadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.DefaultHost != "https://example.com" {
		t.Errorf("Expected default host 'https://example.com', got '%s'", config.DefaultHost)
	}

	if len(config.PathMappings) != 2 {
		t.Errorf("Expected 2 path mappings, got %d", len(config.PathMappings))
	}

	if config.PathMappings[0].From != `/test/(\d+)` {
		t.Errorf("Expected first mapping from '/test/(\\d+)', got '%s'", config.PathMappings[0].From)
	}

	if config.PathMappings[0].To != `https://mapped.com/$1` {
		t.Errorf("Expected first mapping to 'https://mapped.com/$1', got '%s'", config.PathMappings[0].To)
	}
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	_, err := loadConfig("nonexistent.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write invalid YAML
	if _, err := tmpfile.Write([]byte("invalid: yaml: content: [")); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = loadConfig(tmpfile.Name())
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestComplexRegexMapping(t *testing.T) {
	mappings := []PathMapping{
		{From: `.*/(\d+)\.x/(aarch64|armhf)/tcz/watchdog\.tcz`, To: `https://github.com/asssaf/picore-watchdog/releases/download/$1/watchdog-$2.zip`},
	}

	proxy, err := NewProxy("https://default.com", mappings)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	tests := []struct {
		path     string
		expected string
	}{
		{
			path:     "/repo/14.x/aarch64/tcz/watchdog.tcz",
			expected: "https://github.com/asssaf/picore-watchdog/releases/download/14/watchdog-aarch64.zip",
		},
		{
			path:     "/mirror/15.x/armhf/tcz/watchdog.tcz",
			expected: "https://github.com/asssaf/picore-watchdog/releases/download/15/watchdog-armhf.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result, found := proxy.findMapping(tt.path)
			if !found {
				t.Error("Expected to find mapping")
			}
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
