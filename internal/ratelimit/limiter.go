package ratelimit

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"helix/internal/config"
)

// Limiter enforces per-IP, per-user, and per-auth-endpoint rate limits using
// token bucket (burst control) and sliding window (hard cap).
// Both algorithms run atomically in Redis via Lua scripts,
// providing a global aggregator across all gateway pods.
type Limiter struct {
	ipTB   *TokenBucket
	ipSW   *SlidingWindow
	userTB *TokenBucket
	userSW *SlidingWindow
	authTB *TokenBucket
	authSW *SlidingWindow
}

func New(rdb *redis.Client, cfg config.RateLimit) *Limiter {
	return &Limiter{
		ipTB:   newTokenBucket(rdb, cfg.IP.TBCapacity, cfg.IP.TBRate),
		ipSW:   newSlidingWindow(rdb, cfg.IP.SWWindow, cfg.IP.SWLimit),
		userTB: newTokenBucket(rdb, cfg.User.TBCapacity, cfg.User.TBRate),
		userSW: newSlidingWindow(rdb, cfg.User.SWWindow, cfg.User.SWLimit),
		authTB: newTokenBucket(rdb, cfg.Auth.TBCapacity, cfg.Auth.TBRate),
		authSW: newSlidingWindow(rdb, cfg.Auth.SWWindow, cfg.Auth.SWLimit),
	}
}

// AllowIP checks both token bucket and sliding window for the given IP.
// Returns false only when both Redis checks succeed and either rejects.
func (l *Limiter) AllowIP(ctx context.Context, ip string) bool {
	return l.check(ctx, "ip:"+ip, l.ipTB, l.ipSW)
}

// AllowUser checks both token bucket and sliding window for the given userID.
func (l *Limiter) AllowUser(ctx context.Context, userID string) bool {
	return l.check(ctx, "user:"+userID, l.userTB, l.userSW)
}

// AllowAuth applies strict per-IP limits specifically for authentication endpoints.
func (l *Limiter) AllowAuth(ctx context.Context, ip string) bool {
	return l.check(ctx, "auth:"+ip, l.authTB, l.authSW)
}

func (l *Limiter) check(ctx context.Context, key string, tb *TokenBucket, sw *SlidingWindow) bool {
	tbOK, _, tbErr := tb.allow(ctx, key)
	swOK, _, swErr := sw.allow(ctx, key)

	if tbErr != nil {
		slog.WarnContext(ctx, "rate limit redis error (token bucket)", "key", key, "err", tbErr)
	}
	if swErr != nil {
		slog.WarnContext(ctx, "rate limit redis error (sliding window)", "key", key, "err", swErr)
	}

	// Fail open: if both Redis calls error, allow the request.
	if tbErr != nil && swErr != nil {
		return true
	}
	// If only one errored, treat it as allowed and rely on the other.
	if tbErr != nil {
		return swOK
	}
	if swErr != nil {
		return tbOK
	}
	// Both must allow.
	return tbOK && swOK
}
