package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"

	"mercado/internal/config"
	"mercado/internal/ratelimit"
	"mercado/internal/store"
)

type Pinger interface {
	Ping(context.Context) error
}

type PingerFunc func(context.Context) error

func (f PingerFunc) Ping(ctx context.Context) error {
	return f(ctx)
}

type StringCache interface {
	Get(context.Context, string) (string, bool)
	Set(context.Context, string, string, time.Duration)
	Delete(context.Context, ...string)
}

type Deps struct {
	Config   config.Config
	Logger   *slog.Logger
	Postgres Pinger
	Redis    Pinger
	Limiter  *ratelimit.Limiter
	Store    *store.Store
	Storage  BlobStore
	Cache    StringCache
}

type Server struct {
	cfg      config.Config
	logger   *slog.Logger
	postgres Pinger
	redis    Pinger
	limiter  *ratelimit.Limiter
	store    *store.Store
	storage  BlobStore
	cache    StringCache
	registry *prometheus.Registry
	metrics  *metrics
	router   chi.Router
	handler  http.Handler
}

func New(deps Deps) *Server {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	registry := prometheus.NewRegistry()
	server := &Server{
		cfg:      deps.Config,
		logger:   logger,
		postgres: deps.Postgres,
		redis:    deps.Redis,
		limiter:  deps.Limiter,
		store:    deps.Store,
		storage:  deps.Storage,
		cache:    deps.Cache,
		registry: registry,
		metrics:  newMetrics(registry),
	}

	router := chi.NewRouter()
	router.Use(correlation)
	router.Use(server.accessLog)
	router.Use(server.recovery)
	router.Use(server.metrics.middleware)
	router.Use(server.rateLimit)
	router.Use(server.bodyLimit)
	router.Use(server.csrfProtection)
	router.Get("/healthz", server.health)
	router.Get("/readyz", server.ready)
	router.Get("/metrics", server.metricsHandler().ServeHTTP)
	router.Get("/v1/status", server.status)

	router.Route("/v1/auth", func(r chi.Router) {
		r.Post("/register", server.register)
		r.Post("/login", server.login)
		r.With(server.authenticate).Post("/logout", server.logout)
		r.With(server.authenticate).Get("/me", server.me)
	})
	router.Get("/v1/sellers/{sellerId}", server.publicSeller)
	router.Get("/v1/categories", server.listPublicCategories)
	router.Get("/v1/catalog/listings", server.listPublicListings)
	router.Get("/v1/listings/{listingId}", server.getPublicListing)

	router.Route("/v1/seller", func(r chi.Router) {
		r.Use(server.authenticate)
		r.Post("/onboarding", server.becomeSeller)
		r.Get("/listings", server.listSellerListings)
		r.Post("/listings", server.createListing)
		r.Get("/listings/{listingId}", server.getSellerListing)
		r.Patch("/listings/{listingId}", server.updateListing)
		r.Post("/listings/{listingId}/submit", server.submitListing)
		r.Post("/listings/{listingId}/archive", server.archiveListing)
		r.Get("/listings/{listingId}/inventory", server.listInventoryAdjustments)
		r.Post("/listings/{listingId}/inventory", server.adjustInventory)
		r.Post("/listings/{listingId}/images", server.uploadListingImage)
		r.Patch("/listings/{listingId}/images/order", server.reorderListingImages)
		r.Delete("/listings/{listingId}/images/{imageId}", server.deleteListingImage)
	})

	router.Get("/v1/media/{listingId}/{fileName}", server.serveListingMedia)

	router.Route("/v1/admin", func(r chi.Router) {
		r.Use(server.authenticate, server.requireRole("admin"))
		r.Post("/categories", server.createCategory)
		r.Patch("/categories/{categoryId}", server.updateCategory)
		r.Delete("/categories/{categoryId}", server.deleteCategory)
		r.Get("/listings/pending", server.listPendingListings)
		r.Post("/listings/{listingId}/publish", server.publishListing)
		r.Post("/listings/{listingId}/reject", server.rejectListing)
		r.Post("/listings/{listingId}/suspend", server.suspendListing)
	})

	router.NotFound(server.notFound)
	router.MethodNotAllowed(server.methodNotAllowed)

	server.router = router
	server.handler = router
	return server
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) Router() chi.Router {
	return s.router
}

func (s *Server) WrapHandler(wrapper func(http.Handler) http.Handler) {
	s.handler = wrapper(s.handler)
}

func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	httpServer := &http.Server{
		Handler:           s.handler,
		ReadTimeout:       s.cfg.ReadTimeout,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		WriteTimeout:      s.cfg.WriteTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}
	return Serve(ctx, httpServer, listener, s.cfg.ShutdownTimeout)
}

func Serve(ctx context.Context, server *http.Server, listener net.Listener, shutdownTimeout time.Duration) error {
	result := make(chan error, 1)
	go func() {
		result <- server.Serve(listener)
	}()

	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		_ = server.Close()
		return err
	}
	return nil
}
