package config

import (
	"strings"
	"testing"
	"time"
)

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://mercado:secret@127.0.0.1:5432/mercado",
		"REDIS_ADDR":   "127.0.0.1:6379",
	}
}

func envLookup(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadFromValid(t *testing.T) {
	values := validEnv()
	values["PORT"] = "9090"
	values["RATE_LIMIT_IP_PER_MINUTE"] = "10"
	values["RATE_LIMIT_API_KEY_PER_MINUTE"] = "20"
	values["SHUTDOWN_TIMEOUT"] = "5s"

	cfg, err := LoadFrom(envLookup(values))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want :9090", cfg.Addr)
	}
	if cfg.RateLimitIP != 10 || cfg.RateLimitAPIKey != 20 {
		t.Errorf("rate limits = %d/%d, want 10/20", cfg.RateLimitIP, cfg.RateLimitAPIKey)
	}
	if cfg.ShutdownTimeout != 5*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 5s", cfg.ShutdownTimeout)
	}
	if cfg.ServiceName != "mercado-api" {
		t.Errorf("ServiceName = %q, want mercado-api", cfg.ServiceName)
	}
}

func TestLoadFromMissingRequired(t *testing.T) {
	tests := []struct {
		name    string
		missing string
	}{
		{name: "database", missing: "DATABASE_URL"},
		{name: "redis", missing: "REDIS_ADDR"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := validEnv()
			delete(values, test.missing)
			_, err := LoadFrom(envLookup(values))
			if err == nil {
				t.Fatal("LoadFrom() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.missing) {
				t.Errorf("error = %q, want it to mention %s", err, test.missing)
			}
		})
	}
}

func TestLoadFromInvalidValues(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{key: "PORT", value: "not-a-port"},
		{key: "SHUTDOWN_TIMEOUT", value: "soon"},
		{key: "MAX_BODY_BYTES", value: "0"},
		{key: "RATE_LIMIT_IP_PER_MINUTE", value: "0"},
		{key: "REDIS_DB", value: "-1"},
		{key: "LOG_LEVEL", value: "verbose"},
		{key: "TRUST_PROXY", value: "sometimes"},
	}
	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			values := validEnv()
			values[test.key] = test.value
			_, err := LoadFrom(envLookup(values))
			if err == nil {
				t.Fatalf("LoadFrom() error = nil, want error for %s=%s", test.key, test.value)
			}
			if !strings.Contains(err.Error(), test.key) {
				t.Errorf("error = %q, want it to mention %s", err, test.key)
			}
		})
	}
}
