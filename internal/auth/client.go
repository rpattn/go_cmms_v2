// internal/auth/client.go

package auth

import "net/http"

func NewCachingHTTPClient(cacheDir string) *http.Client {
	transport := &cachingRoundTripper{
		cacheDir: cacheDir,
		rt:       http.DefaultTransport,
	}
	return &http.Client{Transport: transport}
}

// Optional: expose transport for refresh
func NewCachingTransport(cacheDir string) *cachingRoundTripper {
	return &cachingRoundTripper{
		cacheDir: cacheDir,
		rt:       http.DefaultTransport,
	}
}
