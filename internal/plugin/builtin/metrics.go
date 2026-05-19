package builtin

import (
	"net/http"
	"strconv"
	"time"

	"helix/internal/plugin"
	"helix/internal/telemetry"
)

func HTTPMetrics(m *telemetry.Metrics) plugin.Plugin {
	return plugin.New("http-metrics", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := telemetry.NormalizePath(r.URL.Path)
			m.ActiveRequests.Inc()
			start := time.Now()

			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			m.ActiveRequests.Dec()
			m.RequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rec.status)).Inc()
			m.RequestDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
		})
	})
}
