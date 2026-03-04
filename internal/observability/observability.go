package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config holds observability settings passed from cmd packages.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string

	TracingEnabled    bool
	TracingEndpoint   string
	TracingSampleRate float64
}

// Telemetry contains providers used by the API server/client.
type Telemetry struct {
	MeterProvider  *metric.MeterProvider
	TracerProvider *trace.TracerProvider
}

func New(cfg Config) (*Telemetry, error) {
	res, err := createResource(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	tracerProvider, err := createTracerProvider(cfg, res)
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(tracerProvider)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Telemetry{
		MeterProvider:  meterProvider,
		TracerProvider: tracerProvider,
	}, nil
}

func createResource(cfg Config) (*resource.Resource, error) {
	// Avoid schema conflicts when merging with auto-detected default resources.
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
}

func createTracerProvider(cfg Config, res *resource.Resource) (*trace.TracerProvider, error) {
	if !cfg.TracingEnabled {
		return newNoopTracerProvider(res), nil
	}

	endpoint := strings.TrimSpace(cfg.TracingEndpoint)
	if endpoint == "" {
		return newNoopTracerProvider(res), nil
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}
	if cfg.Environment != "production" {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(context.Background(), opts...)
	if err != nil {
		return newNoopTracerProvider(res), nil
	}

	sampleRate := cfg.TracingSampleRate
	switch {
	case sampleRate < 0:
		sampleRate = 0
	case sampleRate > 1:
		sampleRate = 1
	}

	return trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(sampleRate))),
	), nil
}

func newNoopTracerProvider(res *resource.Resource) *trace.TracerProvider {
	return trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithSampler(trace.NeverSample()),
	)
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	var shutdownErr error
	if t.TracerProvider != nil {
		shutdownErr = errors.Join(shutdownErr, t.TracerProvider.Shutdown(ctx))
	}
	if t.MeterProvider != nil {
		shutdownErr = errors.Join(shutdownErr, t.MeterProvider.Shutdown(ctx))
	}
	return shutdownErr
}
