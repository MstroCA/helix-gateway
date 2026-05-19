package telemetry

import (
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus collectors for the gateway.
type Metrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RateLimitRejects *prometheus.CounterVec
	ActiveRequests   prometheus.Gauge
}

// NewMetrics registers all collectors into reg and returns the struct.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_http_requests_total",
			Help: "Total HTTP requests processed by the gateway.",
		}, []string{"method", "path", "status"}),

		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gateway_http_request_duration_seconds",
			Help:    "HTTP request latency distribution.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		}, []string{"method", "path"}),

		RateLimitRejects: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_rate_limit_rejected_total",
			Help: "Total requests rejected by the rate limiter.",
		}, []string{"scope"}), // "ip" | "user"

		ActiveRequests: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_active_requests",
			Help: "Number of requests currently in flight.",
		}),
	}

	reg.MustRegister(
		m.RequestsTotal,
		m.RequestDuration,
		m.RateLimitRejects,
		m.ActiveRequests,
	)
	return m
}

var (
	numericSeg = regexp.MustCompile(`^\d+$`)
	uuidSeg    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// NormalizePath replaces dynamic segments (IDs, UUIDs, long tokens) to keep
// Prometheus label cardinality bounded.
//
//	/api/fault-records/123/photos/456  →  /api/fault-records/:id/photos/:id
//	/api/auth/validate-reset/<token>   →  /api/auth/validate-reset/:token
func NormalizePath(path string) string {
	parts := strings.Split(path, "/")
	for i, seg := range parts {
		switch {
		case numericSeg.MatchString(seg):
			parts[i] = ":id"
		case uuidSeg.MatchString(seg):
			parts[i] = ":id"
		case len(seg) > 24 && !strings.Contains(seg, "-"):
			// Long opaque token (e.g. JWT-style reset/setup tokens)
			parts[i] = ":token"
		}
	}
	return strings.Join(parts, "/")
}
