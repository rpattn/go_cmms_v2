// internal/auth/cache.go

package auth

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type cachingRoundTripper struct {
	cacheDir string
	rt       http.RoundTripper
	mu       sync.Mutex
}

// Used to match OIDC well-known discovery URLs
func isDiscoveryURL(path string) bool {
	return strings.HasSuffix(path, "/.well-known/openid-configuration")
}

// RoundTrip intercepts requests to OIDC discovery URLs and caches them persistently
func (c *cachingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if isDiscoveryURL(req.URL.Path) {
		cacheFile := filepath.Join(c.cacheDir, req.URL.Host+".json")

		// Try to serve from cache
		if data, err := os.ReadFile(cacheFile); err == nil {
			log.Printf("[OIDC Cache] HIT: Loaded from cache: %s", cacheFile)

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(data)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}

		// Cache miss
		log.Printf("[OIDC Cache] MISS: Fetching from network: %s", req.URL.String())
		return c.fetchAndCache(req, cacheFile)
	}

	// Not OIDC — normal HTTP
	return c.rt.RoundTrip(req)
}

// ForceRefresh forces a fetch and update of the given discovery URL
func (c *cachingRoundTripper) ForceRefresh(url string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	cacheFile := filepath.Join(c.cacheDir, req.URL.Host+".json")
	log.Printf("[OIDC Cache] REFRESH: Fetching fresh copy: %s", url)

	_, err = c.fetchAndCache(req, cacheFile)
	return err
}

func (c *cachingRoundTripper) fetchAndCache(req *http.Request, cacheFile string) (*http.Response, error) {
	resp, err := c.rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	_ = os.MkdirAll(c.cacheDir, 0755)
	if writeErr := os.WriteFile(cacheFile, bodyBytes, 0644); writeErr != nil {
		log.Printf("[OIDC Cache] ERROR: Failed to write cache file: %s: %v", cacheFile, writeErr)
	} else {
		log.Printf("[OIDC Cache] SAVED: Refreshed cache: %s", cacheFile)
	}

	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return resp, nil
}
