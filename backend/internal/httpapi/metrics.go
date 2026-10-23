package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	registry                   *prometheus.Registry
	requests                   *prometheus.CounterVec
	duration                   *prometheus.HistogramVec
	rateLimitFailOpen          prometheus.Counter
	listingModerationDecisions *prometheus.CounterVec
	listingsByStatus           *prometheus.GaugeVec
	catalogSearches            *prometheus.CounterVec
	catalogSearchFilters       *prometheus.CounterVec
	catalogListingNotFound     prometheus.Counter
}

func newMetrics(registry *prometheus.Registry) *metrics {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total de requisicoes HTTP por rota, metodo e status.",
	}, []string{"route", "method", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duracao das requisicoes HTTP por rota e metodo.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route", "method"})
	failOpen := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rate_limit_fail_open_total",
		Help: "Total de requisicoes liberadas porque o rate limit estava indisponivel.",
	})
	moderation := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "listing_moderation_decisions_total",
		Help: "Total de decisoes de moderacao e submissao por acao.",
	}, []string{"action"})
	listingsByStatus := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "listings_by_status",
		Help: "Total de anuncios por estado.",
	}, []string{"status"})
	searches := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "catalog_searches_total",
		Help: "Total de buscas publicas por resultado.",
	}, []string{"result"})
	filters := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "catalog_search_filters_total",
		Help: "Total de buscas que usaram cada filtro.",
	}, []string{"filter"})
	listingNotFound := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "catalog_listing_not_found_total",
		Help: "Total de detalhes publicos de anuncio nao encontrados.",
	})
	registry.MustRegister(requests, duration, failOpen, moderation, listingsByStatus, searches, filters, listingNotFound)
	return &metrics{
		registry:                   registry,
		requests:                   requests,
		duration:                   duration,
		rateLimitFailOpen:          failOpen,
		listingModerationDecisions: moderation,
		listingsByStatus:           listingsByStatus,
		catalogSearches:            searches,
		catalogSearchFilters:       filters,
		catalogListingNotFound:     listingNotFound,
	}
}

func (m *metrics) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		writer := &statusWriter{ResponseWriter: w}
		defer func() {
			status := statusOf(writer)
			if recovered := recover(); recovered != nil {
				status = http.StatusInternalServerError
				m.observe(r, status, start)
				panic(recovered)
			}
			m.observe(r, status, start)
		}()
		next.ServeHTTP(writer, r)
	})
}

func (m *metrics) observe(r *http.Request, status int, start time.Time) {
	route := chi.RouteContext(r.Context()).RoutePattern()
	if route == "" {
		route = "unmatched"
	}
	m.requests.WithLabelValues(route, r.Method, strconv.Itoa(status)).Inc()
	m.duration.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
}
