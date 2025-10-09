package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

var errAutoApprovalEnabled = errors.New("cannot manually approve/reject when auto-approval is enabled")
var (
	tracer trace.Tracer
	meter  metric.Meter
)

type AdminService struct {
	fliptClient     *sdk.Client
	hotelClient     *hotelclient.Client
	approvalCounter metric.Int64Counter
	viewCounter     metric.Int64Counter
}

func NewAdminService(fliptClient *sdk.Client, hotelClient *hotelclient.Client) *AdminService {
	viewCounter, _ := meter.Int64Counter(
		"admin_booking_views_total",
		metric.WithDescription("Total number of booking views"),
	)

	approvalCounter, _ := meter.Int64Counter(
		"admin_booking_approvals_total",
		metric.WithDescription("Total number of booking approvals"),
	)

	service := &AdminService{
		fliptClient:     fliptClient,
		hotelClient:     hotelClient,
		viewCounter:     viewCounter,
		approvalCounter: approvalCounter,
	}

	return service
}

func (s *AdminService) AutoApprovalEnabled(ctx context.Context) bool {
	req := &sdk.EvaluationRequest{
		FlagKey:  "auto-approval",
		EntityID: "worker",
		Context:  map[string]string{},
	}
	result, err := s.fliptClient.EvaluateBoolean(ctx, req)
	if err != nil {
		log.Printf("Error evaluating auto_approval flag: %v", err)
		return false
	}
	return result.Enabled
}

func (s *AdminService) evaluateApprovalRules(ctx context.Context, booking *hotelclient.Booking) (string, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("booking_id", booking.BookingID),
		attribute.String("hotel_id", booking.HotelID),
		attribute.Float64("total_price", booking.TotalPrice),
	)

	req := &sdk.EvaluationRequest{
		FlagKey:  "approval-tier",
		EntityID: booking.GuestEmail,
		Context: map[string]string{
			"hotel_id":    booking.HotelID,
			"total_price": fmt.Sprintf("%.2f", booking.TotalPrice),
		},
	}
	approvalTier, err := s.fliptClient.EvaluateVariant(ctx, req)
	if err != nil {
		log.Printf("Error evaluating approval-tier flag: %v", err)
		span.RecordError(err)
		return "", err
	}

	return approvalTier.VariantKey, nil
}

func (s *AdminService) GetBookings(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "get_bookings")
	defer span.End()

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	span.SetAttributes(attribute.String("status_filter", status))

	// Fetch bookings from hotel-service using client
	bookings, err := s.getBookings(ctx, status)
	if err != nil {
		log.Printf("Error fetching bookings from hotel-service: %v", err)
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch bookings"})
		return
	}

	log.Printf("Retrieved %d bookings with status=%s from hotel-service", len(bookings), status)

	respondJSON(w, http.StatusOK, map[string]any{
		"bookings": bookings,
		"total":    len(bookings),
		"status":   status,
	})
}

func (s *AdminService) getBookings(ctx context.Context, status string) ([]hotelclient.Booking, error) {
	bookings, err := s.hotelClient.GetBookings(ctx, status)
	if err != nil {
		return nil, err
	}

	s.viewCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", status),
		attribute.Int("count", len(bookings)),
	))
	return bookings, nil
}

func (s *AdminService) GetBooking(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "get_booking")
	defer span.End()

	bookingID := r.PathValue("booking_id")
	span.SetAttributes(attribute.String("booking_id", bookingID))

	// Fetch specific booking from hotel-service using client
	booking, err := s.hotelClient.GetBooking(ctx, bookingID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			span.SetAttributes(attribute.Bool("found", false))
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Booking not found"})
			return
		}
		log.Printf("Error fetching booking from hotel-service: %v", err)
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}

	span.SetAttributes(attribute.Bool("found", true))
	s.viewCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("booking_id", bookingID),
	))

	respondJSON(w, http.StatusOK, booking)
}

func (s *AdminService) ApproveBooking(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "approve_booking")
	defer span.End()

	bookingID := r.PathValue("booking_id")
	span.SetAttributes(attribute.String("booking_id", bookingID))

	// Fetch the specific booking from hotel-service using client
	booking, err := s.hotelClient.GetBooking(ctx, bookingID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			span.SetAttributes(attribute.Bool("found", false))
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Booking not found"})
			return
		}
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}
	if s.AutoApprovalEnabled(ctx) {
		span.RecordError(errAutoApprovalEnabled)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": errAutoApprovalEnabled.Error()})
		return
	}

	err = s.approveBooking(ctx, booking, false)
	if err != nil {
		log.Printf("Hotel service error when updating booking: %v", err)
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to confirm booking"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"booking_id": bookingID,
		"status":     "confirmed",
		"message":    "Booking approved and confirmed successfully",
	})
}

func (s *AdminService) RejectBooking(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "reject_booking")
	defer span.End()

	bookingID := r.PathValue("booking_id")
	span.SetAttributes(attribute.String("booking_id", bookingID))

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	// Fetch specific booking from hotel-service to verify it exists and check status
	booking, err := s.hotelClient.GetBooking(ctx, bookingID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			span.SetAttributes(attribute.Bool("found", false))
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Booking not found"})
			return
		}
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}

	if s.AutoApprovalEnabled(ctx) {
		span.RecordError(errAutoApprovalEnabled)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": errAutoApprovalEnabled.Error()})
		return
	}

	err = s.rejectBooking(ctx, booking, req.Reason, false)
	if err != nil {
		log.Printf("Hotel service error when updating booking: %v", err)
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to reject booking"})
		return
	}

	span.SetAttributes(
		attribute.String("reason", req.Reason),
	)

	respondJSON(w, http.StatusOK, map[string]any{
		"booking_id": bookingID,
		"status":     "rejected",
		"message":    "Booking rejected successfully",
		"reason":     req.Reason,
	})
}

func (s *AdminService) GetFlagStatus(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "get_flag_status")
	defer span.End()

	// Get current flag status for admin features
	entityID := r.URL.Query().Get("entity_id")
	if entityID == "" {
		entityID = "admin"
	}

	req := &sdk.EvaluationRequest{
		FlagKey:  "auto-approval",
		EntityID: entityID,
		Context:  map[string]string{},
	}
	autoApproval, err := s.fliptClient.EvaluateBoolean(ctx, req)
	if err != nil {
		log.Printf("Error evaluating auto-approval: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get flag status"})
		return
	}

	req = &sdk.EvaluationRequest{
		FlagKey:  "approval-tier",
		EntityID: entityID,
		Context:  map[string]string{},
	}
	approvalTier, err := s.fliptClient.EvaluateVariant(ctx, req)
	if err != nil {
		log.Printf("Error evaluating approval-tier: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get flag status"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"auto_approval": map[string]any{
			"enabled": autoApproval.Enabled,
			"reason":  autoApproval.Reason,
		},
		"approval_tier": map[string]any{
			"variant": approvalTier.VariantKey,
			"reason":  approvalTier.Reason,
		},
	})
}

func (s *AdminService) processBooking(ctx context.Context, booking *hotelclient.Booking) error {
	// Fetch hotel details to check available rooms using hotel client
	hotel, err := s.hotelClient.GetHotelAvialibility(ctx, booking.HotelID, booking.Checkin, booking.Checkout, booking.Guests)
	if err != nil {
		log.Printf("Error fetching hotel %s: %v", booking.HotelID, err)
		return err
	}

	// Check if hotel has available rooms
	if hotel.AvailableRooms > 0 {
		log.Printf("Approving booking %s - hotel %s has %d available rooms", booking.BookingID, hotel.ID, hotel.AvailableRooms)
		return s.approveBooking(ctx, booking, true)
	}

	log.Printf("Rejecting booking %s - hotel %s has no available rooms", booking.BookingID, hotel.ID)
	return s.rejectBooking(ctx, booking, "No rooms available", true)
}

func (s *AdminService) approveBooking(ctx context.Context, booking *hotelclient.Booking, autoApproval bool) error {
	if booking.Status != "pending" {
		return fmt.Errorf("booking is already %s", booking.Status)
	}

	// Evaluate approval rules using Flipt
	tier, err := s.evaluateApprovalRules(ctx, booking)
	if err != nil {
		return err
	}

	confirmationNumber := fmt.Sprintf("CNF-%s-%d", strings.ToUpper(booking.BookingID[:8]), time.Now().Unix()%10000)
	err = s.hotelClient.UpdateBooking(ctx, booking.BookingID, hotelclient.BookingUpdateRequest{
		Status:             "confirmed",
		ConfirmationNumber: &confirmationNumber,
	})
	if err != nil {
		return fmt.Errorf("failed to approve booking: %w", err)
	}

	s.approvalCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("booking_id", booking.BookingID),
		attribute.String("hotel_id", booking.HotelID),
		attribute.String("status", "approved"),
		attribute.String("tier", tier),
		attribute.Bool("auto_approval", autoApproval),
	))

	log.Printf("Booking %s auto-approved with confirmation %s", booking.BookingID, confirmationNumber)
	return nil
}

func (s *AdminService) rejectBooking(ctx context.Context, booking *hotelclient.Booking, reason string, autoApproval bool) error {
	if booking.Status != "pending" {
		return fmt.Errorf("booking is already %s", booking.Status)
	}

	err := s.hotelClient.UpdateBooking(ctx, booking.BookingID, hotelclient.BookingUpdateRequest{
		Status: "rejected",
	})
	if err != nil {
		return fmt.Errorf("failed to reject booking: %w", err)
	}

	s.approvalCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("booking_id", booking.BookingID),
		attribute.String("hotel_id", booking.HotelID),
		attribute.String("status", "rejected"),
		attribute.String("reason", reason),
		attribute.Bool("auto_approval", autoApproval),
	))

	log.Printf("Booking %s auto-rejected: %s", booking.BookingID, reason)
	return nil
}

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
	// Setup OpenTelemetry
	shutdown := setupTelemetry(ctx)
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

	// API routes
	mux.HandleFunc("GET /api/bookings", adminService.GetBookings)
	mux.HandleFunc("GET /api/bookings/{booking_id}", adminService.GetBooking)
	mux.HandleFunc("POST /api/bookings/{booking_id}/approve", adminService.ApproveBooking)
	mux.HandleFunc("POST /api/bookings/{booking_id}/reject", adminService.ApproveBooking)
	mux.HandleFunc("GET /api/flags", adminService.GetFlagStatus)

	// Apply middlewares
	handler := corsMiddleware(tracingMiddleware(mux))

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
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func setupTelemetry(ctx context.Context) func() {
	return setupOTEL(ctx)
}
