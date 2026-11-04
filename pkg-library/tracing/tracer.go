package tracing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Config struct {
	ServiceName string
	Version     string
	Env         string

	Endpoint string // host:port, no scheme, e.g. "otel-collector:4317"
	Insecure bool   // plaintext gRPC (in-cluster)

	// SampleRatio in (0,1) samples that fraction of NEW traces.
	// Zero or >=1 samples everything. Parent decisions are always respected.
	SampleRatio float64

	ExportTimeout time.Duration // per export attempt; default 5s
}

func Init(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, error) {
	if cfg.ServiceName == "" {
		return nil, errors.New("tracing: ServiceName is required")
	}
	if cfg.Endpoint == "" {
		return nil, errors.New("tracing: Endpoint is required")
	}
	if cfg.ExportTimeout <= 0 {
		cfg.ExportTimeout = 5 * time.Second
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("tracing: failed to create resource: %w", err)
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithTimeout(cfg.ExportTimeout),
		otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{
			Enabled:         true,
			InitialInterval: 500 * time.Millisecond,
			MaxInterval:     5 * time.Second,
			MaxElapsedTime:  15 * time.Second, // bounded: give up and drop the batch
		}),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exp, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("tracing: failed to create OTLP exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp, sdktrace.WithExportTimeout(cfg.ExportTimeout)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(newSampler(cfg.SampleRatio)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, // W3C Trace Context
			propagation.Baggage{}),
	)

	return tp, nil
}

func InitTracer(serviceName, endpoint string) (*sdktrace.TracerProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return Init(ctx, Config{
		ServiceName: serviceName,
		Endpoint:    endpoint,
		Insecure:    true,
	})
}

func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{attribute.String("service.name", cfg.ServiceName)}
	if cfg.Version != "" {
		attrs = append(attrs, attribute.String("service.version", cfg.Version))
	}
	if cfg.Env != "" {
		attrs = append(attrs, attribute.String("deployment.environment", cfg.Env))
	}
	return resource.New(
		ctx,
		resource.WithTelemetrySDK(),
		resource.WithAttributes(attrs...),
		resource.WithFromEnv(),
	)
}

func newSampler(ratio float64) sdktrace.Sampler {
	if ratio > 0 && ratio < 1 {
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	}
	return sdktrace.ParentBased(sdktrace.AlwaysSample())
}
