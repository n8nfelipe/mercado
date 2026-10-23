package httpapi

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type statusResponse struct {
	Service string `json:"service"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Status  string `json:"status"`
}

type readinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{
		"postgres": s.check(r.Context(), s.postgres),
		"redis":    s.check(r.Context(), s.redis),
	}
	status := http.StatusOK
	overall := "ok"
	for _, result := range checks {
		if result != "up" {
			status = http.StatusServiceUnavailable
			overall = "unavailable"
			break
		}
	}
	writeJSON(w, status, readinessResponse{Status: overall, Checks: checks})
}

func (s *Server) check(parent context.Context, dependency Pinger) string {
	if dependency == nil {
		return "down"
	}
	ctx, cancel := context.WithTimeout(parent, s.cfg.ReadinessTimeout)
	defer cancel()
	if err := dependency.Ping(ctx); err != nil {
		return "down"
	}
	return "up"
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{
		Service: s.cfg.ServiceName,
		Version: s.cfg.Version,
		Commit:  s.cfg.Commit,
		Status:  "ok",
	})
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
}

func (s *Server) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

func (s *Server) metricsHandler() http.Handler {
	return promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{})
}
