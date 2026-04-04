package main

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.flipt.io/flipt/rpc/flipt"
	fliptgrpc "go.flipt.io/flipt/rpc/flipt"
	sdk "go.flipt.io/flipt/sdk/go"
	flipthttp "go.flipt.io/flipt/sdk/go/http"
)

const (
	evalIntervalMs = 200
)

var (
	fliptHook *FliptHook
	flags     atomic.Value // []FlagInfo
)

type FlagInfo struct {
	Key  string
	Type flipt.FlagType
}

func fliptURL() string {
	return cmp.Or(os.Getenv("FLIPT_URL"), "http://localhost:8080")
}

func fliptEnvironment() string {
	return cmp.Or(os.Getenv("FLIPT_ENVIRONMENT"), "onoffinc")
}

func fliptNamespace() string {
	return cmp.Or(os.Getenv("FLIPT_NAMESPACE"), "default")
}

func fliptToken() string {
	return cmp.Or(os.Getenv("FLIPT_TOKEN"), "token")
}

func ofrepURL() string {
	return os.Getenv("OFREP_URL")
}

func ofrepAuthToken() string {
	return os.Getenv("OFREP_AUTH_TOKEN")
}

func numClientWorkers() int {
	if v := os.Getenv("NUM_CLIENT_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 1
}

func numServerWorkers() int {
	if v := os.Getenv("NUM_SERVER_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func fetchFlagsAndSegments(ctx context.Context) []FlagInfo {
	drt, err := newDNSRoundTripper(fliptURL(), map[string]string{"X-Flipt-Environment": fliptEnvironment()})
	if err != nil {
		log.Printf("Failed to create DNS round tripper: %v", err)
		return []FlagInfo{}
	}
	cl := &http.Client{
		Transport: drt,
	}
	transport := flipthttp.NewTransport(fliptURL(), flipthttp.WithHTTPClient(cl))
	fsdk := sdk.New(transport, sdk.WithAuthenticationProvider(sdk.StaticTokenAuthenticationProvider(fliptToken())))
	client := fsdk.Flipt()

	var flagInfos []FlagInfo
	flagList, err := client.ListFlags(ctx, &fliptgrpc.ListFlagRequest{
		NamespaceKey: fliptNamespace(),
	})
	if err != nil {
		log.Printf("Failed to list flags: %v", err)
		return flagInfos
	}

	for _, f := range flagList.Flags {
		flagInfos = append(flagInfos, FlagInfo{Key: f.Key, Type: f.Type})
	}

	if len(flagInfos) == 0 {
		flagInfos = []FlagInfo{{Key: "flag-chaosmonkey", Type: flipt.FlagType_VARIANT_FLAG_TYPE}}
	}

	log.Printf("Fetched %d flags", len(flagInfos))
	return flagInfos
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	setupPrometheus(ctx)

	fliptHook = NewFliptHook(fliptEnvironment(), fliptNamespace())

	flags.Store(fetchFlagsAndSegments(ctx))

	go func() {
		refreshTicker := time.NewTicker(2 * time.Minute)
		defer refreshTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-refreshTicker.C:
				flags.Store(fetchFlagsAndSegments(ctx))
			}
		}
	}()

	log.Printf("Starting %d Flipt clients with streaming mode...", numClientWorkers())

	var wg sync.WaitGroup

	for i := range numClientWorkers() {
		cctx := context.WithValue(ctx, "clientid", fmt.Sprintf("cw-%d", i))
		w, err := newClientWorker(cctx, i)
		if err != nil {
			log.Printf("Client Worker %d: Failed to create client: %v", i, err)
			continue
		}
		log.Printf("Client Worker %d: Created", i)
		wg.Go(func() {
			w.runClientWorker(cctx)
		})
	}

	numServer := numServerWorkers()
	if numServer == 0 {
		log.Println("No server workers configured (set NUM_SERVER_WORKERS to enable OFREP)")
	} else if ofrepURL() != "" {
		log.Printf("Starting %d OpenFeature OFREP clients (server-side)...", numServer)
		for i := range numServer {
			sw, err := newServerWorker(i+numClientWorkers(), ofrepURL(), ofrepAuthToken())
			if err != nil {
				log.Printf("Server Worker %d: Failed to create: %v", i, err)
				continue
			}
			wg.Go(func() {
				sw.runServerWorker(ctx)
			})
		}
	} else {
		log.Printf("Warning: NUM_SERVER_WORKERS set but OFREP_URL not configured")
	}

	wg.Wait()
	log.Println("All workers stopped")
}
