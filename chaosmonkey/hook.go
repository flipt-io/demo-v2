package main

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	sdk "go.flipt.io/flipt-client"
)

var _ sdk.Hook = (*FliptHook)(nil)

type FliptHook struct {
	environment string
	namespace   string
}

func NewFliptHook(environment, namespace string) *FliptHook {
	return &FliptHook{
		environment: environment,
		namespace:   namespace,
	}
}

func (h *FliptHook) Before(ctx context.Context, data sdk.BeforeHookData) {
	clientID := fmt.Sprintf("%v", ctx.Value("clientid"))
	FliptRequestsCounter.With(prometheus.Labels{
		"flipt_flag":        data.FlagKey,
		"flipt_environment": h.environment,
		"flipt_namespace":   h.namespace,
		"instance":          clientID,
	}).Inc()
}

func (h *FliptHook) After(ctx context.Context, data sdk.AfterHookData) {
	clientID := fmt.Sprintf("%v", ctx.Value("clientid"))
	FliptResultsCounter.With(prometheus.Labels{
		"flipt_flag":        data.FlagKey,
		"flipt_environment": h.environment,
		"flipt_namespace":   h.namespace,
		"flipt_value":       data.Value,
		"flipt_reason":      data.Reason,
		"flipt_flag_type":   data.FlagType,
		"instance":          clientID,
	}).Inc()
}
