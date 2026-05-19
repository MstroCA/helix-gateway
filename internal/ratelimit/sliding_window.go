package ratelimit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// slidingWindowScript tracks requests in a sorted set keyed by timestamp.
// Keys: [1] = window key
// Args: [1]=now_ms [2]=window_ms [3]=limit [4]=unique_request_id
// Returns: {allowed(0|1), remaining}
const slidingWindowScript = `
local key    = KEYS[1]
local now_ms = tonumber(ARGV[1])
local win_ms = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
local req_id = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, '-inf', now_ms - win_ms)

local count = redis.call('ZCARD', key)
if count < limit then
  redis.call('ZADD', key, now_ms, req_id)
  redis.call('PEXPIRE', key, win_ms + 1000)
  return { 1, limit - count - 1 }
end
return { 0, 0 }
`

type SlidingWindow struct {
	rdb    *redis.Client
	script *redis.Script
	window time.Duration
	limit  int
}

func newSlidingWindow(rdb *redis.Client, window time.Duration, limit int) *SlidingWindow {
	return &SlidingWindow{
		rdb:    rdb,
		script: redis.NewScript(slidingWindowScript),
		window: window,
		limit:  limit,
	}
}

// allow returns (allowed, remaining). Fails open on Redis error.
func (sw *SlidingWindow) allow(ctx context.Context, key string) (bool, int64, error) {
	res, err := sw.script.Run(ctx, sw.rdb,
		[]string{fmt.Sprintf("rl:sw:%s", key)},
		time.Now().UnixMilli(),
		sw.window.Milliseconds(),
		sw.limit,
		reqID(),
	).Int64Slice()
	if err != nil {
		return true, 0, err
	}
	return res[0] == 1, res[1], nil
}

func reqID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
