package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"helix/internal/plugin"
)

func New(backendURL string) (http.Handler, error) {
	target, err := url.Parse(backendURL)
	if err != nil {
		return nil, fmt.Errorf("invalid backend URL: %w", err)
	}

	wsp := newWebSocketProxy(target)

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.FlushInterval = -1

	rp.Transport = &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	rp.Director = func(r *http.Request) {
		r.URL.Scheme = target.Scheme
		r.URL.Host = target.Host
		r.URL.Path = stripAPIPrefix(r.URL.Path)
		if r.URL.RawPath != "" {
			r.URL.RawPath = stripAPIPrefix(r.URL.RawPath)
		}
		r.Host = target.Host

		otel.GetTextMapPropagator().Inject(r.Context(), propagation.HeaderCarrier(r.Header))

		r.Header.Del("X-Forwarded-For")
		r.Header.Set("X-Forwarded-For", plugin.ClientIP(r))
		r.Header.Set("X-Forwarded-Proto", forwardedProto(r))

		if claims := plugin.ClaimsFrom(r.Context()); claims != nil {
			r.Header.Set("X-Gateway-User-Id", claims.UserID())
			r.Header.Set("X-Gateway-User-Role", claims.Role)
		}

		if isWebSocketUpgrade(r) {
			r.Header.Set("Connection", "Upgrade")
			r.Header.Set("Upgrade", "websocket")
		}
	}

	rp.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Access-Control-Allow-Origin")
		resp.Header.Del("Access-Control-Allow-Credentials")
		resp.Header.Del("Access-Control-Allow-Methods")
		resp.Header.Del("Access-Control-Allow-Headers")
		resp.Header.Del("Access-Control-Max-Age")
		resp.Header.Del("Vary")
		return nil
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("proxy error", "path", r.URL.Path, "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream_unavailable"}`))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWebSocketUpgrade(r) {
			wsp.ServeHTTP(w, r)
			return
		}
		rp.ServeHTTP(w, r)
	}), nil
}

func stripAPIPrefix(path string) string {
	stripped := strings.TrimPrefix(path, "/api")
	if stripped == "" {
		return "/"
	}
	return stripped
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func forwardedProto(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
