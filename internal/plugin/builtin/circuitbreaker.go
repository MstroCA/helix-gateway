package builtin

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"helix/internal/plugin"
)

type cbState int

const (
	cbClosed   cbState = iota
	cbOpen
	cbHalfOpen
)

type CircuitBreakerConfig struct {
	// Failure rate threshold (0.0–1.0) to trip the breaker.
	Threshold float64
	// Rolling window for failure rate calculation.
	Window time.Duration
	// How long the breaker stays open before attempting half-open.
	Timeout time.Duration
	// Number of requests allowed in half-open state to probe recovery.
	ProbeCount int
}

func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Threshold:  0.5,
		Window:     10 * time.Second,
		Timeout:    30 * time.Second,
		ProbeCount: 3,
	}
}

type circuitBreaker struct {
	cfg      CircuitBreakerConfig
	mu       sync.Mutex
	state    cbState
	failures int
	total    int
	openedAt time.Time
	probes   int
}

func CircuitBreaker(cfg CircuitBreakerConfig) plugin.Plugin {
	cb := &circuitBreaker{cfg: cfg}
	return plugin.New("circuit-breaker", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cb.allow() {
				slog.Warn("circuit breaker: open, rejecting request", "path", r.URL.Path)
				plugin.WriteJSON(w, http.StatusServiceUnavailable, `{"error":"service_unavailable","reason":"circuit_open"}`)
				return
			}

			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			cb.record(rec.status >= 500)
		})
	})
}

func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case cbOpen:
		if time.Since(cb.openedAt) >= cb.cfg.Timeout {
			cb.state = cbHalfOpen
			cb.probes = 0
			return true
		}
		return false
	case cbHalfOpen:
		if cb.probes >= cb.cfg.ProbeCount {
			return false
		}
		cb.probes++
		return true
	default:
		return true
	}
}

func (cb *circuitBreaker) record(failed bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.total++
	if failed {
		cb.failures++
	}

	switch cb.state {
	case cbHalfOpen:
		if failed {
			cb.state = cbOpen
			cb.openedAt = time.Now()
		} else if cb.probes >= cb.cfg.ProbeCount && cb.failures == 0 {
			cb.state = cbClosed
			cb.failures = 0
			cb.total = 0
		}
	case cbClosed:
		if cb.total >= 10 && float64(cb.failures)/float64(cb.total) >= cb.cfg.Threshold {
			cb.state = cbOpen
			cb.openedAt = time.Now()
			slog.Warn("circuit breaker: tripped", "failures", cb.failures, "total", cb.total)
		}
	}
}
