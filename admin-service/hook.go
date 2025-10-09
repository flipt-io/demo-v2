package main

import (
	"context"

	sdk "go.flipt.io/flipt-client"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ sdk.Hook = (*FliptHook)(nil)

// FliptHook implements the Flipt SDK Hook interface for tracking evaluations
type FliptHook struct {
	requestCounter metric.Int64Counter
	resultsCounter metric.Int64Counter
	environment    string
	namespace      string
}

func NewFliptHook(environment, namespace string) *FliptHook {
	requestCounter, _ := meter.Int64Counter(
		"flipt_evaluations_requests_total",
		metric.WithDescription("Total number of Flipt evaluation requests"),
	)

	resultsCounter, _ := meter.Int64Counter(
		"flipt_evaluations_results_total",
		metric.WithDescription("Total number of Flipt evaluation results"),
	)

	return &FliptHook{
		requestCounter: requestCounter,
		resultsCounter: resultsCounter,
		environment:    environment,
		namespace:      namespace,
	}
}

func (h *FliptHook) Before(ctx context.Context, data sdk.BeforeHookData) {
	h.requestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("flipt_flag", data.FlagKey),
		attribute.String("flipt_environment", h.environment),
		attribute.String("flipt_namespace", h.namespace),
	))
}

func (h *FliptHook) After(ctx context.Context, data sdk.AfterHookData) {
	h.resultsCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("flipt_flag", data.FlagKey),
		attribute.String("flipt_environment", h.environment),
		attribute.String("flipt_namespace", h.namespace),
		attribute.String("flipt_value", data.Value),
		attribute.String("flipt_reason", data.Reason),
		attribute.String("flipt_flag_type", data.FlagType),
	))
}
