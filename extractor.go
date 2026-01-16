package cdphttp

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"
)

type client struct {
	Jar *cookiejar.Jar

	mu        sync.RWMutex
	cdpClient *cdpClient
	debugURL  string
	userAgent string

	lastRefresh time.Time
	cacheTTL    time.Duration
}

// connect attempts to connect to Chrome, returns error if connection fails
func (c *client) connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Already connected
	if c.cdpClient != nil {
		return nil
	}

	cdpClient, err := createCDPClient(ctx, c.debugURL)
	if err != nil {
		return err
	}

	c.cdpClient = cdpClient
	return nil
}

// disconnect closes the CDP connection
func (c *client) disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cdpClient != nil {
		c.cdpClient.Close()
		c.cdpClient = nil
	}
}

// ensureConnection attempts to connect if not already connected
// Returns the current CDP client or nil if not connected
func (c *client) ensureConnection(ctx context.Context) *cdpClient {
	c.mu.RLock()
	if c.cdpClient != nil {
		defer c.mu.RUnlock()
		return c.cdpClient
	}
	c.mu.RUnlock()

	// Try to connect
	if err := c.connect(ctx); err != nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cdpClient
}

// RefreshCookies fetches fresh cookies from Chrome
// Returns error only if Chrome is unavailable AND cache is expired
func (c *client) RefreshCookies(ctx context.Context) error {
	cdpClient := c.ensureConnection(ctx)
	if cdpClient == nil {
		// Check if cache is still valid
		c.mu.RLock()
		cacheValid := time.Since(c.lastRefresh) < c.cacheTTL
		c.mu.RUnlock()

		if cacheValid {
			return nil // Use cached cookies
		}
		return ErrChromeUnavailable
	}

	cookies, err := cdpClient.fetchCookies(ctx)
	if err != nil {
		// Connection might be stale, try to reconnect
		c.disconnect()
		cdpClient = c.ensureConnection(ctx)
		if cdpClient == nil {
			c.mu.RLock()
			cacheValid := time.Since(c.lastRefresh) < c.cacheTTL
			c.mu.RUnlock()
			if cacheValid {
				return nil
			}
			return ErrChromeUnavailable
		}

		cookies, err = cdpClient.fetchCookies(ctx)
		if err != nil {
			c.disconnect()
			c.mu.RLock()
			cacheValid := time.Since(c.lastRefresh) < c.cacheTTL
			c.mu.RUnlock()
			if cacheValid {
				return nil
			}
			return err
		}
	}

	// Update user agent if not set
	c.mu.RLock()
	hasUserAgent := c.userAgent != ""
	c.mu.RUnlock()

	if !hasUserAgent {
		userAgent, err := cdpClient.fetchUserAgent(ctx)
		if err == nil {
			c.mu.Lock()
			c.userAgent = userAgent
			c.mu.Unlock()
		}
	}

	// Update cookies in jar
	for _, cookie := range cookies {
		c.Jar.SetCookies(&url.URL{
			Scheme: "https",
			Host:   cookie.Domain,
			Path:   cookie.Path,
		}, []*http.Cookie{
			{
				Name:     cookie.Name,
				Value:    cookie.Value,
				Path:     cookie.Path,
				Domain:   cookie.Domain,
				Secure:   cookie.Secure,
				HttpOnly: cookie.HTTPOnly,
			},
		})
	}

	c.mu.Lock()
	c.lastRefresh = time.Now()
	c.mu.Unlock()

	return nil
}

// UserAgent returns the current user agent (may be empty if Chrome never connected)
func (c *client) UserAgent() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userAgent
}

// CacheValid returns true if the cookie cache is still valid
func (c *client) CacheValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.lastRefresh.IsZero() && time.Since(c.lastRefresh) < c.cacheTTL
}

// Close closes the CDP connection
func (c *client) Close() error {
	c.disconnect()
	return nil
}

// newClient creates a new Client (internal)
func newClient(debugURL string, cacheTTL time.Duration) *client {
	if debugURL == "" {
		debugURL = "ws://localhost:9222"
	}
	if cacheTTL == 0 {
		cacheTTL = 5 * time.Minute
	}

	jar, _ := cookiejar.New(nil)

	return &client{
		debugURL: debugURL,
		Jar:      jar,
		cacheTTL: cacheTTL,
	}
}
