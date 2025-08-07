package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/sdk/trace"
)

func Init(ctx context.Context, endpoint, serviceName string, insecure bool) (func(context.Context) error, error) {
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	clientOpts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
	if insecure {
		clientOpts = append(clientOpts, otlptracehttp.WithInsecure())
	}
	exp, err := otlptracehttp.New(ctx, clientOpts...)
	if err != nil {
		return nil, err
	}
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exp, trace.WithBatchTimeout(3*time.Second)),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
