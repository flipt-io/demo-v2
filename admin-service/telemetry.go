package main

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

func setupOTEL(ctx context.Context) func() {
	// Create resource
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
	)
	if err != nil {
		log.Printf("Failed to create resource: %v", err)
	}

	// Setup trace provider
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		log.Printf("Failed to create trace exporter: %v", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	// Setup metric provider
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		log.Printf("Failed to create metric exporter: %v", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			metric.WithInterval(10*time.Second))),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	// Setup propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Println("OpenTelemetry initialized successfully")

	// Return shutdown function
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
		if err := meterProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down meter provider: %v", err)
		}
	}
}
