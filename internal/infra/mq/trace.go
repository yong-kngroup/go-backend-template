package mq

import (
	"context"
	"encoding/json"
	"strings"

	pkgkafka "github.com/freeDog-wy/go-backend-template/pkg/kafka"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const traceIDHeader = "trace_id"

type ctxKey struct{}

var mqTracer = otel.Tracer("github.com/freeDog-wy/go-backend-template/internal/infra/mq")

type kafkaHeaderCarrier struct {
	headers *[]pkgkafka.Header
}

func (c kafkaHeaderCarrier) Get(key string) string {
	for _, header := range *c.headers {
		if strings.EqualFold(header.Key, key) {
			return string(header.Value)
		}
	}
	return ""
}

func (c kafkaHeaderCarrier) Set(key, value string) {
	for i := range *c.headers {
		if strings.EqualFold((*c.headers)[i].Key, key) {
			(*c.headers)[i].Key = key
			(*c.headers)[i].Value = []byte(value)
			return
		}
	}
	*c.headers = append(*c.headers, pkgkafka.Header{Key: key, Value: []byte(value)})
}

func (c kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(*c.headers))
	for _, header := range *c.headers {
		keys = append(keys, header.Key)
	}
	return keys
}

func TraceIDFromContext(ctx context.Context) string {
	if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
		return spanContext.TraceID().String()
	}
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}

func TraceIDFromHeaders(headers []pkgkafka.Header) string {
	if traceID := strings.TrimSpace(pkgkafka.HeaderValue(headers, traceIDHeader)); traceID != "" {
		return traceID
	}
	return TraceIDFromContext(ExtractTraceContext(context.Background(), headers))
}

func ExtractTraceContext(ctx context.Context, headers []pkgkafka.Header) context.Context {
	extracted := otel.GetTextMapPropagator().Extract(ctx, kafkaHeaderCarrier{headers: &headers})
	if traceID := strings.TrimSpace(pkgkafka.HeaderValue(headers, traceIDHeader)); traceID != "" {
		extracted = context.WithValue(extracted, ctxKey{}, traceID)
	}
	return extracted
}

func InjectTraceContext(ctx context.Context, headers []pkgkafka.Header) []pkgkafka.Header {
	cloned := append([]pkgkafka.Header(nil), headers...)
	otel.GetTextMapPropagator().Inject(ctx, kafkaHeaderCarrier{headers: &cloned})
	return cloned
}

func SerializeTraceContext(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if len(carrier) == 0 {
		return ""
	}

	payload, err := json.Marshal(map[string]string(carrier))
	if err != nil {
		return ""
	}
	return string(payload)
}

func ContextWithSerializedTraceContext(ctx context.Context, serialized string) context.Context {
	if strings.TrimSpace(serialized) == "" {
		return ctx
	}

	carrier := propagation.MapCarrier{}
	if err := json.Unmarshal([]byte(serialized), &carrier); err != nil {
		return ctx
	}
	if len(carrier) == 0 {
		return ctx
	}

	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

func SerializeHeadersTraceContext(headers []pkgkafka.Header) string {
	return SerializeTraceContext(ExtractTraceContext(context.Background(), headers))
}
