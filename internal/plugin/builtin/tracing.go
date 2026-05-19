package builtin

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"helix/internal/plugin"
	"helix/internal/telemetry"
)

func Tracing(tracer trace.Tracer) plugin.Plugin {
	return plugin.New("tracing", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			spanName := r.Method + " " + telemetry.NormalizePath(r.URL.Path)
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					semconv.URLPathKey.String(r.URL.Path),
					semconv.ClientAddressKey.String(plugin.ClientIP(r)),
				),
			)
			defer span.End()

			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r.WithContext(ctx))

			span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(rec.status))

			if claims := plugin.ClaimsFrom(ctx); claims != nil {
				span.SetAttributes(
					attribute.String("user.id", claims.UserID()),
					attribute.String("user.role", claims.Role),
				)
			}

			if rec.status >= 500 {
				span.SetStatus(codes.Error, http.StatusText(rec.status))
			}
		})
	})
}
