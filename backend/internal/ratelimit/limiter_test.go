package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newLimiter(t *testing.T, window time.Duration) (*Limiter, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return New(client, window), server, client
}

func TestAllowUsesSharedWindow(t *testing.T) {
	limiter, _, _ := newLimiter(t, time.Minute)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		allowed, _, err := limiter.Allow(ctx, "ip:127.0.0.1", 2)
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	allowed, retryAfter, err := limiter.Allow(ctx, "ip:127.0.0.1", 2)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if allowed {
		t.Fatal("third request should be denied")
	}
	if retryAfter <= 0 || retryAfter > time.Minute {
		t.Errorf("retryAfter = %v, want between 1ns and 1m", retryAfter)
	}
}

func TestAllowIsolatesKeys(t *testing.T) {
	limiter, _, _ := newLimiter(t, time.Minute)
	ctx := context.Background()

	if allowed, _, _ := limiter.Allow(ctx, "ip:one", 1); !allowed {
		t.Fatal("first key should be allowed")
	}
	if allowed, _, _ := limiter.Allow(ctx, "ip:one", 1); allowed {
		t.Fatal("first key should be denied on second request")
	}
	if allowed, _, _ := limiter.Allow(ctx, "ip:two", 1); !allowed {
		t.Fatal("second key should be allowed")
	}
}

func TestAllowReturnsErrorWhenRedisIsDown(t *testing.T) {
	limiter, server, _ := newLimiter(t, time.Minute)
	server.Close()

	if _, _, err := limiter.Allow(context.Background(), "ip:one", 1); err == nil {
		t.Fatal("Allow() error = nil, want error")
	}
}
