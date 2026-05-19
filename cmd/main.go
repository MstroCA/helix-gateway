package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"helix/internal/admin"
	"helix/internal/audit"
	"helix/internal/auth"
	"helix/internal/config"
	"helix/internal/health"
	helixmetrics "helix/internal/metrics"
	"helix/internal/plugin"
	"helix/internal/plugin/builtin"
	"helix/internal/plugin/loader"
	"helix/internal/proxy"
	"helix/internal/ratelimit"
	"helix/internal/router"
	"helix/internal/store"
	"helix/internal/telemetry"
	helixtls "helix/internal/tls"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	config.LoadDotEnv(".env")
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	promReg := prometheus.NewRegistry()
	promReg.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	metrics := telemetry.NewMetrics(promReg)

	otelEndpoint := config.EnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	tracer, shutdownTracer, err := telemetry.InitTracer(ctx, otelEndpoint)
	if err != nil {
		slog.Warn("tracing unavailable, continuing without traces", "err", err)
		tracer, shutdownTracer, _ = telemetry.InitTracer(ctx, "")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		slog.Warn("redis unavailable — rate limiting degraded to fail-open", "err", err)
	}
	pingCancel()

	limiter := ratelimit.New(rdb, cfg.RateLimit)
	jwtValidator := auth.NewValidator(cfg.JWTSecret)

	proxyHandler, err := proxy.New(cfg.BackendURL)
	if err != nil {
		slog.Error("failed to create proxy", "err", err)
		os.Exit(1)
	}

	storePath := config.EnvOrDefault("HELIX_STORE_PATH", "helix-config.json")
	st, err := store.New(storePath)
	if err != nil {
		slog.Error("failed to load config store", "path", storePath, "err", err)
		os.Exit(1)
	}

	tracker := helixmetrics.NewTracker()
	auditLog := audit.New(0)

	checker := health.New()
	checker.Start(ctx, st, 10*time.Second)

	pluginReg := plugin.NewRegistry()
	pluginReg.Register("security-headers", func(_ map[string]any) (plugin.Plugin, error) {
		return builtin.SecurityHeaders(), nil
	})
	pluginReg.Register("cors", func(c map[string]any) (plugin.Plugin, error) {
		var origins []string
		if o, ok := c["allowed_origins"].([]any); ok {
			for _, v := range o {
				if s, ok := v.(string); ok {
					origins = append(origins, s)
				}
			}
		}
		return builtin.CORS(origins), nil
	})
	pluginReg.Register("circuit-breaker", func(_ map[string]any) (plugin.Plugin, error) {
		return builtin.CircuitBreaker(builtin.DefaultCircuitBreakerConfig()), nil
	})
	pluginReg.Register("jwt-auth", func(_ map[string]any) (plugin.Plugin, error) {
		return builtin.JWT(jwtValidator, cfg.PublicPaths), nil
	})
	pluginReg.Register("ip-rate-limit", func(_ map[string]any) (plugin.Plugin, error) {
		return builtin.IPRateLimit(limiter, metrics, cfg.BypassPrefixes), nil
	})
	pluginReg.Register("user-rate-limit", func(_ map[string]any) (plugin.Plugin, error) {
		return builtin.UserRateLimit(limiter, metrics), nil
	})
	pluginReg.Register("auth-rate-limit", func(_ map[string]any) (plugin.Plugin, error) {
		return builtin.AuthRateLimit(limiter, metrics), nil
	})
	pluginReg.Register("ip-policy", func(_ map[string]any) (plugin.Plugin, error) {
		return builtin.IPPolicyEnforcer(st), nil
	})
	pluginReg.Register("route-rate-limit", func(c map[string]any) (plugin.Plugin, error) {
		rps := configInt(c, "rps", 100)
		burst := configInt(c, "burst", 200)
		return builtin.RouteRateLimit(rps, burst), nil
	})
	pluginReg.Register("transform", func(c map[string]any) (plugin.Plugin, error) {
		cfg := transformConfigFromMap(c)
		return builtin.Transform(cfg), nil
	})
	pluginReg.Register("api-key-auth", func(c map[string]any) (plugin.Plugin, error) {
		header, _ := c["header"].(string)
		return builtin.APIKeyAuth(st, header), nil
	})

	if pluginDir := config.EnvOrDefault("HELIX_PLUGIN_DIR", ""); pluginDir != "" {
		pl := loader.New(pluginReg)
		if err := pl.Watch(pluginDir); err != nil {
			slog.Warn("plugin watcher failed to start", "dir", pluginDir, "err", err)
		} else {
			slog.Info("plugin hot-reload watching", "dir", pluginDir)
		}
	}

	fallback := plugin.Chain(
		proxyHandler,
		builtin.CircuitBreaker(builtin.DefaultCircuitBreakerConfig()),
	)

	dynRouter := router.New(st, pluginReg, tracker, checker, fallback)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/metrics", promhttp.HandlerFor(promReg, promhttp.HandlerOpts{}))
	mux.Handle("/", dynRouter)

	handler := plugin.Chain(mux,
		builtin.CorrelationID(),
		builtin.Tracing(tracer),
		builtin.Logging(),
		builtin.HTTPMetrics(metrics),
		builtin.SecurityHeaders(),
		builtin.CORS(cfg.AllowedOrigins),
		builtin.RequestSecurity(),
		builtin.IPPolicyEnforcer(st),
		builtin.AuthRateLimit(limiter, metrics),
		builtin.IPRateLimit(limiter, metrics, cfg.BypassPrefixes),
		builtin.JWT(jwtValidator, cfg.PublicPaths),
		builtin.UserRateLimit(limiter, metrics),
	)

	h2cHandler := h2c.NewHandler(handler, &http2.Server{})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      h2cHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	adminPort := config.EnvOrDefault("HELIX_ADMIN_PORT", "9090")
	adminSrv := admin.New(adminPort, st, pluginReg, tracker, auditLog)
	adminSrv.Start()

	go func() {
		slog.Info("helix gateway started", "port", cfg.Port, "backend", cfg.BackendURL, "admin_port", adminPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	if domains := tlsDomains(); len(domains) > 0 {
		cacheDir := config.EnvOrDefault("HELIX_TLS_CACHE_DIR", "")
		tlsMgr := helixtls.New(domains, cacheDir)

		httpRedirectSrv := &http.Server{
			Addr:         ":80",
			Handler:      tlsMgr.HTTPChallengeHandler(),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		go func() {
			slog.Info("helix tls http challenge server started", "addr", ":80")
			if err := httpRedirectSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Warn("http challenge server error", "err", err)
			}
		}()

		httpsSrv := &http.Server{
			Addr:         ":443",
			Handler:      h2cHandler,
			TLSConfig:    tlsMgr.TLSConfig(),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
		go func() {
			slog.Info("helix tls gateway started", "addr", ":443", "domains", domains)
			if err := httpsSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				slog.Error("tls server error", "err", err)
			}
		}()
	}

	<-ctx.Done()
	slog.Info("shutting down helix")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
	adminSrv.Shutdown(shutCtx)
	if shutdownTracer != nil {
		shutdownTracer(shutCtx)
	}
}

func tlsDomains() []string {
	raw := config.EnvOrDefault("HELIX_TLS_DOMAINS", "")
	if raw == "" {
		return nil
	}
	var domains []string
	for _, d := range strings.Split(raw, ",") {
		if d = strings.TrimSpace(d); d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

func configInt(c map[string]any, key string, def int) int {
	if c == nil {
		return def
	}
	switch v := c[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	}
	return def
}

func transformConfigFromMap(c map[string]any) builtin.TransformConfig {
	cfg := builtin.TransformConfig{}
	if v, ok := c["set_request_headers"].(map[string]any); ok {
		cfg.SetRequestHeaders = make(map[string]string, len(v))
		for k, val := range v {
			if s, ok := val.(string); ok {
				cfg.SetRequestHeaders[k] = s
			}
		}
	}
	if v, ok := c["remove_request_headers"].([]any); ok {
		for _, val := range v {
			if s, ok := val.(string); ok {
				cfg.RemoveRequestHeaders = append(cfg.RemoveRequestHeaders, s)
			}
		}
	}
	if v, ok := c["set_response_headers"].(map[string]any); ok {
		cfg.SetResponseHeaders = make(map[string]string, len(v))
		for k, val := range v {
			if s, ok := val.(string); ok {
				cfg.SetResponseHeaders[k] = s
			}
		}
	}
	if v, ok := c["remove_response_headers"].([]any); ok {
		for _, val := range v {
			if s, ok := val.(string); ok {
				cfg.RemoveResponseHeaders = append(cfg.RemoveResponseHeaders, s)
			}
		}
	}
	if rw, ok := c["url_rewrite"].(map[string]any); ok {
		prefix, _ := rw["prefix_match"].(string)
		replacement, _ := rw["replacement"].(string)
		if prefix != "" {
			cfg.URLRewrite = &builtin.URLRewriteConfig{
				PrefixMatch: prefix,
				Replacement: replacement,
			}
		}
	}
	return cfg
}
