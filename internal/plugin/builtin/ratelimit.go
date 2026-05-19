package builtin

import (
	"net/http"
	"strconv"

	"helix/internal/plugin"
	"helix/internal/ratelimit"
	"helix/internal/telemetry"
)

func IPRateLimit(l *ratelimit.Limiter, m *telemetry.Metrics, bypassPrefixes []string) plugin.Plugin {
	return plugin.New("ip-rate-limit", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions || isPublic(r.URL.Path, bypassPrefixes) {
				next.ServeHTTP(w, r)
				return
			}
			if !l.AllowIP(r.Context(), plugin.ClientIP(r)) {
				m.RateLimitRejects.WithLabelValues("ip").Inc()
				w.Header().Set("Retry-After", "5")
				plugin.WriteJSON(w, http.StatusTooManyRequests, `{"error":"rate_limit_exceeded","scope":"ip"}`)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
}

func AuthRateLimit(l *ratelimit.Limiter, m *telemetry.Metrics) plugin.Plugin {
	return plugin.New("auth-rate-limit", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/api/auth/login" {
				if !l.AllowAuth(r.Context(), plugin.ClientIP(r)) {
					m.RateLimitRejects.WithLabelValues("auth").Inc()
					w.Header().Set("Retry-After", "60")
					plugin.WriteJSON(w, http.StatusTooManyRequests, `{"error":"rate_limit_exceeded","scope":"auth"}`)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	})
}

func UserRateLimit(l *ratelimit.Limiter, m *telemetry.Metrics) plugin.Plugin {
	return plugin.New("user-rate-limit", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := plugin.ClaimsFrom(r.Context())
			if claims == nil {
				next.ServeHTTP(w, r)
				return
			}
			if !l.AllowUser(r.Context(), claims.UserID()) {
				m.RateLimitRejects.WithLabelValues("user").Inc()
				w.Header().Set("Retry-After", strconv.Itoa(5))
				plugin.WriteJSON(w, http.StatusTooManyRequests, `{"error":"rate_limit_exceeded","scope":"user"}`)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
}
