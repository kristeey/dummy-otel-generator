package main

import (
	"context"
	reglog "log"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	metric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	trace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.9.0"
)

var (
	serviceName    = "dummy-otel-generator"
	serviceVersion = "0.0.1"
)

func initTracer(endpoint string) (*trace.TracerProvider, error) {
	// Set up tracer
	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, err
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(serviceName),
				semconv.ServiceVersionKey.String(serviceVersion),
			),
		),
		trace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(traceProvider)

	// Propagation refers to the process of passing trace context information across process boundaries.
	// The code ensures that both trace context and baggage information are propagated across service boundaries.
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
	return traceProvider, nil
}

func initMeter(endpoint string) (*metric.MeterProvider, error) {
	exporter, err := otlpmetrichttp.New(
		context.Background(),
		otlpmetrichttp.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, err
	}

	metricProvider := metric.NewMeterProvider(
		metric.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(serviceName),
				semconv.ServiceVersionKey.String(serviceVersion),
			),
		),
		metric.WithReader(
			metric.NewPeriodicReader(exporter),
		),
	)
	otel.SetMeterProvider(metricProvider)
	return metricProvider, nil
}

func initLogger(endpoint string) (*log.LoggerProvider, error) {
	exporter, err := otlploghttp.New(
		context.Background(),
		otlploghttp.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, err
	}

	loggerProvider := log.NewLoggerProvider(
		log.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(serviceName),
				semconv.ServiceVersionKey.String(serviceVersion),
			),
		),
		log.WithProcessor(log.NewBatchProcessor(exporter)),
	)
	return loggerProvider, nil
}

func main() {
	if len(os.Args) < 2 {
		reglog.Fatal("Usage: otel-app <endpoint>")
	}
	endpoint := os.Args[1]

	ctx := context.Background()

	traceProvider, err := initTracer(endpoint)
	if err != nil {
		reglog.Fatalf("failed to initialize tracer: %v", err)
	}
	defer traceProvider.Shutdown(ctx)

	metricProvider, err := initMeter(endpoint)
	if err != nil {
		reglog.Fatalf("failed to initialize meter: %v", err)
	}
	defer metricProvider.Shutdown(ctx)
	counter, _ := metricProvider.Meter("dummy-otel-app").Int64Counter("my_custom_counter")

	loggerProvider, err := initLogger(endpoint)
	if err != nil {
		reglog.Fatalf("failed to initialize logger: %v", err)
	}
	defer loggerProvider.Shutdown(ctx)
	//logger := loggerProvider.Logger("otel-app")
	loggerOptions := otelslog.WithLoggerProvider(loggerProvider)
	logger := otelslog.NewLogger("dummy-otel-app", loggerOptions)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx2, span := otel.Tracer("dummy-otel-app").Start(ctx, "IncrementCounter")
			counter.Add(ctx2, 1)
			reglog.Println("Info: Counter incremented")
			logger.InfoContext(ctx2, "Counter incremented")
			span.End()
		}
	}
}
