package platform

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"mercado/internal/config"
)

func InitTracer(ctx context.Context, cfg config.Config) (func(context.Context) error, error) {
	if cfg.OTLPEndpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	endpoint := strings.TrimSpace(cfg.OTLPEndpoint)
	insecure := true
	if strings.HasPrefix(endpoint, "https://") {
		insecure = false
		endpoint = strings.TrimPrefix(endpoint, "https://")
	} else {
		endpoint = strings.TrimPrefix(endpoint, "http://")
	}

	options := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
	if insecure {
		options = append(options, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, options...)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceNameKey.String(cfg.ServiceName),
		semconv.ServiceVersionKey.String(cfg.Version),
		semconv.ServiceInstanceIDKey.String(cfg.Commit),
	))
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return provider.Shutdown, nil
}
