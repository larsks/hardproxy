package main

import (
	"log"
	"os"
	"strings"

	"hardproxy/proxy"

	"github.com/spf13/pflag"
)

func main() {
	var listen string
	var cacheDir string

	pflag.StringVarP(&listen, "listen", "l", "127.0.0.1:8888", "Listen address (address:port or :port)")
	pflag.StringVarP(&cacheDir, "cache-directory", "d", "", "Cache directory path")
	pflag.Parse()

	// Handle environment variable for cache directory
	if cacheDir == "" {
		if envCacheDir := os.Getenv("CACHE_DIR"); envCacheDir != "" {
			cacheDir = envCacheDir
		} else {
			cacheDir = ".cache"
		}
	}

	// Parse listen address - if it starts with :, prepend 0.0.0.0
	if strings.HasPrefix(listen, ":") {
		listen = "0.0.0.0" + listen
	}

	log.Printf("Starting hardproxy on %s", listen)
	log.Printf("Cache directory: %s", cacheDir)

	p, err := proxy.NewProxy(cacheDir)
	if err != nil {
		log.Fatalf("Failed to create proxy: %v", err)
	}

	if err := p.Start(listen); err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}
}
