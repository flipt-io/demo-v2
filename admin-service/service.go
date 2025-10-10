package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flipt-io/labs/admin-service/api"
	"github.com/flipt-io/labs/admin-service/hotelclient"
	sdk "go.flipt.io/flipt-client"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
	"go.opentelemetry.io/otel/trace"
)

var errAutoApprovalEnabled = errors.New("cannot manually approve/reject when auto-approval is enabled")

type AdminService struct {
	fliptClient     *sdk.Client
	hotelClient     *hotelclient.Client
	approvalCounter metric.Int64Counter
	viewCounter     metric.Int64Counter
}

var _ api.ServerInterface = (*AdminService)(nil)

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

func (s *AdminService) autoApprovalEnabled(ctx context.Context) bool {
	span := trace.SpanFromContext(ctx)
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

	span.AddEvent("feature_flag.evaluation", trace.WithAttributes(
		semconv.FeatureFlagKey(req.FlagKey),
		semconv.FeatureFlagResultVariant(strconv.FormatBool(result.Enabled)),
		semconv.FeatureFlagResultReasonKey.String(result.Reason),
	))

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

	span.AddEvent("feature_flag.evaluation", trace.WithAttributes(
		semconv.FeatureFlagKey(req.FlagKey),
		semconv.FeatureFlagResultVariant(approvalTier.VariantKey),
		semconv.FeatureFlagResultReasonKey.String(approvalTier.Reason),
	))

	return approvalTier.VariantKey, nil
}

func (s *AdminService) GetHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "healthy", "service": "admin-service"})
}

func (s *AdminService) GetApiBookings(w http.ResponseWriter, r *http.Request, params api.GetApiBookingsParams) {
	ctx, span := tracer.Start(r.Context(), "get_bookings")
	defer span.End()

	status := ""
	if params.Status != nil {
		status = string(*params.Status)
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

func (s *AdminService) GetApiBookingsBookingId(w http.ResponseWriter, r *http.Request, bookingID string) {
	ctx, span := tracer.Start(r.Context(), "get_booking")
	defer span.End()

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

func (s *AdminService) PostApiBookingsBookingIdApprove(w http.ResponseWriter, r *http.Request, bookingID string) {
	ctx, span := tracer.Start(r.Context(), "approve_booking")
	defer span.End()

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
	if s.autoApprovalEnabled(ctx) {
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

func (s *AdminService) PostApiBookingsBookingIdReject(w http.ResponseWriter, r *http.Request, bookingID string) {
	ctx, span := tracer.Start(r.Context(), "reject_booking")
	defer span.End()

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

	if s.autoApprovalEnabled(ctx) {
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

func (s *AdminService) GetApiFlags(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "get_flag_status")
	defer span.End()

	autoApprovalEnabled := s.autoApprovalEnabled(ctx)
	approvalTier, err := s.evaluateApprovalRules(ctx, &hotelclient.Booking{})
	if err != nil {
		log.Printf("Error evaluating approval-tier: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get flag status"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"auto_approval": map[string]any{
			"enabled": autoApprovalEnabled,
		},
		"approval_tier": map[string]any{
			"variant": approvalTier,
		},
	})
}

func (s *AdminService) processBooking(ctx context.Context, booking *hotelclient.Booking) error {
	ctx, span := tracer.Start(ctx, "process_booking")
	defer span.End()
	// Fetch hotel details to check available rooms using hotel client
	hotel, err := s.hotelClient.GetHotelAvailability(ctx, booking.HotelID, booking.Checkin, booking.Checkout, booking.Guests)
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

	confirmationNumber := fmt.Sprintf("CNF-%000000X", rand.Int64N(time.Now().Unix()))
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

	approvalType := "manually approved"
	if autoApproval {
		approvalType = "auto-approved"
	}
	log.Printf("Booking %s %s with confirmation %s", booking.BookingID, approvalType, confirmationNumber)
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

	rejectionType := "manually rejected"
	if autoApproval {
		rejectionType = "auto-rejected"
	}
	log.Printf("Booking %s %s: %s", booking.BookingID, rejectionType, reason)
	return nil
}
