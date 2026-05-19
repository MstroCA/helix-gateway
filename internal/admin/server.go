package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"helix/internal/audit"
	helixmetrics "helix/internal/metrics"
	"helix/internal/plugin"
	"helix/internal/store"
)

type Server struct {
	srv *http.Server
}

func New(port string, st *store.Store, reg *plugin.Registry, tracker *helixmetrics.Tracker, alog *audit.Log) *Server {
	h := &handlers{store: st, reg: reg, tracker: tracker, auditLog: alog}
	mux := http.NewServeMux()
	registerRoutes(mux, h)

	return &Server{
		srv: &http.Server{
			Addr:         ":" + port,
			Handler:      basicAuth(mux),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

func (s *Server) Start() {
	go func() {
		slog.Info("admin api started", "addr", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("admin api error", "err", err)
		}
	}()
}

func (s *Server) Shutdown(ctx context.Context) {
	_ = s.srv.Shutdown(ctx)
}

func basicAuth(next http.Handler) http.Handler {
	user := os.Getenv("HELIX_ADMIN_USER")
	pass := os.Getenv("HELIX_ADMIN_PASSWORD")
	if user == "" {
		user = "admin"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if pass == "" {
			next.ServeHTTP(w, r)
			return
		}
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Helix Admin"`)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
