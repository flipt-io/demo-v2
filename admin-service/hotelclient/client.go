package hotelclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// HotelInfo represents a hotel entity
type HotelInfo struct {
	ID             string `json:"id"`
	AvailableRooms int    `json:"available_rooms"`
}

// Booking represents a booking entity
type Booking struct {
	BookingID          string  `json:"booking_id"`
	HotelID            string  `json:"hotel_id"`
	Status             string  `json:"status"`
	ConfirmationNumber *string `json:"confirmation_number,omitempty"`
	TotalPrice         float64 `json:"total_price"`
	GuestName          string  `json:"guest_name"`
	GuestEmail         string  `json:"guest_email"`
	Checkin            string  `json:"checkin"`
	Checkout           string  `json:"checkout"`
	Guests             int     `json:"guests"`
}

// BookingsResponse represents the response from the bookings list endpoint
type BookingsResponse struct {
	Bookings []Booking `json:"bookings"`
	Total    int       `json:"total"`
}

// BookingUpdateRequest represents a booking update request
type BookingUpdateRequest struct {
	Status             string  `json:"status,omitempty"`
	ConfirmationNumber *string `json:"confirmation_number,omitempty"`
}

// Client is a client for the hotel service
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new hotel service client
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
	}
}

// GetBookings fetches bookings with optional status filter
func (c *Client) GetBookings(ctx context.Context, status string) ([]Booking, error) {
	url := fmt.Sprintf("%s/api/bookings", c.baseURL)
	if status != "" {
		url = fmt.Sprintf("%s?status=%s", url, status)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result BookingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Bookings, nil
}

// GetBooking fetches a specific booking by ID
func (c *Client) GetBooking(ctx context.Context, bookingID string) (*Booking, error) {
	url := fmt.Sprintf("%s/api/bookings/%s", c.baseURL, bookingID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("booking not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var booking Booking
	if err := json.NewDecoder(resp.Body).Decode(&booking); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &booking, nil
}

// UpdateBooking updates a booking
func (c *Client) UpdateBooking(ctx context.Context, bookingID string, update BookingUpdateRequest) error {
	url := fmt.Sprintf("%s/api/bookings/%s", c.baseURL, bookingID)

	body, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// GetHotelAvailability checks hotel availability for given dates and guests
func (c *Client) GetHotelAvailability(ctx context.Context, hotelID, checkin, checkout string, guests int) (*HotelInfo, error) {
	url := fmt.Sprintf("%s/api/hotels/%s/availability?guests=%d&checkin=%s&checkout=%s", c.baseURL, hotelID, guests, checkin, checkout)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("hotel not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var hotel HotelInfo
	if err := json.NewDecoder(resp.Body).Decode(&hotel); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &hotel, nil
}
