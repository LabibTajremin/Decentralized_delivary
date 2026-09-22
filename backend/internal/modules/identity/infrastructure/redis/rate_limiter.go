package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RateLimiter is a fixed-window counter.
//
// Fixed window, not sliding. A sliding window is fairer at the boundary, and
// costs a sorted set per key with the memory and expiry management that implies.
// For "five OTPs an hour" the boundary effect is that someone could send ten
// across two adjacent windows — which is still ten, still logged, and still far
// short of useful for brute force against a six-digit code with a five-attempt
// cap. The simpler structure is worth more than closing that gap.
type RateLimiter struct {
	client Client
}

// NewRateLimiter builds a limiter.
func NewRateLimiter(client Client) *RateLimiter { return &RateLimiter{client: client} }

// allowScript increments a counter and returns the count and the remaining TTL.
//
// One round trip, and atomic: with INCR and TTL as separate calls, a key that
// expires between them reports a fresh count with no window, and a caller that
// then sets the TTL has silently extended somebody's limit.
const allowScript = `
local count = redis.call('INCR', KEYS[1])
if count == 1 then
    redis.call('EXPIRE', KEYS[1], tonumber(ARGV[1]))
end
return {count, redis.call('TTL', KEYS[1])}
`

// Allow records one event and reports whether it is within the limit.
func (r *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	script := goredis.NewScript(allowScript)
	result, err := script.Run(ctx, r.client,
		[]string{"rate:" + key},
		int64(window.Seconds()),
	).Result()
	if err != nil {
		return false, 0, fmt.Errorf("rate limit: %w", err)
	}

	return ParseAllowReply(result, limit, window)
}

// ParseAllowReply reads the limiter script's reply.
//
// Pure and exported so its failure handling is testable. A limiter that
// misreads its own counter either blocks legitimate traffic or stops limiting,
// and both are silent.
func ParseAllowReply(result any, limit int, window time.Duration) (bool, time.Duration, error) {
	values, ok := result.([]any)
	if !ok || len(values) < 2 {
		return false, 0, fmt.Errorf("rate limit: unexpected reply %T", result)
	}
	count, _ := values[0].(int64)
	ttl, _ := values[1].(int64)

	retryAfter := time.Duration(ttl) * time.Second
	if ttl < 0 {
		// The key has no expiry, which the script should have set. Reporting
		// the full window rather than a negative one keeps a missing TTL from
		// becoming an unbounded lockout.
		retryAfter = window
	}
	return count <= int64(limit), retryAfter, nil
}
