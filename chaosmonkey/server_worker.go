package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"go.flipt.io/flipt/rpc/flipt"

	"github.com/open-feature/go-sdk-contrib/providers/ofrep"
	of "github.com/open-feature/go-sdk/openfeature"
)

type serverWorker struct {
	id       int
	ofClient *of.Client
}

func newServerWorker(id int, ofrepURL, authToken string) (*serverWorker, error) {
	drt, err := newDNSRoundTripper(ofrepURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create DNS round tripper: %w", err)
	}

	httpClient := &http.Client{Transport: drt}

	provider := ofrep.NewProvider(ofrepURL, ofrep.WithApiKeyAuth(authToken), ofrep.WithClient(httpClient))

	err = of.SetProviderAndWait(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to set OFREP provider: %w", err)
	}

	ofClient := of.NewClient("chaosmonkey-server")

	return &serverWorker{
		id:       id,
		ofClient: ofClient,
	}, nil
}

func (w *serverWorker) runServerWorker(ctx context.Context) {
	log.Printf("Server Worker %d: Started", w.id)

	ticker := time.NewTicker(time.Duration(evalIntervalMs+rand.Intn(100)) * time.Millisecond)
	defer ticker.Stop()

	entityID := fmt.Sprintf("entity-%d", w.id)
	for {
		select {
		case <-ctx.Done():
			log.Printf("Server Worker %d: Context cancelled", w.id)
			return
		case <-ticker.C:
			flagInfos := flags.Load().([]FlagInfo)
			if len(flagInfos) == 0 {
				continue
			}

			flagInfo := flagInfos[rand.Intn(len(flagInfos))]

			evalCtx := of.NewEvaluationContext(
				entityID,
				map[string]any{},
			)
			var err error
			if flagInfo.Type == flipt.FlagType_BOOLEAN_FLAG_TYPE {
				_, err = w.ofClient.BooleanValue(
					ctx,
					flagInfo.Key,
					false,
					evalCtx,
				)
			} else {
				_, err = w.ofClient.StringValue(
					ctx,
					flagInfo.Key,
					"",
					evalCtx,
				)
			}

			if err != nil {
				log.Printf("Server Worker %d: Evaluate error: %v", w.id, err)
			}
		}
	}
}
