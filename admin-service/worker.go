package main

import (
	"context"
	"log"
	"time"
)

type AutoApprovalWorker struct {
	svc          *AdminService
	pollInterval time.Duration
}

func NewAutoApprovalWorker(svc *AdminService) *AutoApprovalWorker {
	return &AutoApprovalWorker{
		svc:          svc,
		pollInterval: 10 * time.Second,
	}
}

func (w *AutoApprovalWorker) Start(ctx context.Context) {
	log.Println("Starting auto-approval worker...")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Auto-approval worker stopped")
			return
		case <-ticker.C:
			if w.svc.AutoApprovalEnabled(ctx) {
				log.Println("Auto-approval worker check - enabled")
				w.processBookings(ctx)
			}
		}
	}
}

func (w *AutoApprovalWorker) processBookings(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "worker_process_bookings")
	defer span.End()

	// Fetch pending bookings using hotel client
	bookings, err := w.svc.getBookings(ctx, "pending")
	if err != nil {
		log.Printf("Error fetching pending bookings: %v", err)
		span.RecordError(err)
		return
	}

	if len(bookings) == 0 {
		return
	}

	log.Printf("Processing %d pending bookings", len(bookings))

	for _, booking := range bookings {
		if err := w.svc.processBooking(ctx, &booking); err != nil {
			log.Printf("Error processing booking %s: %v", booking.BookingID, err)
		}
	}
}
