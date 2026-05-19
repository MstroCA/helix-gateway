package health

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"helix/internal/store"
)

// State holds the health status of a single endpoint URL.
type State struct {
	Healthy   bool      `json:"healthy"`
	LastCheck time.Time `json:"lastCheck"`
	Error     string    `json:"error,omitempty"`
}

// Checker probes upstream endpoints in the background and caches their health states.
// Unknown endpoints are treated as healthy (fail-open).
type Checker struct {
	mu     sync.RWMutex
	states map[string]*State // endpoint URL → state
	hc     *http.Client
}

func New() *Checker {
	return &Checker{
		states: make(map[string]*State),
		hc: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Start launches background health probes against all upstreams in the store.
// Probes run every interval and stop when ctx is cancelled.
func (c *Checker) Start(ctx context.Context, st *store.Store, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		c.probeAll(st)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.probeAll(st)
			}
		}
	}()
}

func (c *Checker) probeAll(st *store.Store) {
	upstreams := st.ListUpstreams()
	for _, u := range upstreams {
		urls := upstreamURLs(u)
		for _, url := range urls {
			if u.HealthPath == "" {
				c.markHealthy(url)
				continue
			}
			probeURL := url + u.HealthPath
			err := c.probe(probeURL)
			if err != nil {
				slog.Debug("upstream unhealthy", "url", url, "err", err)
				c.markUnhealthy(url, err.Error())
			} else {
				c.markHealthy(url)
			}
		}
	}
}

func (c *Checker) probe(url string) error {
	resp, err := c.hc.Get(url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func (c *Checker) markHealthy(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states[url] = &State{Healthy: true, LastCheck: time.Now().UTC()}
}

func (c *Checker) markUnhealthy(url, errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states[url] = &State{Healthy: false, LastCheck: time.Now().UTC(), Error: errMsg}
}

// IsHealthy returns true if the endpoint is healthy or has not been probed yet.
func (c *Checker) IsHealthy(url string) bool {
	c.mu.RLock()
	s, ok := c.states[url]
	c.mu.RUnlock()
	return !ok || s.Healthy
}

// States returns a snapshot of all known health states.
func (c *Checker) States() map[string]State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]State, len(c.states))
	for k, v := range c.states {
		out[k] = *v
	}
	return out
}

func upstreamURLs(u *store.Upstream) []string {
	if len(u.Endpoints) > 0 {
		return u.Endpoints
	}
	if u.URL != "" {
		return []string{u.URL}
	}
	return nil
}
