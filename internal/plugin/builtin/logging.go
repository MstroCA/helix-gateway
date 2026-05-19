package builtin

import (
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"

	"helix/internal/plugin"
)

func Logging() plugin.Plugin {
	return plugin.New("logging", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			ctx := r.Context()
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"size_bytes", rec.written,
				"ip", plugin.ClientIP(r),
				"correlation_id", r.Header.Get("X-Correlation-ID"),
			}

			if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
				attrs = append(attrs,
					"trace_id", sc.TraceID().String(),
					"span_id", sc.SpanID().String(),
				)
			}

			switch {
			case rec.status >= 500:
				slog.ErrorContext(ctx, "request", attrs...)
			case rec.status >= 400:
				slog.WarnContext(ctx, "request", attrs...)
			default:
				slog.InfoContext(ctx, "request", attrs...)
			}
		})
	})
}
