package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/trace"
)

type contextKey int

const correlationKey contextKey = iota

var correlationPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func CorrelationID(r *http.Request) string {
	value, _ := r.Context().Value(correlationKey).(string)
	return value
}

func correlation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Correlation-ID"))
		if !correlationPattern.MatchString(id) {
			id = newCorrelationID()
		}
		w.Header().Set("X-Correlation-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), correlationKey, id)))
	})
}

func newCorrelationID() string {
	var buffer [16]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buffer[:])
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	written, err := w.ResponseWriter.Write(body)
	w.bytes += written
	return written, err
}

func statusOf(w *statusWriter) int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		writer := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		attributes := []any{
			"service", s.cfg.ServiceName,
			"correlation_id", CorrelationID(r),
			"method", r.Method,
			"path", r.URL.Path,
			"route", route,
			"status", statusOf(writer),
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_ip", clientIP(r, s.cfg.TrustProxy),
			"bytes", writer.bytes,
		}
		if query := sanitizeQuery(r.URL.Query()); query != "" {
			attributes = append(attributes, "query", query)
		}
		if traceID := traceIDFrom(r.Context()); traceID != "" {
			attributes = append(attributes, "trace_id", traceID)
		}
		s.logger.Info("http request", attributes...)
	})
}

func (s *Server) recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("panic recovered",
					"correlation_id", CorrelationID(r),
					"panic", fmt.Sprint(recovered),
				)
				writeError(w, r, http.StatusInternalServerError, "internal_error", "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) bodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := s.cfg.MaxBodyBytes
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/seller/listings/") && strings.HasSuffix(r.URL.Path, "/images") {
			limit = maxImageBytes + 512<<10
		}
		if r.ContentLength > limit {
			writeError(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the configured limit")
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimit(next http.Handler) http.Handler {
	if !s.cfg.RateLimitEnabled || s.limiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rateLimitBypass(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		ip := clientIP(r, s.cfg.TrustProxy)
		if denied := s.checkRateLimit(w, r, "ip:"+ip, s.cfg.RateLimitIP); denied {
			return
		}

		if apiKey := strings.TrimSpace(r.Header.Get("X-API-Key")); apiKey != "" {
			if denied := s.checkRateLimit(w, r, "apikey:"+hashKey(apiKey), s.cfg.RateLimitAPIKey); denied {
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) checkRateLimit(w http.ResponseWriter, r *http.Request, key string, limit int) bool {
	allowed, retryAfter, err := s.limiter.Allow(r.Context(), key, limit)
	if err != nil {
		s.metrics.rateLimitFailOpen.Inc()
		s.logger.Warn("rate limit unavailable, failing open",
			"correlation_id", CorrelationID(r),
			"error", err,
		)
		return false
	}
	if allowed {
		return false
	}
	seconds := int(retryAfter / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, r, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
	return true
}

func rateLimitBypass(path string) bool {
	switch path {
	case "/healthz", "/readyz", "/metrics":
		return true
	default:
		return false
	}
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			if first != "" {
				return first
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func hashKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func traceIDFrom(ctx context.Context) string {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return ""
	}
	return spanContext.TraceID().String()
}

func sanitizeQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	copyValues := url.Values{}
	for key, entries := range values {
		if sensitiveKey(key) {
			copyValues[key] = []string{"[REDACTED]"}
			continue
		}
		copyValues[key] = entries
	}
	return copyValues.Encode()
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(key)
	for _, fragment := range []string{
		"password", "passwd", "token", "secret", "api_key", "apikey",
		"authorization", "cookie", "pan", "card", "cvv",
	} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}
