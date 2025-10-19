package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Proxy struct {
	client  *http.Client
	mirrors []string
}

func NewProxy(mirrors []string) *Proxy {
	return &Proxy{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		},
		mirrors: mirrors,
	}
}

func (p *Proxy) tryRequest(targetURL string, r *http.Request) (*http.Response, error) {
	proxyReq, err := http.NewRequest(r.Method, targetURL, nil)
	if err != nil {
		return nil, err
	}

	// Copy headers from original request
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Add X-Forwarded-For header
	if clientIP := r.RemoteAddr; clientIP != "" {
		proxyReq.Header.Set("X-Forwarded-For", clientIP)
	}

	return p.client.Do(proxyReq)
}

func (p *Proxy) replaceHost(originalURL, newHost string) (string, error) {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		return "", err
	}

	mirrorParsed, err := url.Parse(newHost)
	if err != nil {
		return "", err
	}

	parsed.Scheme = mirrorParsed.Scheme
	parsed.Host = mirrorParsed.Host

	return parsed.String(), nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Proxying request: %s %s", r.Method, r.URL.String())

	// Create target URL
	targetURL := r.URL.String()
	if r.URL.Scheme == "" {
		targetURL = "http://" + r.Host + r.URL.Path
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}
	}

	// Try original request
	resp, err := p.tryRequest(targetURL, r)
	if err != nil {
		http.Error(w, "Failed to reach target server", http.StatusBadGateway)
		log.Printf("Error sending request: %v", err)
		return
	}

	// If we got a 404 and have mirrors, try them
	if resp.StatusCode == http.StatusNotFound && len(p.mirrors) > 0 {
		resp.Body.Close()
		log.Printf("Received 404, trying mirrors...")

		for i, mirror := range p.mirrors {
			mirrorURL, err := p.replaceHost(targetURL, mirror)
			if err != nil {
				log.Printf("Failed to create mirror URL for %s: %v", mirror, err)
				continue
			}

			log.Printf("Trying mirror %d/%d: %s", i+1, len(p.mirrors), mirrorURL)
			mirrorResp, err := p.tryRequest(mirrorURL, r)
			if err != nil {
				log.Printf("Mirror %s failed: %v", mirror, err)
				continue
			}

			// If we got something other than 404, use this response
			if mirrorResp.StatusCode != http.StatusNotFound {
				log.Printf("Mirror %s succeeded with status %d", mirror, mirrorResp.StatusCode)
				resp = mirrorResp
				break
			}

			mirrorResp.Body.Close()
			log.Printf("Mirror %s also returned 404", mirror)
		}
	}

	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response body: %v", err)
	}

	log.Printf("Completed: %s %s - Status: %d", r.Method, targetURL, resp.StatusCode)
}

func main() {
	// Define mirror servers - add your mirrors here
	mirrors := []string{
		"https://mirror1.example.com",
		"https://mirror2.example.com",
		"http://backup.example.com",
	}

	// Filter out empty mirrors
	var activeMirrors []string
	for _, m := range mirrors {
		if strings.TrimSpace(m) != "" {
			activeMirrors = append(activeMirrors, m)
		}
	}

	proxy := NewProxy(activeMirrors)

	port := ":8080"
	fmt.Printf("Starting HTTP proxy server on http://localhost%s\n", port)
	if len(activeMirrors) > 0 {
		fmt.Printf("Configured mirrors (%d):\n", len(activeMirrors))
		for i, m := range activeMirrors {
			fmt.Printf("  %d. %s\n", i+1, m)
		}
	} else {
		fmt.Println("No mirrors configured - running as simple proxy")
	}
	fmt.Println("\nExample usage: curl -x http://localhost:8080 http://example.com")

	if err := http.ListenAndServe(port, proxy); err != nil {
		log.Fatal(err)
	}
}
