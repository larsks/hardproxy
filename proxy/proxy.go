package proxy

import (
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type Proxy struct {
	cacheDir string
	client   *http.Client
}

func NewProxy(cacheDir string) (*Proxy, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create HTTP client that follows redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects (default Go behavior)
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	return &Proxy{
		cacheDir: cacheDir,
		client:   client,
	}, nil
}

func (p *Proxy) Start(listen string) error {
	http.HandleFunc("/", p.handleRequest)
	log.Printf("Proxy listening on %s", listen)
	return http.ListenAndServe(listen, nil)
}

func (p *Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Get the target URL from the request
	targetURL := r.URL.String()
	if r.URL.Scheme == "" {
		// If no scheme, try to construct from Host header
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		targetURL = fmt.Sprintf("%s://%s%s", scheme, r.Host, r.URL.Path)
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}
	}

	log.Printf("Request for: %s", targetURL)

	// Check cache first
	cacheFile := p.getCacheFilename(targetURL)
	if content, err := os.ReadFile(cacheFile); err == nil {
		log.Printf("Cache hit: %s", targetURL)
		w.Header().Set("X-Cache", "HIT")
		w.Write(content)
		return
	}

	// Cache miss - fetch from remote
	log.Printf("Cache miss: %s", targetURL)
	resp, err := p.client.Get(targetURL)
	if err != nil {
		log.Printf("Error fetching %s: %v", targetURL, err)
		http.Error(w, fmt.Sprintf("Error fetching URL: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	// Store in cache
	// Create directory structure for the cache file
	cacheDir := filepath.Dir(cacheFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("Warning: failed to create cache directory: %v", err)
	}
	if err := os.WriteFile(cacheFile, body, 0644); err != nil {
		log.Printf("Warning: failed to write cache file: %v", err)
	}

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Set("X-Cache", "MISS")

	// Write status code and body
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func (p *Proxy) getCacheFilename(url string) string {
	hash := sha1.Sum([]byte(url))
	hashStr := fmt.Sprintf("%x", hash)
	// Create hierarchical structure: a/c/ac4cbe16220c61319d192bf9078f01de42e383e3
	return filepath.Join(p.cacheDir, hashStr[0:1], hashStr[1:2], hashStr)
}

func GetCacheFilename(cacheDir, url string) string {
	hash := sha1.Sum([]byte(url))
	hashStr := fmt.Sprintf("%x", hash)
	// Create hierarchical structure: a/c/ac4cbe16220c61319d192bf9078f01de42e383e3
	return filepath.Join(cacheDir, hashStr[0:1], hashStr[1:2], hashStr)
}
