from datetime import datetime
import random
import uuid
from models import Hotel


# Sample hotel data
HOTELS = [
    Hotel(
        id="hotel-1",
        name="Seaside Paradise Resort",
        location="Miami Beach, FL",
        description="Luxurious beachfront resort with stunning ocean views and world-class amenities.",
        rating=4.8,
        base_price_per_night=299.99,
        amenities=["Pool", "Beach Access", "Spa", "Restaurant", "WiFi", "Gym"],
        image_url="https://images.unsplash.com/photo-1520250497591-112f2f40a3f4?w=800",
        available_rooms=15,
        category="luxury"
    ),
    Hotel(
        id="hotel-2",
        name="Mountain View Lodge",
        location="Aspen, CO",
        description="Cozy mountain retreat perfect for ski enthusiasts and nature lovers.",
        rating=4.6,
        base_price_per_night=189.99,
        amenities=["Ski Access", "Fireplace", "Restaurant", "WiFi", "Hot Tub"],
        image_url="https://images.unsplash.com/photo-1566073771259-6a8506099945?w=800",
        available_rooms=8,
        category="premium"
    ),
    Hotel(
        id="hotel-3",
        name="Downtown Business Hotel",
        location="New York, NY",
        description="Modern hotel in the heart of Manhattan, perfect for business travelers.",
        rating=4.4,
        base_price_per_night=249.99,
        amenities=["Business Center", "WiFi", "Gym", "Restaurant", "Room Service"],
        image_url="https://images.unsplash.com/photo-1542314831-068cd1dbfeeb?w=800",
        available_rooms=22,
        category="standard"
    ),
    Hotel(
        id="hotel-4",
        name="Budget Inn Express",
        location="Orlando, FL",
        description="Affordable and comfortable accommodation near major attractions.",
        rating=4.0,
        base_price_per_night=79.99,
        amenities=["WiFi", "Parking", "Breakfast"],
        image_url="https://images.unsplash.com/photo-1551882547-ff40c63fe5fa?w=800",
        available_rooms=30,
        category="economy"
    ),
    Hotel(
        id="hotel-5",
        name="Historic City Center Inn",
        location="Boston, MA",
        description="Charming historic hotel with modern comforts in the heart of Boston.",
        rating=4.5,
        base_price_per_night=169.99,
        amenities=["WiFi", "Restaurant", "Bar", "Concierge"],
        image_url="https://images.unsplash.com/photo-1564501049412-61c2a3083791?w=800",
        available_rooms=12,
        category="standard"
    ),
    Hotel(
        id="hotel-6",
        name="Desert Oasis Spa Resort",
        location="Scottsdale, AZ",
        description="Luxury desert resort with championship golf and world-renowned spa.",
        rating=4.9,
        base_price_per_night=349.99,
        amenities=["Golf Course", "Spa", "Pool", "Restaurant", "WiFi", "Gym", "Tennis"],
        image_url="https://images.unsplash.com/photo-1571896349842-33c89424de2d?w=800",
        available_rooms=18,
        category="luxury"
    ),
    Hotel(
        id="hotel-7",
        name="Coastal Breeze Hotel",
        location="San Diego, CA",
        description="Relaxing beachside hotel with easy access to local attractions.",
        rating=4.3,
        base_price_per_night=159.99,
        amenities=["Beach Access", "Pool", "WiFi", "Parking"],
        image_url="https://images.unsplash.com/photo-1571003123894-1f0594d2b5d9?w=800",
        available_rooms=25,
        category="standard"
    ),
    Hotel(
        id="hotel-8",
        name="Urban Boutique Suites",
        location="Seattle, WA",
        description="Trendy boutique hotel in Seattle's vibrant downtown area.",
        rating=4.7,
        base_price_per_night=219.99,
        amenities=["WiFi", "Restaurant", "Bar", "Gym", "Rooftop Terrace"],
        image_url="https://images.unsplash.com/photo-1596436889106-be35e843f974?w=800",
        available_rooms=10,
        category="premium"
    ),
]


def get_all_hotels():
    """Get all hotels."""
    return [hotel.model_dump() for hotel in HOTELS]


def get_hotel_by_id(hotel_id: str):
    """Get hotel by ID."""
    for hotel in HOTELS:
        if hotel.id == hotel_id:
            return hotel
    return None


def search_hotels(location: str = None):
    """Search hotels by location."""
    if not location:
        return HOTELS
    
    # Simple case-insensitive search
    location_lower = location.lower()
    results = [
        hotel for hotel in HOTELS 
        if location_lower in hotel.location.lower()
    ]
    return results


def calculate_nights(checkin: str, checkout: str):
    """Calculate number of nights between dates."""
    try:
        checkin_date = datetime.fromisoformat(checkin.replace('Z', '+00:00'))
        checkout_date = datetime.fromisoformat(checkout.replace('Z', '+00:00'))
        nights = (checkout_date - checkin_date).days
        return max(1, nights)
    except:
        return 1


def calculate_price_with_strategy(base_price: float, nights: int, strategy: str):
    """Calculate price based on display strategy."""
    total = base_price * nights
    
    if strategy == "total":
        return {
            "display_price": total,
            "label": "Total Price",
            "breakdown": None
        }
    elif strategy == "per-night":
        return {
            "display_price": base_price,
            "label": "Per Night",
            "breakdown": {
                "per_night": base_price,
                "nights": nights,
                "total": total
            }
        }
    elif strategy == "with-fees":
        taxes = total * 0.12
        fees = 25.00
        grand_total = total + taxes + fees
        return {
            "display_price": grand_total,
            "label": "Total with Fees",
            "breakdown": {
                "base": total,
                "taxes": round(taxes, 2),
                "fees": fees,
                "total": round(grand_total, 2)
            }
        }
    elif strategy == "dynamic":
        # Simulate dynamic pricing with random variation
        variation = random.uniform(0.9, 1.15)
        dynamic_price = total * variation
        savings = total - dynamic_price if dynamic_price < total else 0
        return {
            "display_price": round(dynamic_price, 2),
            "label": "Dynamic Price",
            "breakdown": {
                "original": total,
                "current": round(dynamic_price, 2),
                "savings": round(savings, 2) if savings > 0 else 0
            }
        }
    
    return {
        "display_price": total,
        "label": "Total Price",
        "breakdown": None
    }


def generate_booking_id():
    """Generate a unique booking ID."""
    return f"BK-{uuid.uuid4().hex[:8].upper()}"


def generate_confirmation_number():
    """Generate a confirmation number."""
    return f"CONF-{uuid.uuid4().hex[:6].upper()}"
