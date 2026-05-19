package builtin

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"helix/internal/plugin"
)

// RouteRateLimit creates a per-client-IP token bucket rate limiter for a single route.
// Unlike the global ip-rate-limit plugin, this one is configured per-route via PluginRef config.
func RouteRateLimit(rps, burst int) plugin.Plugin {
	type entry struct {
		lim      *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*entry)
	)

	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			mu.Lock()
			for k, v := range clients {
				if time.Since(v.lastSeen) > 10*time.Minute {
					delete(clients, k)
				}
			}
			mu.Unlock()
		}
	}()

	getLimiter := func(key string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		e, ok := clients[key]
		if !ok {
			e = &entry{lim: rate.NewLimiter(rate.Limit(rps), burst)}
			clients[key] = e
		}
		e.lastSeen = time.Now()
		return e.lim
	}

	return plugin.New("route-rate-limit", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !getLimiter(plugin.ClientIP(r)).Allow() {
				plugin.WriteJSON(w, http.StatusTooManyRequests, `{"error":"route_rate_limit_exceeded"}`)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
}
