package plugin

import (
	"context"
	"net"
	"net/http"
	"strings"

	"helix/internal/auth"
)

type ctxKey int

const claimsKey ctxKey = iota

// StoreClaimsInContext returns a new context carrying the JWT claims.
func StoreClaimsInContext(ctx context.Context, c *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

// ClaimsFrom returns JWT claims stored in the context, or nil.
func ClaimsFrom(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(claimsKey).(*auth.Claims)
	return c
}

// ClientIP extracts the real client IP from X-Forwarded-For, X-Real-IP or RemoteAddr.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}
