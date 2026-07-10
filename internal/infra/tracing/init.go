// Package tracing initializes OpenTelemetry tracing.
package tracing

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// Init creates and registers a global TracerProvider.
// If endpoint is empty, spans are exported to stdout for local development.
func Init(mode, endpoint, serviceName string) (*sdktrace.TracerProvider, error) {
	exporter, err := createExporter(endpoint)
	if err != nil {
		return nil, fmt.Errorf("create exporter: %w", err)
	}

	res, err := resource.New(context.Background(),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithProcess(),
		resource.WithHost(),
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	sampler := sdktrace.AlwaysSample()
	if mode != "development" {
		sampler = sdktrace.TraceIDRatioBased(0.1)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

func createExporter(endpoint string) (sdktrace.SpanExporter, error) {
	if endpoint != "" {
		exporter, err := otlptracehttp.New(context.Background(),
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("create otlp exporter: %w", err)
		}
		return exporter, nil
	}

	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("create stdout exporter: %w", err)
	}
	return exporter, nil
}

// Shutdown flushes and closes the TracerProvider.
func Shutdown(ctx context.Context, tp *sdktrace.TracerProvider) {
	if tp == nil {
		return
	}
	if err := tp.Shutdown(ctx); err != nil {
		log.Printf("[tracing] shutdown error: %v\n", err)
	}
}
