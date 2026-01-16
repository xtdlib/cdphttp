package cdphttp

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type roundTripper struct {
	base      http.RoundTripper
	client    *client
	refreshMu sync.Mutex
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Try to refresh cookies if cache is stale
	rt.refreshMu.Lock()
	if !rt.client.CacheValid() {
		if err := rt.client.RefreshCookies(ctx); err != nil {
			rt.refreshMu.Unlock()
			return nil, err
		}
	}
	rt.refreshMu.Unlock()

	// Set user agent if available
	if ua := rt.client.UserAgent(); ua != "" {
		req.Header.Set("User-Agent", ua)
	}

	return rt.base.RoundTrip(req)
}

// NewClient creates an http.Client that injects Chrome cookies.
// This function always succeeds - Chrome connection happens lazily on first request.
// Errors are only returned from requests if Chrome is unavailable AND cache is expired.
func NewClient(debugURL string) *http.Client {
	return newClientWithOptions(debugURL, 5*time.Minute)
}

// newClientWithOptions creates an http.Client with custom cache TTL.
func newClientWithOptions(debugURL string, cacheTTL time.Duration) *http.Client {
	c := newClient(debugURL, cacheTTL)

	return &http.Client{
		Jar: c.Jar,
		Transport: &roundTripper{
			base:   http.DefaultTransport,
			client: c,
		},
	}
}
