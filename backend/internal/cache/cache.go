package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
	logger *slog.Logger
}

func New(client *redis.Client, logger *slog.Logger) *Redis {
	if logger == nil {
		logger = slog.Default()
	}
	return &Redis{client: client, logger: logger}
}

func (c *Redis) Get(ctx context.Context, key string) (string, bool) {
	value, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			c.logger.Warn("cache get failed", "key", key, "error", err)
		}
		return "", false
	}
	return value, true
}

func (c *Redis) Set(ctx context.Context, key, value string, ttl time.Duration) {
	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		c.logger.Warn("cache set failed", "key", key, "error", err)
	}
}

func (c *Redis) Delete(ctx context.Context, keys ...string) {
	if len(keys) == 0 {
		return
	}
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		c.logger.Warn("cache delete failed", "keys", keys, "error", err)
	}
}
