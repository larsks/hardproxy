package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGetCacheFilename(t *testing.T) {
	cacheDir := "/tmp/cache"
	url := "http://example.com/test"

	filename := GetCacheFilename(cacheDir, url)

	// SHA1 of "http://example.com/test" should be consistent
	// Hash is 03ffbabecf0d8b96c34415a8746f036f50f06edd
	// Structure should be: 0/3/03ffbabecf0d8b96c34415a8746f036f50f06edd
	expected := filepath.Join(cacheDir, "0", "3", "03ffbabecf0d8b96c34415a8746f036f50f06edd")
	if filename != expected {
		t.Errorf("Expected %s, got %s", expected, filename)
	}
}

func TestGetCacheFilenameDifferentURLs(t *testing.T) {
	cacheDir := "/tmp/cache"
	url1 := "http://example.com/test1"
	url2 := "http://example.com/test2"

	filename1 := GetCacheFilename(cacheDir, url1)
	filename2 := GetCacheFilename(cacheDir, url2)

	if filename1 == filename2 {
		t.Error("Different URLs should have different cache filenames")
	}
}

func TestProxyCreation(t *testing.T) {
	tmpDir := t.TempDir()

	p, err := NewProxy(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	if p.cacheDir != tmpDir {
		t.Errorf("Expected cache dir %s, got %s", tmpDir, p.cacheDir)
	}

	// Verify directory was created
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("Cache directory was not created")
	}
}

func TestProxyCacheMiss(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	defer testServer.Close()

	p, err := NewProxy(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Create a request through the proxy
	req := httptest.NewRequest("GET", testServer.URL, nil)
	w := httptest.NewRecorder()

	p.handleRequest(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if w.Body.String() != "test response" {
		t.Errorf("Expected 'test response', got '%s'", w.Body.String())
	}

	// Verify X-Cache header
	if resp.Header.Get("X-Cache") != "MISS" {
		t.Errorf("Expected X-Cache: MISS, got %s", resp.Header.Get("X-Cache"))
	}

	// Verify file was cached
	cacheFile := p.getCacheFilename(testServer.URL)
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Error("Response was not cached")
	}
}

func TestProxyCacheHit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test server
	callCount := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	defer testServer.Close()

	p, err := NewProxy(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// First request - should be a cache miss
	req1 := httptest.NewRequest("GET", testServer.URL, nil)
	w1 := httptest.NewRecorder()
	p.handleRequest(w1, req1)

	if callCount != 1 {
		t.Errorf("Expected 1 call to test server, got %d", callCount)
	}

	// Second request - should be a cache hit
	req2 := httptest.NewRequest("GET", testServer.URL, nil)
	w2 := httptest.NewRecorder()
	p.handleRequest(w2, req2)

	// Server should still only have been called once
	if callCount != 1 {
		t.Errorf("Expected 1 call to test server (cached), got %d", callCount)
	}

	resp := w2.Result()
	if resp.Header.Get("X-Cache") != "HIT" {
		t.Errorf("Expected X-Cache: HIT, got %s", resp.Header.Get("X-Cache"))
	}

	if w2.Body.String() != "test response" {
		t.Errorf("Expected 'test response', got '%s'", w2.Body.String())
	}
}

func TestProxyFollowsRedirects(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test server that redirects
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("final destination"))
	}))
	defer finalServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusMovedPermanently)
	}))
	defer redirectServer.Close()

	p, err := NewProxy(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Request the redirect URL
	req := httptest.NewRequest("GET", redirectServer.URL, nil)
	w := httptest.NewRecorder()

	p.handleRequest(w, req)

	// Should receive the final response, not the redirect
	if w.Body.String() != "final destination" {
		t.Errorf("Expected 'final destination', got '%s'", w.Body.String())
	}

	// Should not get a redirect status code
	resp := w.Result()
	if resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusFound {
		t.Errorf("Proxy should follow redirects, not pass them through. Got status %d", resp.StatusCode)
	}
}

func TestProxyRealURLNoRedirect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real URL test in short mode")
	}

	tmpDir := t.TempDir()

	p, err := NewProxy(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Request a real URL with no redirects
	url := "http://mirror.csclub.uwaterloo.ca/gutenberg/"
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()

	p.handleRequest(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify X-Cache header is MISS on first request
	if resp.Header.Get("X-Cache") != "MISS" {
		t.Errorf("Expected X-Cache: MISS, got %s", resp.Header.Get("X-Cache"))
	}

	// Second request should be cached
	req2 := httptest.NewRequest("GET", url, nil)
	w2 := httptest.NewRecorder()
	p.handleRequest(w2, req2)

	resp2 := w2.Result()
	if resp2.Header.Get("X-Cache") != "HIT" {
		t.Errorf("Expected X-Cache: HIT on second request, got %s", resp2.Header.Get("X-Cache"))
	}
}

func TestProxyRealURLWithRedirect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real URL test in short mode")
	}

	tmpDir := t.TempDir()

	p, err := NewProxy(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Request a real URL that has a redirect
	url := "http://lobste.rs"
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()

	p.handleRequest(w, req)

	resp := w.Result()
	// Should get final response, not a redirect
	if resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect {
		t.Errorf("Proxy should follow redirects, not pass them through. Got status %d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		t.Logf("Got status %d, expected 200 (redirect should be followed)", resp.StatusCode)
	}
}

func TestProxyErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()

	p, err := NewProxy(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Request an invalid URL
	req := httptest.NewRequest("GET", "http://this-domain-should-not-exist-12345.com", nil)
	w := httptest.NewRecorder()

	p.handleRequest(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("Expected status %d for failed request, got %d", http.StatusBadGateway, resp.StatusCode)
	}
}
