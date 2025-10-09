import logging
import random
from datetime import datetime
from typing import Optional, Dict

from fastapi import FastAPI, HTTPException, Query
from fastapi.middleware.cors import CORSMiddleware
from opentelemetry import trace, metrics

from config import settings
from telemetry import setup_telemetry
from models import (
    HotelSearchRequest,
    HotelSearchResponse,
    AvailabilityResponse,
    BookingRequest,
    BookingResponse,
    BookingUpdateRequest,
)
from data import (
    get_all_hotels,
    get_hotel_by_id,
    search_hotels,
    calculate_nights,
    calculate_price_with_strategy,
    generate_booking_id,
    generate_confirmation_number,
)
from flipt_service import flipt_service

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Create FastAPI app
app = FastAPI(
    title="Hotel Service API",
    description="Hotel availability and booking service with Flipt feature flags",
    version=settings.service_version,
    docs_url="/",
    redoc_url=None,
)

# Setup CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Setup OpenTelemetry
tracer, meter = setup_telemetry(app)

# Create custom metrics
search_counter = meter.create_counter(
    name="hotel_searches_total",
    description="Total number of hotel searches",
    unit="1",
)

availability_counter = meter.create_counter(
    name="hotel_availability_checks_total",
    description="Total number of availability checks",
    unit="1",
)

booking_counter = meter.create_counter(
    name="hotel_bookings_total",
    description="Total number of bookings",
    unit="1",
)

feature_flag_counter = meter.create_counter(
    name="feature_flag_evaluations_total",
    description="Total number of feature flag evaluations",
    unit="1",
)

price_strategy_histogram = meter.create_histogram(
    name="price_display_strategy_usage",
    description="Usage of different price display strategies",
    unit="1",
)

# In-memory bookings storage
bookings_storage: Dict[str, dict] = {}





@app.get("/health")
async def health():
    """Health check endpoint."""
    return {
        "status": "healthy",
        "timestamp": datetime.utcnow().isoformat(),
        "flipt_connected": flipt_service.client is not None,
    }


@app.get("/api/hotels", response_model=HotelSearchResponse)
async def search_hotels_endpoint(
    location: Optional[str] = Query(None, description="Location to search"),
    checkin: Optional[str] = Query(None, description="Check-in date (ISO format)"),
    checkout: Optional[str] = Query(None, description="Check-out date (ISO format)"),
    guests: int = Query(1, ge=1, le=10, description="Number of guests"),
    entity_id: str = Query("anonymous", description="User/entity ID for feature flags"),
):
    """
    Search for hotels based on criteria.
    Uses feature flags to control display and functionality.
    """
    with tracer.start_as_current_span("search_hotels") as span:
        span.set_attribute("location", location or "all")
        span.set_attribute("guests", guests)
        span.set_attribute("entity_id", entity_id)
        
        # Evaluate feature flags
        context = {
            "guests": str(guests),
            "has_checkin": str(checkin is not None),
        }
        
        price_strategy = flipt_service.get_price_display_strategy(entity_id, context)
        
        # Batch evaluate boolean flags for better performance
        batch_flags = flipt_service.evaluate_batch_boolean(
            flag_keys=["real-time-availability", "loyalty-program"],
            entity_id=entity_id,
            context=context,
            defaults={"real-time-availability": True, "loyalty-program": False}
        )
        real_time_avail = batch_flags["real-time-availability"]
        loyalty_enabled = batch_flags["loyalty-program"]
        
        span.set_attribute("feature.price_strategy", price_strategy)
        span.set_attribute("feature.real_time_availability", real_time_avail)
        span.set_attribute("feature.loyalty_program", loyalty_enabled)
        
        # Record metrics
        search_counter.add(1, {
            "location": location or "all",
            "has_dates": str(checkin is not None and checkout is not None),
        })
        
        feature_flag_counter.add(1, {
            "flag": "price-display-strategy",
            "value": price_strategy,
        })
        
        price_strategy_histogram.record(1, {"strategy": price_strategy})
        
        # Search hotels
        hotels = search_hotels(location)
        
        # Calculate prices based on strategy
        nights = calculate_nights(checkin, checkout) if checkin and checkout else 1
        
        hotels_data = []
        for hotel in hotels:
            hotel_dict = hotel.model_dump()
            
            # Calculate price with strategy
            price_info = calculate_price_with_strategy(
                hotel.base_price_per_night,
                nights,
                price_strategy
            )
            
            hotel_dict["price"] = price_info["display_price"]
            hotel_dict["price_label"] = price_info["label"]
            hotel_dict["price_breakdown"] = price_info["breakdown"]
            
            # Add loyalty discount if enabled
            if loyalty_enabled and hotel.rating >= 4.5:
                hotel_dict["loyalty_discount"] = 10  # 10% discount
                hotel_dict["loyalty_member_price"] = round(
                    price_info["display_price"] * 0.9, 2
                )
            
            # Add real-time availability if enabled
            if real_time_avail:
                # Simulate real-time check with slight variation
                hotel_dict["available_rooms"] = max(0, hotel.available_rooms - random.randint(0, 2))
                hotel_dict["last_updated"] = datetime.utcnow().isoformat()
            
            hotels_data.append(hotel_dict)
        
        logger.info(
            f"Search completed: {len(hotels_data)} hotels, "
            f"strategy={price_strategy}, loyalty={loyalty_enabled}"
        )
        
        return HotelSearchResponse(
            hotels=hotels_data,
            total_count=len(hotels_data),
            price_display_strategy=price_strategy,
            real_time_availability=real_time_avail,
            loyalty_program_enabled=loyalty_enabled,
        )


@app.get("/api/hotels/{hotel_id}/availability", response_model=AvailabilityResponse)
async def check_availability(
    hotel_id: str,
    checkin: str = Query(..., description="Check-in date (ISO format)"),
    checkout: str = Query(..., description="Check-out date (ISO format)"),
    guests: int = Query(1, ge=1, le=10),
    entity_id: str = Query("anonymous", description="User/entity ID"),
):
    """
    Check availability for a specific hotel.
    Uses feature flags to control availability display and booking options.
    """
    with tracer.start_as_current_span("check_availability") as span:
        span.set_attribute("hotel_id", hotel_id)
        span.set_attribute("entity_id", entity_id)
        
        # Get hotel
        hotel = get_hotel_by_id(hotel_id)
        if not hotel:
            raise HTTPException(status_code=404, detail="Hotel not found")
        
        # Evaluate feature flags
        context = {"hotel_category": hotel.category, "guests": str(guests)}
        
        price_strategy = flipt_service.get_price_display_strategy(entity_id, context)
        instant_booking = flipt_service.is_instant_booking_enabled(entity_id, context)
        
        span.set_attribute("feature.price_strategy", price_strategy)
        span.set_attribute("feature.instant_booking", instant_booking)
        
        # Record metrics
        availability_counter.add(1, {
            "hotel_id": hotel_id,
            "instant_booking": str(instant_booking),
        })
        
        # Calculate price
        nights = calculate_nights(checkin, checkout)
        price_info = calculate_price_with_strategy(
            hotel.base_price_per_night,
            nights,
            price_strategy
        )
        
        # Check availability (simplified)
        available = hotel.available_rooms > 0
        
        return AvailabilityResponse(
            hotel_id=hotel_id,
            available=available,
            available_rooms=hotel.available_rooms,
            price_per_night=hotel.base_price_per_night,
            total_price=price_info["display_price"],
            price_breakdown=price_info["breakdown"],
            instant_booking_available=instant_booking and available,
        )


@app.post("/api/hotels/{hotel_id}/book", response_model=BookingResponse)
async def book_hotel(
    hotel_id: str,
    booking: BookingRequest,
    entity_id: str = Query("anonymous", description="User/entity ID"),
):
    """
    Book a hotel.
    Uses feature flags to control booking flow and confirmation.
    """
    with tracer.start_as_current_span("book_hotel") as span:
        span.set_attribute("hotel_id", hotel_id)
        span.set_attribute("entity_id", entity_id)
        
        # Validate hotel exists
        hotel = get_hotel_by_id(hotel_id)
        if not hotel:
            raise HTTPException(status_code=404, detail="Hotel not found")
        
        if booking.hotel_id != hotel_id:
            raise HTTPException(status_code=400, detail="Hotel ID mismatch")
        
        # Evaluate feature flags
        context = {"hotel_category": hotel.category}
        
        instant_booking = flipt_service.is_instant_booking_enabled(entity_id, context)
        price_strategy = flipt_service.get_price_display_strategy(entity_id, context)
        
        span.set_attribute("feature.instant_booking", instant_booking)
        
        # Calculate price
        nights = calculate_nights(booking.checkin, booking.checkout)
        price_info = calculate_price_with_strategy(
            hotel.base_price_per_night,
            nights,
            price_strategy
        )
        
        # Create booking
        booking_id = generate_booking_id()
        
        # Instant booking gets immediate confirmation
        if instant_booking:
            status = "confirmed"
            confirmation = generate_confirmation_number()
        else:
            status = "pending"
            confirmation = None
        
        # Record metrics
        booking_counter.add(1, {
            "hotel_id": hotel_id,
            "status": status,
            "instant": str(instant_booking),
        })
        
        logger.info(
            f"Booking created: {booking_id}, hotel={hotel_id}, "
            f"status={status}, instant={instant_booking}"
        )
        
        # Store booking in memory
        # Get total price from breakdown if available, otherwise use display_price
        if price_info["breakdown"] and "total" in price_info["breakdown"]:
            total_price = price_info["breakdown"]["total"]
        else:
            total_price = price_info["display_price"]
        
        booking_data = {
            "booking_id": booking_id,
            "hotel_id": hotel_id,
            "status": status,
            "confirmation_number": confirmation,
            "total_price": total_price,
            "guest_name": booking.guest_name,
            "guest_email": booking.guest_email,
            "checkin": booking.checkin,
            "checkout": booking.checkout,
            "guests": booking.guests,
            "created_at": datetime.utcnow(),
            "updated_at": datetime.utcnow(),
        }
        bookings_storage[booking_id] = booking_data
        
        return BookingResponse(
            booking_id=booking_id,
            hotel_id=hotel_id,
            status=status,
            confirmation_number=confirmation,
            total_price=booking_data["total_price"],
            created_at=datetime.utcnow(),
        )


@app.get("/api/bookings")
async def get_bookings(
    status: Optional[str] = Query(None, description="Filter by status (pending, confirmed, rejected)"),
):
    """
    Get all bookings, optionally filtered by status.
    Used by admin-service to retrieve unapproved bookings.
    """
    with tracer.start_as_current_span("get_bookings") as span:
        span.set_attribute("status", status or "all")
        
        # Filter by status if provided
        if status:
            filtered_bookings = [
                b for b in bookings_storage.values() 
                if b["status"] == status
            ]
        else:
            filtered_bookings = list(bookings_storage.values())
        
        # Create a copy and convert datetime to ISO string for JSON serialization
        serializable_bookings = []
        for booking in filtered_bookings:
            booking_copy = booking.copy()
            # Convert datetime objects to ISO strings if they aren't already
            if hasattr(booking_copy["created_at"], "isoformat"):
                booking_copy["created_at"] = booking_copy["created_at"].isoformat()
            if hasattr(booking_copy["updated_at"], "isoformat"):
                booking_copy["updated_at"] = booking_copy["updated_at"].isoformat()
            serializable_bookings.append(booking_copy)
        
        logger.info(f"Retrieved {len(serializable_bookings)} bookings with status={status}")
        
        return {
            "bookings": serializable_bookings,
            "total": len(serializable_bookings),
            "status": status or "all",
        }


@app.get("/api/bookings/{booking_id}")
async def get_booking(
    booking_id: str,
):
    """
    Get a specific booking by ID.
    Used by admin-service to retrieve a single booking.
    """
    with tracer.start_as_current_span("get_booking") as span:
        span.set_attribute("booking_id", booking_id)
        
        # Check if booking exists
        if booking_id not in bookings_storage:
            span.set_attribute("found", False)
            raise HTTPException(status_code=404, detail="Booking not found")
        
        booking = bookings_storage[booking_id]
        span.set_attribute("found", True)
        span.set_attribute("booking.status", booking["status"])
        
        # Create a copy and convert datetime to ISO string for JSON serialization
        booking_copy = booking.copy()
        if hasattr(booking_copy["created_at"], "isoformat"):
            booking_copy["created_at"] = booking_copy["created_at"].isoformat()
        if hasattr(booking_copy["updated_at"], "isoformat"):
            booking_copy["updated_at"] = booking_copy["updated_at"].isoformat()
        
        logger.info(f"Retrieved booking {booking_id}, status={booking['status']}")
        
        return booking_copy


@app.patch("/api/bookings/{booking_id}")
async def update_booking(
    booking_id: str,
    update_request: BookingUpdateRequest,
):
    """
    Update a booking's status and/or confirmation number.
    Used by admin-service to confirm or reject bookings.
    """
    with tracer.start_as_current_span("update_booking") as span:
        span.set_attribute("booking_id", booking_id)
        
        # Check if booking exists
        if booking_id not in bookings_storage:
            raise HTTPException(status_code=404, detail="Booking not found")
        
        booking = bookings_storage[booking_id]
        
        # Update fields if provided
        updated = False
        if update_request.status is not None:
            # Validate status
            valid_statuses = ["pending", "confirmed", "rejected"]
            if update_request.status not in valid_statuses:
                raise HTTPException(
                    status_code=400, 
                    detail=f"Invalid status. Must be one of: {', '.join(valid_statuses)}"
                )
            booking["status"] = update_request.status
            span.set_attribute("updated.status", update_request.status)
            updated = True
        
        if update_request.confirmation_number is not None:
            booking["confirmation_number"] = update_request.confirmation_number
            span.set_attribute("updated.confirmation_number", update_request.confirmation_number)
            updated = True
        
        if updated:
            booking["updated_at"] = datetime.utcnow()
            logger.info(
                f"Booking {booking_id} updated: status={booking['status']}, "
                f"confirmation={booking.get('confirmation_number')}"
            )
        
        # Serialize datetime for response
        booking_copy = booking.copy()
        if hasattr(booking_copy["created_at"], "isoformat"):
            booking_copy["created_at"] = booking_copy["created_at"].isoformat()
        if hasattr(booking_copy["updated_at"], "isoformat"):
            booking_copy["updated_at"] = booking_copy["updated_at"].isoformat()
        
        return booking_copy


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
