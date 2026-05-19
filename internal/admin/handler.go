package admin

import (
	"net/http"
	"runtime"
	"time"

	"helix/internal/audit"
	helixmetrics "helix/internal/metrics"
	"helix/internal/plugin"
	"helix/internal/store"
)

type handlers struct {
	store    *store.Store
	reg      *plugin.Registry
	tracker  *helixmetrics.Tracker
	auditLog *audit.Log
}

var startTime = time.Now()

func registerRoutes(mux *http.ServeMux, h *handlers) {
	mux.HandleFunc("GET /admin/health", handleHealth)
	mux.HandleFunc("GET /admin/v1/info", handleInfo)
	mux.HandleFunc("GET /admin/v1/plugins", h.handlePlugins)
	mux.HandleFunc("GET /admin/v1/config/export", h.handleConfigExport)
	mux.HandleFunc("GET /admin/v1/metrics", h.handleMetrics)
	mux.HandleFunc("GET /admin/v1/metrics/stream", h.streamMetrics)

	mux.HandleFunc("GET /admin/v1/routes", h.listRoutes)
	mux.HandleFunc("POST /admin/v1/routes", h.createRoute)
	mux.HandleFunc("GET /admin/v1/routes/{id}", h.getRoute)
	mux.HandleFunc("PUT /admin/v1/routes/{id}", h.updateRoute)
	mux.HandleFunc("DELETE /admin/v1/routes/{id}", h.deleteRoute)
	mux.HandleFunc("PATCH /admin/v1/routes/{id}/toggle", h.toggleRoute)

	mux.HandleFunc("GET /admin/v1/upstreams", h.listUpstreams)
	mux.HandleFunc("POST /admin/v1/upstreams", h.createUpstream)
	mux.HandleFunc("GET /admin/v1/upstreams/{id}", h.getUpstream)
	mux.HandleFunc("PUT /admin/v1/upstreams/{id}", h.updateUpstream)
	mux.HandleFunc("DELETE /admin/v1/upstreams/{id}", h.deleteUpstream)
	mux.HandleFunc("GET /admin/v1/upstreams/{id}/health", h.upstreamHealth)

	mux.HandleFunc("GET /admin/v1/policies", h.listPolicies)
	mux.HandleFunc("POST /admin/v1/policies", h.createPolicy)
	mux.HandleFunc("GET /admin/v1/policies/{id}", h.getPolicy)
	mux.HandleFunc("PUT /admin/v1/policies/{id}", h.updatePolicy)
	mux.HandleFunc("DELETE /admin/v1/policies/{id}", h.deletePolicy)
	mux.HandleFunc("PATCH /admin/v1/policies/{id}/toggle", h.togglePolicy)

	mux.HandleFunc("GET /admin/v1/keys", h.listKeys)
	mux.HandleFunc("POST /admin/v1/keys", h.createKey)
	mux.HandleFunc("GET /admin/v1/keys/{id}", h.getKey)
	mux.HandleFunc("DELETE /admin/v1/keys/{id}", h.deleteKey)
	mux.HandleFunc("PATCH /admin/v1/keys/{id}/toggle", h.toggleKey)

	mux.HandleFunc("GET /admin/v1/audit", h.handleAuditLog)

	serveUI(mux)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":    "Helix Gateway",
		"version": "0.1.0",
		"uptime":  time.Since(startTime).String(),
		"go":      runtime.Version(),
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	})
}

func (h *handlers) handlePlugins(w http.ResponseWriter, _ *http.Request) {
	global := []string{
		"correlation-id", "tracing", "logging", "http-metrics",
		"security-headers", "cors", "request-security", "ip-policy",
		"auth-rate-limit", "ip-rate-limit", "jwt-auth", "user-rate-limit",
		"circuit-breaker",
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"global":    global,
		"available": h.reg.Names(),
	})
}

func (h *handlers) handleConfigExport(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.store.Export())
}
