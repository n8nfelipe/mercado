package platform

import (
	"fmt"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"mercado/internal/config"
)

func NewRedis(cfg config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := redisotel.InstrumentTracing(client); err != nil {
		return nil, fmt.Errorf("instrument Redis tracing: %w", err)
	}
	return client, nil
}
