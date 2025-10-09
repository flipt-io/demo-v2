package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	sdk "go.flipt.io/flipt-client"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

//go:embed openapi.json
var openAPISpec []byte

var (
	tracer trace.Tracer
	meter  metric.Meter
)

type Booking struct {
	BookingID          string    `json:"booking_id"`
	HotelID            string    `json:"hotel_id"`
	Status             string    `json:"status"`
	ConfirmationNumber *string   `json:"confirmation_number"`
	TotalPrice         float64   `json:"total_price"`
	GuestName          string    `json:"guest_name"`
	GuestEmail         string    `json:"guest_email"`
	Checkin            string    `json:"checkin"`
	Checkout           string    `json:"checkout"`
	Guests             int       `json:"guests"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type AdminService struct {
	fliptClient     *sdk.Client
	hotelServiceURL string
	approvalCounter metric.Int64Counter
	viewCounter     metric.Int64Counter
}

func NewAdminService(fliptClient *sdk.Client, hotelServiceURL string) *AdminService {
	approvalCounter, _ := meter.Int64Counter(
		"admin_booking_approvals_total",
		metric.WithDescription("Total number of booking approvals"),
	)

	viewCounter, _ := meter.Int64Counter(
		"admin_booking_views_total",
		metric.WithDescription("Total number of booking views"),
	)

	service := &AdminService{
		fliptClient:     fliptClient,
		hotelServiceURL: hotelServiceURL,
		approvalCounter: approvalCounter,
		viewCounter:     viewCounter,
	}

	return service
}

func (s *AdminService) evaluateApprovalRules(ctx context.Context, booking *Booking) (bool, string, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("booking_id", booking.BookingID),
		attribute.String("hotel_id", booking.HotelID),
		attribute.Float64("total_price", booking.TotalPrice),
	)

	// Evaluate auto-approval feature flag
	req := &sdk.EvaluationRequest{
		FlagKey:  "auto-approval",
		EntityID: booking.GuestEmail,
		Context: map[string]string{
			"hotel_id":    booking.HotelID,
			"total_price": fmt.Sprintf("%.2f", booking.TotalPrice),
			"guests":      fmt.Sprintf("%d", booking.Guests),
		},
	}
	autoApproval, err := s.fliptClient.EvaluateBoolean(ctx, req)
	if err != nil {
		log.Printf("Error evaluating auto-approval flag: %v", err)
		span.RecordError(err)
		return false, "", err
	}

	span.AddEvent("feature_flag.evaluation", trace.WithAttributes(
		semconv.FeatureFlagKey("auto-approval"),
		semconv.FeatureFlagResultVariant(strconv.FormatBool(autoApproval.Enabled)),
		semconv.FeatureFlagResultReasonKey.String(autoApproval.Reason),
		semconv.FeatureFlagContextID(booking.BookingID),
	))

	// Evaluate approval tier variant flag to determine review level
	req = &sdk.EvaluationRequest{
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
		return false, "", err
	}

	span.AddEvent("feature_flag.evaluation", trace.WithAttributes(
		semconv.FeatureFlagKey("approval-tier"),
		semconv.FeatureFlagResultVariant((approvalTier.VariantKey)),
		semconv.FeatureFlagResultReasonKey.String(approvalTier.Reason),
		semconv.FeatureFlagContextID(booking.BookingID),
	))

	return autoApproval.Enabled, approvalTier.VariantKey, nil
}

func (s *AdminService) GetBookings(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "get_bookings")
	defer span.End()

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	span.SetAttributes(attribute.String("status_filter", status))

	// Fetch bookings from hotel-service
	hotelServiceURL := fmt.Sprintf("%s/api/bookings?status=%s", s.hotelServiceURL, status)
	req, err := http.NewRequestWithContext(ctx, "GET", hotelServiceURL, nil)
	if err != nil {
		log.Printf("Error creating request to hotel-service: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch bookings"})
		return
	}

	// Propagate trace context
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error fetching bookings from hotel-service: %v", err)
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch bookings"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Hotel service returned status %d", resp.StatusCode)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch bookings from hotel service"})
		return
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Error decoding response: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to parse bookings"})
		return
	}

	bookings, ok := result["bookings"].([]any)
	if !ok {
		bookings = []any{}
	}

	s.viewCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", status),
		attribute.Int("count", len(bookings)),
	))

	log.Printf("Retrieved %d bookings with status=%s from hotel-service", len(bookings), status)

	respondJSON(w, http.StatusOK, map[string]any{
		"bookings": bookings,
		"total":    len(bookings),
		"status":   status,
	})
}

func (s *AdminService) GetBooking(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "get_booking")
	defer span.End()

	bookingID := strings.TrimPrefix(r.URL.Path, "/api/bookings/")
	span.SetAttributes(attribute.String("booking_id", bookingID))

	// Fetch specific booking from hotel-service
	hotelServiceURL := fmt.Sprintf("%s/api/bookings/%s", s.hotelServiceURL, bookingID)
	req, err := http.NewRequestWithContext(ctx, "GET", hotelServiceURL, nil)
	if err != nil {
		log.Printf("Error creating request to hotel-service: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error fetching booking from hotel-service: %v", err)
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		span.SetAttributes(attribute.Bool("found", false))
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Booking not found"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}

	var booking map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&booking); err != nil {
		log.Printf("Error decoding response: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to parse booking"})
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

	path := strings.TrimPrefix(r.URL.Path, "/api/bookings/")
	bookingID := strings.TrimSuffix(path, "/approve")
	span.SetAttributes(attribute.String("booking_id", bookingID))

	// Fetch the specific booking from hotel-service
	hotelServiceURL := fmt.Sprintf("%s/api/bookings/%s", s.hotelServiceURL, bookingID)
	req, err := http.NewRequestWithContext(ctx, "GET", hotelServiceURL, nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		span.SetAttributes(attribute.Bool("found", false))
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Booking not found"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}

	var bMap map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&bMap); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to parse booking"})
		return
	}

	// Parse booking data
	booking := &Booking{
		BookingID:  bookingID,
		HotelID:    bMap["hotel_id"].(string),
		Status:     bMap["status"].(string),
		TotalPrice: bMap["total_price"].(float64),
		GuestName:  bMap["guest_name"].(string),
		GuestEmail: bMap["guest_email"].(string),
		Checkin:    bMap["checkin"].(string),
		Checkout:   bMap["checkout"].(string),
		Guests:     int(bMap["guests"].(float64)),
	}

	if booking.Status != "pending" {
		span.SetAttributes(attribute.String("current_status", booking.Status))
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Booking is already %s", booking.Status)})
		return
	}

	// Evaluate approval rules using Flipt
	autoApprove, tier, err := s.evaluateApprovalRules(ctx, booking)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to evaluate approval rules"})
		return
	}

	span.SetAttributes(
		attribute.Bool("auto_approved", autoApprove),
		attribute.String("approval_tier", tier),
	)

	// Generate confirmation number
	confirmationNumber := fmt.Sprintf("CNF-%s-%d", strings.ToUpper(bookingID[:8]), time.Now().Unix()%10000)

	// Update booking status in hotel-service via PATCH
	updatePayload := map[string]string{
		"status":              "confirmed",
		"confirmation_number": confirmationNumber,
	}
	updateJSON, err := json.Marshal(updatePayload)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to prepare update"})
		return
	}

	patchURL := fmt.Sprintf("%s/api/bookings/%s", s.hotelServiceURL, bookingID)
	patchReq, err := http.NewRequestWithContext(ctx, "PATCH", patchURL, strings.NewReader(string(updateJSON)))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update booking"})
		return
	}
	patchReq.Header.Set("Content-Type", "application/json")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(patchReq.Header))

	patchResp, err := client.Do(patchReq)
	if err != nil {
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update booking status"})
		return
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode != http.StatusOK {
		log.Printf("Hotel service returned status %d when updating booking", patchResp.StatusCode)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to confirm booking"})
		return
	}

	s.approvalCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("booking_id", bookingID),
		attribute.Bool("auto_approved", autoApprove),
		attribute.String("tier", tier),
	))

	log.Printf("Booking approved and confirmed: %s, tier: %s, auto: %v, confirmation: %s", bookingID, tier, autoApprove, confirmationNumber)

	respondJSON(w, http.StatusOK, map[string]any{
		"booking_id":          bookingID,
		"auto_approved":       autoApprove,
		"approval_tier":       tier,
		"status":              "confirmed",
		"confirmation_number": confirmationNumber,
		"message":             "Booking approved and confirmed successfully",
	})
}

func (s *AdminService) RejectBooking(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "reject_booking")
	defer span.End()

	path := strings.TrimPrefix(r.URL.Path, "/api/bookings/")
	bookingID := strings.TrimSuffix(path, "/reject")
	span.SetAttributes(attribute.String("booking_id", bookingID))

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	// Fetch specific booking from hotel-service to verify it exists
	hotelServiceURL := fmt.Sprintf("%s/api/bookings/%s", s.hotelServiceURL, bookingID)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", hotelServiceURL, nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(httpReq.Header))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		span.SetAttributes(attribute.Bool("found", false))
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Booking not found"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch booking"})
		return
	}

	var booking map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&booking); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to parse booking"})
		return
	}

	status := booking["status"].(string)
	if status != "pending" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Booking is already %s", status)})
		return
	}

	// Update booking status in hotel-service via PATCH
	updatePayload := map[string]string{
		"status": "rejected",
	}
	updateJSON, err := json.Marshal(updatePayload)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to prepare update"})
		return
	}

	patchURL := fmt.Sprintf("%s/api/bookings/%s", s.hotelServiceURL, bookingID)
	patchReq, err := http.NewRequestWithContext(ctx, "PATCH", patchURL, strings.NewReader(string(updateJSON)))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update booking"})
		return
	}
	patchReq.Header.Set("Content-Type", "application/json")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(patchReq.Header))

	patchResp, err := client.Do(patchReq)
	if err != nil {
		span.RecordError(err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update booking status"})
		return
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode != http.StatusOK {
		log.Printf("Hotel service returned status %d when updating booking", patchResp.StatusCode)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to reject booking"})
		return
	}

	span.SetAttributes(
		attribute.String("reason", req.Reason),
	)

	log.Printf("Booking rejected: %s, reason: %s", bookingID, req.Reason)

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

	// Create admin service
	adminService := NewAdminService(fliptClient, hotelServiceURL)

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
	mux.HandleFunc("/api/bookings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			adminService.GetBookings(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/bookings/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/approve") {
			if r.Method == http.MethodPost {
				adminService.ApproveBooking(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		} else if strings.HasSuffix(path, "/reject") {
			if r.Method == http.MethodPost {
				adminService.RejectBooking(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		} else {
			if r.Method == http.MethodGet {
				adminService.GetBooking(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
	})

	mux.HandleFunc("/api/flags", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			adminService.GetFlagStatus(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

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
