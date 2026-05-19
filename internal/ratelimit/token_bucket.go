package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// tokenBucketScript atomically refills and consumes tokens from a bucket stored in a Redis hash.
// Keys: [1] = bucket key
// Args: [1]=capacity [2]=rate(tokens/sec) [3]=now_ms [4]=cost
// Returns: {allowed(0|1), floor(remaining_tokens)}
const tokenBucketScript = `
local key      = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate     = tonumber(ARGV[2])
local now_ms   = tonumber(ARGV[3])
local cost     = tonumber(ARGV[4])

local data   = redis.call('HMGET', key, 't', 'ts')
local tokens = tonumber(data[1])
local ts     = tonumber(data[2])

if not tokens then
  tokens = capacity
  ts     = now_ms
end

local elapsed = (now_ms - ts) / 1000.0
tokens = math.min(capacity, tokens + elapsed * rate)
ts     = now_ms

local allowed = 0
if tokens >= cost then
  tokens  = tokens - cost
  allowed = 1
end

redis.call('HMSET', key, 't', tokens, 'ts', ts)
redis.call('PEXPIRE', key, 3600000)
return { allowed, math.floor(tokens) }
`

type TokenBucket struct {
	rdb      *redis.Client
	script   *redis.Script
	capacity float64
	rate     float64
}

func newTokenBucket(rdb *redis.Client, capacity, rate float64) *TokenBucket {
	return &TokenBucket{
		rdb:      rdb,
		script:   redis.NewScript(tokenBucketScript),
		capacity: capacity,
		rate:     rate,
	}
}

// allow returns (allowed, remaining). Fails open on Redis error.
func (tb *TokenBucket) allow(ctx context.Context, key string) (bool, int64, error) {
	res, err := tb.script.Run(ctx, tb.rdb,
		[]string{fmt.Sprintf("rl:tb:%s", key)},
		tb.capacity, tb.rate, time.Now().UnixMilli(), 1,
	).Int64Slice()
	if err != nil {
		return true, 0, err
	}
	return res[0] == 1, res[1], nil
}
