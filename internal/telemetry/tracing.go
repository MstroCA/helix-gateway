package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const TracerName = "helix"

// InitTracer connects to the OTLP collector and configures the global tracer.
// Returns a no-op tracer when endpoint is empty (safe for local dev without a collector).
func InitTracer(ctx context.Context, endpoint string) (trace.Tracer, func(context.Context), error) {
	if endpoint == "" {
		noop := trace.NewNoopTracerProvider()
		otel.SetTracerProvider(noop)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		return noop.Tracer(TracerName), func(context.Context) {}, nil
	}

	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("otlp exporter: %w", err)
	}

	res, _ := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String("helix-gateway")),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	// W3C TraceContext + Baggage — compatible with Spring Boot OTel auto-instrumentation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := func(ctx context.Context) { _ = tp.Shutdown(ctx) }
	return tp.Tracer(TracerName), shutdown, nil
}
