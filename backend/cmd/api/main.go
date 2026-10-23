package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"mercado/internal/cache"
	"mercado/internal/config"
	"mercado/internal/httpapi"
	"mercado/internal/platform"
	"mercado/internal/ratelimit"
	"mercado/internal/storage"
	"mercado/internal/store"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		return 1
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracer, err := platform.InitTracer(ctx, cfg)
	if err != nil {
		logger.Error("failed to initialize tracing", "error", err)
		return 1
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracer(shutdownContext)
	}()

	postgres, err := platform.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to configure PostgreSQL", "error", err)
		return 1
	}
	defer postgres.Close()

	redisClient, err := platform.NewRedis(cfg)
	if err != nil {
		logger.Error("failed to configure Redis", "error", err)
		return 1
	}
	defer func() { _ = redisClient.Close() }()

	var blobs httpapi.BlobStore
	if cfg.MinIOAccessKey != "" {
		storageContext, cancel := context.WithTimeout(ctx, 15*time.Second)
		blobStore, err := storage.New(storageContext, storage.Config{
			Endpoint:  cfg.MinIOEndpoint,
			AccessKey: cfg.MinIOAccessKey,
			SecretKey: cfg.MinIOSecretKey,
			Bucket:    cfg.MinIOBucket,
			UseSSL:    cfg.MinIOUseSSL,
		})
		cancel()
		if err != nil {
			logger.Error("failed to configure object storage", "error", err)
			return 1
		}
		blobs = blobStore
	} else {
		logger.Warn("object storage disabled because credentials are empty")
	}

	limiter := ratelimit.New(redisClient, cfg.RateLimitWindow)
	server := httpapi.New(httpapi.Deps{
		Config:   cfg,
		Logger:   logger,
		Postgres: postgres,
		Redis:    httpapi.PingerFunc(func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }),
		Limiter:  limiter,
		Store:    store.New(postgres),
		Storage:  blobs,
		Cache:    cache.New(redisClient, logger),
	})
	server.WrapHandler(func(handler http.Handler) http.Handler {
		return otelhttp.NewHandler(handler, "http.server")
	})

	logger.Info("starting API",
		"addr", cfg.Addr,
		"service", cfg.ServiceName,
		"version", cfg.Version,
		"commit", cfg.Commit,
	)
	if err := server.Run(ctx); err != nil {
		logger.Error("API stopped with error", "error", err)
		return 1
	}
	logger.Info("API stopped")
	return 0
}
