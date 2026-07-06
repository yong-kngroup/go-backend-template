// Package tracing 负责 OpenTelemetry 链路追踪的初始化。
package tracing

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Init 初始化 OpenTelemetry TracerProvider。
//
//	若 endpoint 非空 → 通过 OTLP HTTP 发送到 Jaeger。
//	若 endpoint 为空 → 退回到 stdout（开发便利模式）。
//
// 返回的 TracerProvider 已注册为全局。调用方需在程序退出前执行 Shutdown。
func Init(mode, endpoint string) (*sdktrace.TracerProvider, error) {
	exporter, err := createExporter(endpoint)
	if err != nil {
		return nil, fmt.Errorf("create exporter: %w", err)
	}

	sampler := sdktrace.AlwaysSample()
	if mode != "development" {
		sampler = sdktrace.TraceIDRatioBased(0.1)
	}

	tp := sdktrace.NewTracerProvider(
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

	// 退化到 stdout（本地开发没起 Jaeger 也能用）
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("create stdout exporter: %w", err)
	}
	return exporter, nil
}

// Shutdown 优雅关闭 TracerProvider，flush 所有未发送的 span。
func Shutdown(ctx context.Context, tp *sdktrace.TracerProvider) {
	if tp == nil {
		return
	}
	if err := tp.Shutdown(ctx); err != nil {
		log.Printf("[tracing] shutdown error: %v\n", err)
	}
}
