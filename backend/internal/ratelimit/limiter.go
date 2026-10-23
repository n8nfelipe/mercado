package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var script = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("TTL", KEYS[1])
return {current, ttl}
`)

type Limiter struct {
	client *redis.Client
	window time.Duration
}

func New(client *redis.Client, window time.Duration) *Limiter {
	return &Limiter{client: client, window: window}
}

func (l *Limiter) Allow(ctx context.Context, key string, limit int) (bool, time.Duration, error) {
	windowSeconds := int64(l.window / time.Second)
	if windowSeconds < 1 {
		windowSeconds = 1
	}

	result, err := script.Run(ctx, l.client, []string{"ratelimit:" + key}, windowSeconds).Slice()
	if err != nil {
		return false, 0, fmt.Errorf("rate limit script: %w", err)
	}
	if len(result) < 2 {
		return false, 0, fmt.Errorf("rate limit script returned %d values", len(result))
	}

	current, err := toInt64(result[0])
	if err != nil {
		return false, 0, err
	}
	ttl, err := toInt64(result[1])
	if err != nil {
		return false, 0, err
	}
	if ttl <= 0 {
		ttl = windowSeconds
	}

	if current > int64(limit) {
		return false, time.Duration(ttl) * time.Second, nil
	}
	return true, 0, nil
}

func toInt64(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("rate limit script returned %q: %w", typed, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("rate limit script returned unexpected type %T", value)
	}
}
