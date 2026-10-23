package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"mercado/internal/version"
)

type Config struct {
	Addr              string
	ServiceName       string
	Version           string
	Commit            string
	DatabaseURL       string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	OTLPEndpoint      string
	ShutdownTimeout   time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	ReadinessTimeout  time.Duration
	MaxBodyBytes      int64
	RateLimitEnabled  bool
	RateLimitIP       int
	RateLimitAPIKey   int
	RateLimitWindow   time.Duration
	TrustProxy        bool
	LogLevel          slog.Level
	SessionCookieName string
	SessionTTL        time.Duration
	SessionSecure     bool
	AllowedOrigins    []string
	LoginRateLimit    int
	MinIOEndpoint     string
	MinIOAccessKey    string
	MinIOSecretKey    string
	MinIOBucket       string
	MinIOUseSSL       bool
}

func Load() (Config, error) {
	return LoadFrom(os.Getenv)
}

func LoadFrom(getenv func(string) string) (Config, error) {
	var errs []string

	port := text(getenv, "PORT", "8080")
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		errs = append(errs, fmt.Sprintf("PORT must be a number between 1 and 65535, got %q", port))
	}
	addr := text(getenv, "ADDR", ":"+port)

	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	if databaseURL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}
	redisAddr := strings.TrimSpace(getenv("REDIS_ADDR"))
	if redisAddr == "" {
		errs = append(errs, "REDIS_ADDR is required")
	}

	redisDB := integer(getenv, "REDIS_DB", 0, &errs)
	if redisDB < 0 {
		errs = append(errs, "REDIS_DB must be greater than or equal to 0")
	}

	shutdownTimeout := duration(getenv, "SHUTDOWN_TIMEOUT", 10*time.Second, &errs)
	readTimeout := duration(getenv, "READ_TIMEOUT", 10*time.Second, &errs)
	writeTimeout := duration(getenv, "WRITE_TIMEOUT", 15*time.Second, &errs)
	idleTimeout := duration(getenv, "IDLE_TIMEOUT", 60*time.Second, &errs)
	readHeaderTimeout := duration(getenv, "READ_HEADER_TIMEOUT", 5*time.Second, &errs)
	readinessTimeout := duration(getenv, "READINESS_TIMEOUT", 2*time.Second, &errs)
	rateLimitWindow := duration(getenv, "RATE_LIMIT_WINDOW", time.Minute, &errs)

	maxBodyBytes := int64(integer(getenv, "MAX_BODY_BYTES", 1<<20, &errs))
	if maxBodyBytes <= 0 {
		errs = append(errs, "MAX_BODY_BYTES must be greater than 0")
	}

	sessionTTL := duration(getenv, "SESSION_TTL", 7*24*time.Hour, &errs)
	if sessionTTL <= 0 {
		errs = append(errs, "SESSION_TTL must be greater than 0")
	}
	sessionCookieName := text(getenv, "SESSION_COOKIE_NAME", "mercado_session")
	if strings.ContainsAny(sessionCookieName, " ;,") {
		errs = append(errs, "SESSION_COOKIE_NAME must not contain spaces, semicolons or commas")
	}
	loginRateLimit := integer(getenv, "LOGIN_RATE_LIMIT_PER_MINUTE", 10, &errs)
	if loginRateLimit <= 0 {
		errs = append(errs, "LOGIN_RATE_LIMIT_PER_MINUTE must be greater than 0")
	}
	minioAccessKey := strings.TrimSpace(getenv("MINIO_ACCESS_KEY"))
	minioSecretKey := strings.TrimSpace(getenv("MINIO_SECRET_KEY"))
	if (minioAccessKey == "") != (minioSecretKey == "") {
		errs = append(errs, "MINIO_ACCESS_KEY and MINIO_SECRET_KEY must be set together")
	}
	minioBucket := text(getenv, "MINIO_BUCKET", "mercado-listings")
	if minioBucket == "" {
		errs = append(errs, "MINIO_BUCKET must not be empty")
	}
	minioUseSSL, err := strconv.ParseBool(text(getenv, "MINIO_USE_SSL", "false"))
	if err != nil {
		errs = append(errs, fmt.Sprintf("MINIO_USE_SSL must be a boolean, got %q", getenv("MINIO_USE_SSL")))
	}

	allowedOrigins := parseOrigins(text(getenv, "ALLOWED_ORIGINS",
		"http://mercado.localhost:8080,http://127.0.0.1:3000,http://localhost:3000"))
	if len(allowedOrigins) == 0 {
		errs = append(errs, "ALLOWED_ORIGINS must contain at least one origin")
	}

	rateLimitIP := integer(getenv, "RATE_LIMIT_IP_PER_MINUTE", 120, &errs)
	rateLimitAPIKey := integer(getenv, "RATE_LIMIT_API_KEY_PER_MINUTE", 600, &errs)
	if rateLimitIP <= 0 {
		errs = append(errs, "RATE_LIMIT_IP_PER_MINUTE must be greater than 0")
	}
	if rateLimitAPIKey <= 0 {
		errs = append(errs, "RATE_LIMIT_API_KEY_PER_MINUTE must be greater than 0")
	}

	for name, value := range map[string]time.Duration{
		"SHUTDOWN_TIMEOUT":    shutdownTimeout,
		"READ_TIMEOUT":        readTimeout,
		"WRITE_TIMEOUT":       writeTimeout,
		"IDLE_TIMEOUT":        idleTimeout,
		"READ_HEADER_TIMEOUT": readHeaderTimeout,
		"READINESS_TIMEOUT":   readinessTimeout,
		"RATE_LIMIT_WINDOW":   rateLimitWindow,
	} {
		if value <= 0 {
			errs = append(errs, name+" must be greater than 0")
		}
	}

	level, err := parseLevel(text(getenv, "LOG_LEVEL", "info"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	trustProxy, err := strconv.ParseBool(text(getenv, "TRUST_PROXY", "true"))
	if err != nil {
		errs = append(errs, fmt.Sprintf("TRUST_PROXY must be a boolean, got %q", getenv("TRUST_PROXY")))
	}
	rateLimitEnabled, err := strconv.ParseBool(text(getenv, "RATE_LIMIT_ENABLED", "true"))
	if err != nil {
		errs = append(errs, fmt.Sprintf("RATE_LIMIT_ENABLED must be a boolean, got %q", getenv("RATE_LIMIT_ENABLED")))
	}
	sessionSecure, err := strconv.ParseBool(text(getenv, "SESSION_SECURE", "false"))
	if err != nil {
		errs = append(errs, fmt.Sprintf("SESSION_SECURE must be a boolean, got %q", getenv("SESSION_SECURE")))
	}

	if len(errs) > 0 {
		return Config{}, errors.New(strings.Join(errs, "; "))
	}

	return Config{
		Addr:              addr,
		ServiceName:       text(getenv, "SERVICE_NAME", text(getenv, "OTEL_SERVICE_NAME", "mercado-api")),
		Version:           text(getenv, "SERVICE_VERSION", version.Version),
		Commit:            text(getenv, "GIT_COMMIT", version.Commit),
		DatabaseURL:       databaseURL,
		RedisAddr:         redisAddr,
		RedisPassword:     getenv("REDIS_PASSWORD"),
		RedisDB:           redisDB,
		OTLPEndpoint:      strings.TrimSpace(getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
		ShutdownTimeout:   shutdownTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadinessTimeout:  readinessTimeout,
		MaxBodyBytes:      maxBodyBytes,
		RateLimitEnabled:  rateLimitEnabled,
		RateLimitIP:       rateLimitIP,
		RateLimitAPIKey:   rateLimitAPIKey,
		RateLimitWindow:   rateLimitWindow,
		TrustProxy:        trustProxy,
		LogLevel:          level,
		SessionCookieName: sessionCookieName,
		SessionTTL:        sessionTTL,
		SessionSecure:     sessionSecure,
		AllowedOrigins:    allowedOrigins,
		LoginRateLimit:    loginRateLimit,
		MinIOEndpoint:     text(getenv, "MINIO_ENDPOINT", "minio.mercado-data.svc.cluster.local:9000"),
		MinIOAccessKey:    minioAccessKey,
		MinIOSecretKey:    minioSecretKey,
		MinIOBucket:       minioBucket,
		MinIOUseSSL:       minioUseSSL,
	}, nil
}

func text(getenv func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" {
		return value
	}
	return fallback
}

func integer(getenv func(string) string, key string, fallback int, errs *[]string) int {
	raw := text(getenv, key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s must be an integer, got %q", key, raw))
		return fallback
	}
	return value
}

func duration(getenv func(string) string, key string, fallback time.Duration, errs *[]string) time.Duration {
	raw := text(getenv, key, fallback.String())
	value, err := time.ParseDuration(raw)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s must be a duration, got %q", key, raw))
		return fallback
	}
	return value
}

func parseOrigins(raw string) []string {
	var origins []string
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin != "" && (strings.HasPrefix(origin, "http://") || strings.HasPrefix(origin, "https://")) {
			origins = append(origins, origin)
		}
	}
	return origins
}

func parseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error, got %q", raw)
	}
}
