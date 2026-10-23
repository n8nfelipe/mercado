package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"

	"mercado/internal/config"
	"mercado/internal/ratelimit"
)

func testServer(t *testing.T, mutate func(*Deps)) (*Server, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	deps := Deps{
		Config: config.Config{
			ServiceName:      "mercado-api",
			Version:          "test",
			Commit:           "abc123",
			ReadinessTimeout: time.Second,
			MaxBodyBytes:     1024,
			ShutdownTimeout:  time.Second,
			TrustProxy:       false,
			RateLimitEnabled: false,
		},
		Logger: slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Postgres: PingerFunc(func(context.Context) error {
			return nil
		}),
		Redis: PingerFunc(func(context.Context) error {
			return nil
		}),
	}
	if mutate != nil {
		mutate(&deps)
	}
	return New(deps), &logs
}

func perform(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, recorder.Body.String())
	}
	return response
}

func TestHealthAndReadiness(t *testing.T) {
	server, _ := testServer(t, nil)

	if response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/healthz", nil)); response.Code != http.StatusOK {
		t.Fatalf("GET /healthz = %d, want 200", response.Code)
	}
	if response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/readyz", nil)); response.Code != http.StatusOK {
		t.Fatalf("GET /readyz = %d, want 200", response.Code)
	}

	server, _ = testServer(t, func(deps *Deps) {
		deps.Postgres = PingerFunc(func(context.Context) error {
			return errors.New("postgres unavailable")
		})
	})

	if response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/healthz", nil)); response.Code != http.StatusOK {
		t.Fatalf("GET /healthz with postgres down = %d, want 200", response.Code)
	}
	response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz with postgres down = %d, want 503", response.Code)
	}
	var readiness readinessResponse
	if err := json.Unmarshal(response.Body.Bytes(), &readiness); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	if readiness.Checks["postgres"] != "down" || readiness.Checks["redis"] != "up" {
		t.Fatalf("readiness checks = %+v, want postgres down and redis up", readiness.Checks)
	}
}

func TestStatus(t *testing.T) {
	server, _ := testServer(t, nil)
	response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /v1/status = %d, want 200", response.Code)
	}
	var status statusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if status.Service != "mercado-api" || status.Version != "test" || status.Commit != "abc123" || status.Status != "ok" {
		t.Fatalf("status response = %+v", status)
	}
}

func TestCorrelationID(t *testing.T) {
	server, _ := testServer(t, nil)
	server.Router().Get("/correlation", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(CorrelationID(r)))
	})

	request := httptest.NewRequest(http.MethodGet, "/correlation", nil)
	request.Header.Set("X-Correlation-ID", "known-id")
	response := perform(server.Handler(), request)
	if response.Header().Get("X-Correlation-ID") != "known-id" {
		t.Fatalf("response correlation id = %q, want known-id", response.Header().Get("X-Correlation-ID"))
	}
	if response.Body.String() != "known-id" {
		t.Fatalf("context correlation id = %q, want known-id", response.Body.String())
	}

	response = perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/correlation", nil))
	if response.Header().Get("X-Correlation-ID") == "" {
		t.Fatal("generated correlation id is empty")
	}
}

func TestPanicRecovery(t *testing.T) {
	server, _ := testServer(t, nil)
	server.Router().Get("/panic", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/panic", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("GET /panic = %d, want 500", response.Code)
	}
	if payload := decodeError(t, response); payload.Error.Code != "internal_error" {
		t.Fatalf("error code = %q, want internal_error", payload.Error.Code)
	}

	if response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/healthz", nil)); response.Code != http.StatusOK {
		t.Fatalf("GET /healthz after panic = %d, want 200", response.Code)
	}
}

func TestAccessLogRedactsSensitiveData(t *testing.T) {
	server, logs := testServer(t, nil)
	request := httptest.NewRequest(http.MethodGet, "/v1/status?password=super-secret&token=abc123&normal=ok", strings.NewReader(`{"password":"body-secret","token":"body-token"}`))
	request.Header.Set("Authorization", "Bearer top-secret")
	request.Header.Set("Cookie", "session=secret-cookie")
	request.Header.Set("X-Correlation-ID", "log-correlation")

	response := perform(server.Handler(), request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET /v1/status = %d, want 200", response.Code)
	}

	logged := logs.String()
	for _, secret := range []string{"super-secret", "abc123", "top-secret", "secret-cookie", "body-secret", "body-token"} {
		if strings.Contains(logged, secret) {
			t.Errorf("log contains sensitive value %q: %s", secret, logged)
		}
	}
	if !strings.Contains(logged, "REDACTED") {
		t.Errorf("log does not contain redaction marker: %s", logged)
	}
	if !strings.Contains(logged, "normal=ok") {
		t.Errorf("log does not contain non-sensitive query value: %s", logged)
	}
	if !strings.Contains(logged, "log-correlation") {
		t.Errorf("log does not contain correlation id: %s", logged)
	}
}

func TestMetrics(t *testing.T) {
	server, _ := testServer(t, nil)
	perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/status", nil))
	response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want 200", response.Code)
	}

	body := response.Body.String()
	if !strings.Contains(body, `http_requests_total{method="GET",route="/v1/status",status="200"} 1`) {
		t.Errorf("metrics do not contain the exercised route: %s", body)
	}
	if !strings.Contains(body, `http_request_duration_seconds_count{method="GET",route="/v1/status"} 1`) {
		t.Errorf("metrics do not contain the duration series: %s", body)
	}
	if strings.Contains(body, `route="/never"`) {
		t.Errorf("metrics contain a route without traffic: %s", body)
	}
}

func TestBodyLimit(t *testing.T) {
	server, _ := testServer(t, func(deps *Deps) {
		deps.Config.MaxBodyBytes = 16
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/status", strings.NewReader(strings.Repeat("a", 100)))
	response := perform(server.Handler(), request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized request = %d, want 413", response.Code)
	}
	if payload := decodeError(t, response); payload.Error.Code != "request_too_large" {
		t.Fatalf("error code = %q, want request_too_large", payload.Error.Code)
	}
}

func TestNotFoundAndMethodNotAllowed(t *testing.T) {
	server, _ := testServer(t, nil)

	response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("GET /missing = %d, want 404", response.Code)
	}
	if payload := decodeError(t, response); payload.Error.Code != "not_found" {
		t.Fatalf("error code = %q, want not_found", payload.Error.Code)
	}

	response = perform(server.Handler(), httptest.NewRequest(http.MethodPost, "/v1/status", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /v1/status = %d, want 405", response.Code)
	}
	if payload := decodeError(t, response); payload.Error.Code != "method_not_allowed" {
		t.Fatalf("error code = %q, want method_not_allowed", payload.Error.Code)
	}
}

func newRateLimitedServer(t *testing.T, ipLimit, keyLimit int) (*Server, *miniredis.Miniredis) {
	t.Helper()
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	server, _ := testServer(t, func(deps *Deps) {
		deps.Config.RateLimitEnabled = true
		deps.Config.RateLimitIP = ipLimit
		deps.Config.RateLimitAPIKey = keyLimit
		deps.Config.RateLimitWindow = time.Minute
		deps.Limiter = ratelimit.New(client, time.Minute)
	})
	return server, redisServer
}

func TestRateLimitReturns429AndRetryAfter(t *testing.T) {
	server, _ := newRateLimitedServer(t, 1, 100)

	if response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/status", nil)); response.Code != http.StatusOK {
		t.Fatalf("first request = %d, want 200", response.Code)
	}
	response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/status", nil))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("second request = %d, want 429", response.Code)
	}
	retryAfter, err := strconv.Atoi(response.Header().Get("Retry-After"))
	if err != nil || retryAfter < 1 {
		t.Fatalf("Retry-After = %q, want a positive integer", response.Header().Get("Retry-After"))
	}
	if payload := decodeError(t, response); payload.Error.Code != "rate_limited" {
		t.Fatalf("error code = %q, want rate_limited", payload.Error.Code)
	}
}

func TestRateLimitIsolatesAPIKeys(t *testing.T) {
	server, _ := newRateLimitedServer(t, 100, 1)

	first := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	first.Header.Set("X-API-Key", "key-a")
	if response := perform(server.Handler(), first); response.Code != http.StatusOK {
		t.Fatalf("first key-a request = %d, want 200", response.Code)
	}

	second := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	second.Header.Set("X-API-Key", "key-a")
	if response := perform(server.Handler(), second); response.Code != http.StatusTooManyRequests {
		t.Fatalf("second key-a request = %d, want 429", response.Code)
	}

	other := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	other.Header.Set("X-API-Key", "key-b")
	if response := perform(server.Handler(), other); response.Code != http.StatusOK {
		t.Fatalf("key-b request = %d, want 200", response.Code)
	}
}

func TestRateLimitFailsOpenWhenRedisIsDown(t *testing.T) {
	server, redisServer := newRateLimitedServer(t, 1, 1)
	redisServer.Close()

	response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("request with Redis down = %d, want 200 (fail-open)", response.Code)
	}
	if value := testutil.ToFloat64(server.metrics.rateLimitFailOpen); value < 1 {
		t.Fatalf("rate_limit_fail_open_total = %v, want at least 1", value)
	}
}

func TestGracefulShutdownFinishesInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, &http.Server{Handler: mux}, listener, 5*time.Second)
	}()

	responseChannel := make(chan *http.Response, 1)
	errorChannel := make(chan error, 1)
	go func() {
		client := &http.Client{Transport: &http.Transport{Proxy: nil}}
		response, err := client.Get("http://" + listener.Addr().String() + "/slow")
		if err != nil {
			errorChannel <- err
			return
		}
		responseChannel <- response
	}()

	<-started
	cancel()
	time.Sleep(100 * time.Millisecond)

	if connection, err := net.DialTimeout("tcp", listener.Addr().String(), 200*time.Millisecond); err == nil {
		_ = connection.Close()
		t.Fatal("new connection was accepted during shutdown")
	}

	close(release)
	select {
	case err := <-errorChannel:
		t.Fatalf("in-flight request failed: %v", err)
	case response := <-responseChannel:
		defer func() { _ = response.Body.Close() }()
		body, _ := io.ReadAll(response.Body)
		if response.StatusCode != http.StatusOK || string(body) != "ok" {
			t.Fatalf("in-flight response = %d %q, want 200 ok", response.StatusCode, body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight request did not finish")
	}
	if err := <-done; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
}
