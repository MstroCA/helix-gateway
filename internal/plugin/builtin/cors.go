package builtin

import (
	"log/slog"
	"net/http"
	"strings"

	"helix/internal/plugin"
)

func CORS(allowedOrigins []string) plugin.Plugin {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[strings.TrimSpace(o)] = struct{}{}
	}

	set := func(w http.ResponseWriter, origin string) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-KEY, X-TENANT, X-Auth-Mode, X-CSRF-Token")
		h.Set("Access-Control-Max-Age", "3600")
		h.Set("Vary", "Origin")
	}

	return plugin.New("cors", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := allowed[origin]; !ok {
				slog.Warn("cors: rejected origin", "origin", origin, "path", r.URL.Path)
				w.WriteHeader(http.StatusForbidden)
				return
			}
			set(w, origin)
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
}
