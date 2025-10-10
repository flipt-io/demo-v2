package main

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flipt-io/labs/admin-service/api"
	"github.com/flipt-io/labs/admin-service/hotelclient"
	sdk "go.flipt.io/flipt-client"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

//go:embed openapi.json
var openAPISpec []byte

var (
	tracer trace.Tracer
	meter  metric.Meter
)

// HTTP middleware for CORS
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HTTP middleware for OpenTelemetry tracing
func tracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context from headers
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Start a new span
		ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path)
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.route", r.URL.Path),
		)

		// Create a custom response writer to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Serve the request with traced context
		next.ServeHTTP(rw, r.WithContext(ctx))

		span.SetAttributes(attribute.Int("http.status_code", rw.statusCode))
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	shutdown := setupOTEL(ctx)
	defer shutdown()

	tracer = otel.Tracer("admin-service")
	meter = otel.Meter("admin-service")

	// Get configuration from environment
	fliptURL := getEnv("FLIPT_URL", "http://flipt:8080")
	namespace := getEnv("FLIPT_NAMESPACE", "default")
	environment := getEnv("FLIPT_ENVIRONMENT", "onoffinc")
	port := getEnv("PORT", "8001")
	hotelServiceURL := getEnv("HOTEL_SERVICE_URL", "http://hotel-service:8000")

	log.Printf("Starting Admin Service...")
	log.Printf("Flipt URL: %s", fliptURL)
	log.Printf("Namespace: %s", namespace)
	log.Printf("Environment: %s", environment)
	log.Printf("Hotel Service URL: %s", hotelServiceURL)

	// Create an HTTP client with OpenTelemetry instrumentation
	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   12 * time.Hour,
	}

	// Create Flipt hook for tracking evaluations
	fliptHook := NewFliptHook(environment, namespace)

	// Initialize Flipt client with streaming and instrumented HTTP client
	fliptClient, err := sdk.NewClient(
		ctx,
		sdk.WithURL(fliptURL),
		sdk.WithNamespace(namespace),
		sdk.WithEnvironment(environment),
		sdk.WithFetchMode(sdk.FetchModeStreaming),
		sdk.WithHTTPClient(httpClient),
		sdk.WithHook(fliptHook),
		sdk.WithErrorStrategy(sdk.ErrorStrategyFallback),
	)
	if err != nil {
		log.Fatalf("Failed to create Flipt client: %v", err)
	}
	defer fliptClient.Close(ctx)

	log.Println("Flipt client initialized with streaming enabled")

	// Create hotel service client
	hotelClient := hotelclient.NewClient(hotelServiceURL, httpClient)

	// Create admin service
	adminService := NewAdminService(fliptClient, hotelClient)

	// Create and start auto-approval worker
	worker := NewAutoApprovalWorker(adminService)
	go worker.Start(ctx)

	// Setup HTTP router
	mux := http.NewServeMux()

	// Root endpoint - Swagger UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin Service API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
                url: "/openapi.json",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
        };
    </script>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	})

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "healthy", "service": "admin-service"})
	})

	// OpenAPI spec endpoint
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(openAPISpec)
	})

	handler := api.HandlerFromMux(adminService, mux)

	// Apply middlewares
	handler = corsMiddleware(tracingMiddleware(handler))

	// Start server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("Admin Service started on port %s", port)

	// Wait for interrupt signal
	<-ctx.Done()

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}

func getEnv(key, defaultValue string) string {
	return cmp.Or(os.Getenv(key), defaultValue)
}
