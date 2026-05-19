package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port            string
	BackendURL      string
	JWTSecret       string
	AllowedOrigins  []string
	PublicPaths     []string
	BypassPrefixes  []string
	Redis           Redis
	RateLimit       RateLimit
}

type Redis struct {
	Addr     string
	Password string
	DB       int
}

type RateLimit struct {
	IP   LimiterCfg
	User LimiterCfg
	Auth LimiterCfg // strict limits for /api/auth/login — brute-force protection
}

type LimiterCfg struct {
	TBCapacity float64
	TBRate     float64
	SWWindow   time.Duration
	SWLimit    int
}

func Load() *Config {
	origins := splitEnv("ALLOWED_ORIGINS", "*")
	publicPaths := splitEnv("PUBLIC_PATHS", "/health")
	bypassPrefixes := splitEnv("BYPASS_PREFIXES", "")

	return &Config{
		Port:           env("PORT", "8090"),
		BackendURL:     env("BACKEND_URL", "http://backend:8080"),
		JWTSecret:      env("JWT_SECRET", ""),
		AllowedOrigins: origins,
		PublicPaths:    publicPaths,
		BypassPrefixes: bypassPrefixes,
		Redis: Redis{
			Addr:     env("REDIS_ADDR", "redis:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		RateLimit: RateLimit{
			IP: LimiterCfg{
				TBCapacity: envFloat("RL_IP_TB_CAPACITY", 100),
				TBRate:     envFloat("RL_IP_TB_RATE", 10),
				SWWindow:   envDur("RL_IP_SW_WINDOW", 60*time.Second),
				SWLimit:    envInt("RL_IP_SW_LIMIT", 300),
			},
			User: LimiterCfg{
				TBCapacity: envFloat("RL_USER_TB_CAPACITY", 500),
				TBRate:     envFloat("RL_USER_TB_RATE", 50),
				SWWindow:   envDur("RL_USER_SW_WINDOW", 60*time.Second),
				SWLimit:    envInt("RL_USER_SW_LIMIT", 1000),
			},
			Auth: LimiterCfg{
				// 5 login attempts burst, 1 token per 10s refill, hard cap 5/60s per IP
				TBCapacity: envFloat("RL_AUTH_TB_CAPACITY", 5),
				TBRate:     envFloat("RL_AUTH_TB_RATE", 0.1),
				SWWindow:   envDur("RL_AUTH_SW_WINDOW", 60*time.Second),
				SWLimit:    envInt("RL_AUTH_SW_LIMIT", 5),
			},
		},
	}
}

// LoadDotEnv reads key=value pairs from path and sets them as env vars when not
// already set. Silent if the file doesn't exist (safe to call in production).
// Existing shell variables always take precedence.
func LoadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || os.Getenv(key) != "" {
			continue // don't override existing vars
		}
		_ = os.Setenv(key, value)
	}
}

// EnvOrDefault is exported for use outside the package.
func EnvOrDefault(k, def string) string { return env(k, def) }

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitEnv(k, def string) []string {
	raw := env(k, def)
	if raw == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(k string, def float64) float64 {
	if v := os.Getenv(k); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envDur(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
