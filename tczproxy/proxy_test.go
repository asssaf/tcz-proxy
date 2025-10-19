package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewProxy(t *testing.T) {
	mirrors := []string{"https://mirror1.com", "https://mirror2.com"}
	proxy := NewProxy(mirrors)

	if proxy == nil {
		t.Fatal("NewProxy returned nil")
	}

	if proxy.client == nil {
		t.Error("proxy.client is nil")
	}

	if len(proxy.mirrors) != 2 {
		t.Errorf("Expected 2 mirrors, got %d", len(proxy.mirrors))
	}
}

func TestReplaceHost(t *testing.T) {
	proxy := NewProxy(nil)

	tests := []struct {
		name        string
		originalURL string
		newHost     string
		expected    string
		shouldError bool
	}{
		{
			name:        "Replace HTTP host",
			originalURL: "http://example.com/path/to/file",
			newHost:     "https://mirror.com",
			expected:    "https://mirror.com/path/to/file",
			shouldError: false,
		},
		{
			name:        "Replace with query params",
			originalURL: "http://example.com/path?query=value",
			newHost:     "https://mirror.com",
			expected:    "https://mirror.com/path?query=value",
			shouldError: false,
		},
		{
			name:        "Replace HTTPS with HTTP",
			originalURL: "https://secure.com/file",
			newHost:     "http://mirror.com",
			expected:    "http://mirror.com/file",
			shouldError: false,
		},
		{
			name:        "Invalid original URL",
			originalURL: "://invalid",
			newHost:     "https://mirror.com",
			expected:    "",
			shouldError: true,
		},
		{
			name:        "Invalid mirror URL",
			originalURL: "https://example.com/path",
			newHost:     "://invalid",
			expected:    "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := proxy.replaceHost(tt.originalURL, tt.newHost)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestServeHTTP_Success(t *testing.T) {
	// Create a test backend server that returns 200
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer backend.Close()

	// Create proxy without mirrors
	proxy := NewProxy(nil)

	// Create a test request through the proxy
	req := httptest.NewRequest("GET", backend.URL+"/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if string(body) != "success" {
		t.Errorf("Expected body 'success', got '%s'", string(body))
	}

	if resp.Header.Get("X-Test-Header") != "test-value" {
		t.Error("Expected X-Test-Header to be forwarded")
	}
}

func TestServeHTTP_404_NoMirrors(t *testing.T) {
	// Create a test backend that returns 404
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer backend.Close()

	// Create proxy without mirrors
	proxy := NewProxy(nil)

	req := httptest.NewRequest("GET", backend.URL+"/missing", nil)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestServeHTTP_404_WithMirrorFallback(t *testing.T) {
	// Track which servers were called
	var callOrder []string

	// Primary server returns 404
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "primary")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer primary.Close()

	// First mirror also returns 404
	mirror1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "mirror1")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer mirror1.Close()

	// Second mirror returns 200
	mirror2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "mirror2")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("found on mirror"))
	}))
	defer mirror2.Close()

	// Create proxy with mirrors
	proxy := NewProxy([]string{mirror1.URL, mirror2.URL})

	req := httptest.NewRequest("GET", primary.URL+"/file.txt", nil)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if string(body) != "found on mirror" {
		t.Errorf("Expected body 'found on mirror', got '%s'", string(body))
	}

	// Verify call order
	expectedOrder := []string{"primary", "mirror1", "mirror2"}
	if len(callOrder) != len(expectedOrder) {
		t.Errorf("Expected %d calls, got %d", len(expectedOrder), len(callOrder))
	}

	for i, expected := range expectedOrder {
		if i >= len(callOrder) || callOrder[i] != expected {
			t.Errorf("Expected call order %v, got %v", expectedOrder, callOrder)
			break
		}
	}
}

func TestServeHTTP_404_AllMirrorsFail(t *testing.T) {
	// All servers return 404
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	})

	primary := httptest.NewServer(handler)
	defer primary.Close()

	mirror1 := httptest.NewServer(handler)
	defer mirror1.Close()

	mirror2 := httptest.NewServer(handler)
	defer mirror2.Close()

	proxy := NewProxy([]string{mirror1.URL, mirror2.URL})

	req := httptest.NewRequest("GET", primary.URL+"/missing", nil)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestServeHTTP_PreservesHeaders(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if headers were forwarded
		if r.Header.Get("User-Agent") != "test-agent" {
			t.Error("User-Agent header not forwarded")
		}
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Error("Custom header not forwarded")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy := NewProxy(nil)

	req := httptest.NewRequest("GET", backend.URL, nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("X-Custom-Header", "custom-value")
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestServeHTTP_PathPreservation(t *testing.T) {
	// Verify that paths are preserved when using mirrors
	var receivedPath string

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer primary.Close()

	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path + "?" + r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer mirror.Close()

	proxy := NewProxy([]string{mirror.URL})

	req := httptest.NewRequest("GET", primary.URL+"/path/to/file?key=value", nil)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	expectedPath := "/path/to/file?key=value"
	if !strings.Contains(receivedPath, "/path/to/file") || !strings.Contains(receivedPath, "key=value") {
		t.Errorf("Expected path to contain '%s', got '%s'", expectedPath, receivedPath)
	}
}
