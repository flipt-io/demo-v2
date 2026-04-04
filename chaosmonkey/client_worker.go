package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	fliptclient "go.flipt.io/flipt-client"
	"go.flipt.io/flipt/rpc/flipt"
)

type clientWorker struct {
	id          int
	client      *fliptclient.Client
	environment string
	namespace   string
	instance    string
}

func (c *clientWorker) EvaluateBoolean(ctx context.Context, req *fliptclient.EvaluationRequest) (*fliptclient.BooleanEvaluationResponse, error) {
	defer func(startTime time.Time) {
		c.recordEvaluationResult(startTime, req)
	}(time.Now())
	return c.client.EvaluateBoolean(ctx, req)
}

func (c *clientWorker) EvaluateVariant(ctx context.Context, req *fliptclient.EvaluationRequest) (*fliptclient.VariantEvaluationResponse, error) {
	defer func(startTime time.Time) {
		c.recordEvaluationResult(startTime, req)
	}(time.Now())

	return c.client.EvaluateVariant(ctx, req)
}

func (c *clientWorker) recordEvaluationResult(startTime time.Time, req *fliptclient.EvaluationRequest) {
	latency := time.Since(startTime).Milliseconds()
	EvaluationLatencyHistogram.With(prometheus.Labels{
		"flipt_flag":        req.FlagKey,
		"flipt_environment": c.environment,
		"flipt_namespace":   c.namespace,
		"instance":          c.instance,
	}).Observe(float64(latency))
}

func newClientWorker(ctx context.Context, id int) (*clientWorker, error) {
	w := &clientWorker{
		id:          id,
		environment: fliptEnvironment(),
		namespace:   fliptNamespace(),
		instance:    fmt.Sprintf("cw-%d", id),
	}

	client, err := fliptclient.NewClient(
		ctx,
		fliptclient.WithURL(fliptURL()),
		fliptclient.WithFetchMode(fliptclient.FetchModeStreaming),
		fliptclient.WithUpdateInterval(2*time.Minute),
		fliptclient.WithRequestTimeout(30*time.Second),
		fliptclient.WithErrorStrategy(fliptclient.ErrorStrategyFail),
		fliptclient.WithHook(fliptHook),
	)
	if err != nil {
		return nil, err
	}
	w.client = client
	return w, nil
}

func (w *clientWorker) runClientWorker(ctx context.Context) {
	defer func() {
		if w.client != nil {
			ctx, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			err := w.client.Close(ctx)
			if err != nil {
				log.Printf("Worker %d: Client close err: %v", w.id, err)
			}
		}
	}()

	log.Printf("Worker %d: Started", w.id)

	ticker := time.NewTicker(time.Duration(evalIntervalMs+rand.Intn(100)) * time.Millisecond)
	defer ticker.Stop()
	entityID := fmt.Sprintf("entity-%d", w.id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d: Context cancelled", w.id)
			return
		case <-ticker.C:
			simulateSlowClient()

			flagInfos := flags.Load().([]FlagInfo)

			flagInfo := flagInfos[rand.Intn(len(flagInfos))]

			context := map[string]string{
				"month": fmt.Sprintf("%d", rand.Intn(12)),
				"plan":  []string{"free", "pro", "enterprise"}[rand.Intn(3)],
			}
			var err error

			if flagInfo.Type == flipt.FlagType_BOOLEAN_FLAG_TYPE {
				_, err = w.EvaluateBoolean(ctx, &fliptclient.EvaluationRequest{
					FlagKey:  flagInfo.Key,
					EntityID: entityID,
					Context:  context,
				})
			} else {
				_, err = w.EvaluateVariant(ctx, &fliptclient.EvaluationRequest{
					FlagKey:  flagInfo.Key,
					EntityID: entityID,
					Context:  context,
				})
			}
			if err != nil {
				log.Printf("Worker %d: Evaluate error: %v", w.id, err)
			}
		}
	}
}

func simulateSlowClient() {
	delay := rand.Intn(50)
	if delay > 45 {
		sleepTime := time.Duration(rand.Intn(100)+50) * time.Millisecond
		time.Sleep(sleepTime)
	}
}
