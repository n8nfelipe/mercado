package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"mercado/internal/config"
	"mercado/internal/platform"
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

	logger.Info("worker started", "service", cfg.ServiceName, "version", cfg.Version, "commit", cfg.Commit)
	<-ctx.Done()
	logger.Info("worker stopped")
	return 0
}
