from datetime import datetime
from typing import Optional
from pydantic import BaseModel, Field


class Hotel(BaseModel):
    """Hotel model."""
    id: str
    name: str
    location: str
    description: str
    rating: float = Field(ge=0, le=5)
    base_price_per_night: float
    amenities: list[str]
    image_url: str
    available_rooms: int
    category: str  # economy, standard, premium, luxury


class HotelSearchRequest(BaseModel):
    """Hotel search request."""
    location: str
    checkin: str  # ISO date string
    checkout: str  # ISO date string
    guests: int = 1


class HotelSearchResponse(BaseModel):
    """Hotel search response."""
    hotels: list[dict]
    total_count: int
    price_display_strategy: str
    real_time_availability: bool
    loyalty_program_enabled: bool


class AvailabilityResponse(BaseModel):
    """Hotel availability response."""
    hotel_id: str
    available: bool
    available_rooms: int
    price_per_night: float
    total_price: float
    price_breakdown: Optional[dict] = None
    instant_booking_available: bool


class BookingRequest(BaseModel):
    """Booking request."""
    hotel_id: str
    checkin: str
    checkout: str
    guests: int
    guest_name: str
    guest_email: str


class BookingResponse(BaseModel):
    """Booking response."""
    booking_id: str
    hotel_id: str
    status: str
    confirmation_number: Optional[str] = None
    total_price: float
    created_at: datetime


class BookingUpdateRequest(BaseModel):
    """Booking update request for PATCH endpoint."""
    status: Optional[str] = Field(None, description="Booking status (pending, confirmed, rejected)")
    confirmation_number: Optional[str] = Field(None, description="Confirmation number")
