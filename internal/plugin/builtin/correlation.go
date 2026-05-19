package builtin

import (
	"net/http"

	"github.com/google/uuid"
	"helix/internal/plugin"
)

func CorrelationID() plugin.Plugin {
	return plugin.New("correlation-id", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Correlation-ID")
			if id == "" {
				id = uuid.NewString()
			}
			r.Header.Set("X-Correlation-ID", id)
			w.Header().Set("X-Correlation-ID", id)
			next.ServeHTTP(w, r)
		})
	})
}
