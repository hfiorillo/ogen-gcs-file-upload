package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type OpenTelemetry struct {
	TracerProvider *trace.TracerProvider
	MeterProvider  *metric.MeterProvider
	LoggerProvider *log.LoggerProvider
	Propagator     propagation.TextMapPropagator
	Shutdown       func(context.Context) error
}

// createDefaultResource creates a resource with service details
func createDefaultResource(serviceName, version string) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
}

// createTraceProvider sets up and returns a trace provider
func createTraceProvider(ctx context.Context, res *resource.Resource) (*trace.TracerProvider, error) {
	// Create trace exporter
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create tracer provider
	tracerProvider := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(
			traceExporter,
			trace.WithBatchTimeout(5*time.Second),
			trace.WithMaxExportBatchSize(500),
			trace.WithMaxQueueSize(1000),
		),
	)

	return tracerProvider, nil
}

// createMeterProvider sets up and returns a meter provider
func createMeterProvider(ctx context.Context, res *resource.Resource) (*metric.MeterProvider, error) {
	// Create metric exporter
	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Create meter provider
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(
			metric.NewPeriodicReader(metricExporter),
		),
	)

	return meterProvider, nil
}

// createLoggerProvider sets up and returns a logger provider
func createLoggerProvider(res *resource.Resource) (*log.LoggerProvider, error) {
	// Create log exporter
	logExporter, err := stdoutlog.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create log exporter: %w", err)
	}

	loggerProvider := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(
			log.NewBatchProcessor(
				logExporter,
				log.WithMaxQueueSize(1000),
				log.WithExportTimeout(30*time.Second),
			),
		),
	)

	return loggerProvider, nil
}

// createPropagator creates a composite text map propagator
func createPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// SetupOTelSDK sets up OpenTelemetry providers
func SetupOTelSDK(ctx context.Context, serviceName string, version string) (*OpenTelemetry, error) {
	// Create default resources
	res, err := createDefaultResource(serviceName, version)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create trace provider
	tracerProvider, err := createTraceProvider(ctx, res)
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(tracerProvider)

	// Create meter provider
	meterProvider, err := createMeterProvider(ctx, res)
	if err != nil {
		return nil, err
	}
	otel.SetMeterProvider(meterProvider)

	// Create logger provider
	loggerProvider, err := createLoggerProvider(res)
	if err != nil {
		return nil, err
	}
	global.SetLoggerProvider(loggerProvider)

	// Create propagator
	propagator := createPropagator()
	otel.SetTextMapPropagator(propagator)

	// Create shutdown function
	shutdown := func(ctx context.Context) error {
		var err error

		if shutdownErr := tracerProvider.Shutdown(ctx); shutdownErr != nil {
			err = fmt.Errorf("failed to shutdown tracer provider: %w", shutdownErr)
		}

		if shutdownErr := meterProvider.Shutdown(ctx); shutdownErr != nil {
			err = fmt.Errorf("%v; failed to shutdown meter provider: %w", err, shutdownErr)
		}

		if shutdownErr := loggerProvider.Shutdown(ctx); shutdownErr != nil {
			err = fmt.Errorf("%v; failed to shutdown logger provider: %w", err, shutdownErr)
		}

		return err
	}

	// handleErr := func(inErr error) {
	// 	err = errors.Join(inErr, shutdown(ctx))
	// }

	return &OpenTelemetry{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
		LoggerProvider: loggerProvider,
		Propagator:     propagator,
		Shutdown:       shutdown,
	}, nil
}
