package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

type PathMapping struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type Config struct {
	DefaultHost     string        `yaml:"default_host"`
	PathMappings    []PathMapping `yaml:"path_mappings"`
	FollowRedirects bool          `yaml:"follow_redirects"`
}

type compiledMapping struct {
	regex *regexp.Regexp
	to    string
}

type Proxy struct {
	client           *http.Client
	defaultHost      string
	compiledMappings []compiledMapping
	followRedirects  bool
}

func NewProxy(defaultHost string, mappings []PathMapping, followRedirects bool) (*Proxy, error) {
	var compiled []compiledMapping
	
	for _, mapping := range mappings {
		regex, err := regexp.Compile(mapping.From)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern '%s': %w", mapping.From, err)
		}
		compiled = append(compiled, compiledMapping{
			regex: regex,
			to:    mapping.To,
		})
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Configure redirect behavior
	if !followRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return &Proxy{
		client:           client,
		defaultHost:      defaultHost,
		compiledMappings: compiled,
		followRedirects:  followRedirects,
	}, nil
}

func (p *Proxy) findMapping(path string) (string, bool) {
	for _, mapping := range p.compiledMappings {
		if mapping.regex.MatchString(path) {
			result := mapping.regex.ReplaceAllString(path, mapping.to)
			return result, true
		}
	}
	return "", false
}

func (p *Proxy) buildTargetURL(r *http.Request) (string, error) {
	originalPath := r.URL.Path
	if r.URL.RawQuery != "" {
		originalPath += "?" + r.URL.RawQuery
	}

	// Check if path matches any mapping
	if mappedURL, found := p.findMapping(originalPath); found {
		log.Printf("Path matched mapping: %s -> %s", originalPath, mappedURL)
		return mappedURL, nil
	}

	// Use default host replacement
	if p.defaultHost == "" {
		return "", fmt.Errorf("no default host configured and no mapping matched")
	}

	parsed, err := url.Parse(p.defaultHost)
	if err != nil {
		return "", fmt.Errorf("invalid default host: %w", err)
	}

	targetURL := &url.URL{
		Scheme:   parsed.Scheme,
		Host:     parsed.Host,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	return targetURL.String(), nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Proxying request: %s %s", r.Method, r.URL.String())

	targetURL, err := p.buildTargetURL(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build target URL: %v", err), http.StatusBadGateway)
		log.Printf("Error building target URL: %v", err)
		return
	}

	log.Printf("Target URL: %s", targetURL)

	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		log.Printf("Error creating request: %v", err)
		return
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

	// Send the request
	resp, err := p.client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Failed to reach target server", http.StatusBadGateway)
		log.Printf("Error sending request: %v", err)
		return
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

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func main() {
	// Don't run main in test mode
	if len(os.Args) > 1 && os.Args[1] == "test" {
		return
	}

	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	port := flag.String("port", "", "Port to listen on (defaults to PORT env var or 8080)")
	hostOverride := flag.String("host", "", "Override default host from config")
	followRedirectsFlag := flag.Bool("follow-redirects", false, "Follow redirects automatically")
	flag.Parse()

	// Check environment variables
	if configFileEnv := os.Getenv("CONFIG_FILE"); configFileEnv != "" && *configFile == "config.yaml" {
		*configFile = configFileEnv
	}

	if followRedirectsEnv := os.Getenv("FOLLOW_REDIRECTS"); followRedirectsEnv == "true" && !*followRedirectsFlag {
		*followRedirectsFlag = true
	}

	// Determine port: flag > PORT env var > default 8080
	listenPort := *port
	if listenPort == "" {
		if portEnv := os.Getenv("PORT"); portEnv != "" {
			listenPort = portEnv
		} else {
			listenPort = "8080"
		}
	}

	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Printf("Warning: Failed to load config file: %v", err)
		config = &Config{}
	}

	// Apply host override if provided
	defaultHost := config.DefaultHost
	if *hostOverride != "" {
		defaultHost = *hostOverride
		log.Printf("Using host override: %s", defaultHost)
	}

	// Determine follow redirects setting (command line takes precedence)
	followRedirects := config.FollowRedirects
	if *followRedirectsFlag {
		followRedirects = true
		log.Printf("Following redirects enabled via command line")
	}

	// Create proxy
	proxy, err := NewProxy(defaultHost, config.PathMappings, followRedirects)
	if err != nil {
		log.Fatalf("Failed to create proxy: %v", err)
	}

	// Display configuration
	fmt.Printf("Starting tcz-proxy on port %s\n", listenPort)
	if defaultHost != "" {
		fmt.Printf("Default host: %s\n", defaultHost)
	}
	fmt.Printf("Follow redirects: %v\n", followRedirects)
	if len(config.PathMappings) > 0 {
		fmt.Printf("Path mappings (%d):\n", len(config.PathMappings))
		for i, mapping := range config.PathMappings {
			fmt.Printf("  %d. %s -> %s\n", i+1, mapping.From, mapping.To)
		}
	}
	fmt.Println()

	// Start server
	addr := ":" + listenPort
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, proxy); err != nil {
		log.Fatal(err)
	}
}
