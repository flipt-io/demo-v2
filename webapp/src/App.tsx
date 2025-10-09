import { useState } from "react";
import {
  useFliptBoolean,
  useFliptSelector,
} from "@flipt-io/flipt-client-react";

const entityId = "user-123";
const fallbackImage =
  "https://images.unsplash.com/photo-1758132123976-6730692335f7?q=80&w=1544";

const loadTheme = (flagKey: string) => {
  return function (client: any, isLoading: boolean, error: any) {
    if (isLoading) {
      return "";
    }
    if (client && !isLoading && !error) {
      try {
        return (
          JSON.parse(
            client.evaluateVariant({
              flagKey,
              entityId,
              context: {
                month: (new Date().getMonth() + 1).toFixed(0),
              },
            }).variantAttachment,
          )[0] || fallbackImage
        );
      } catch (e) {
        console.error("Error evaluating variant flag theme:", e);
      }
      return fallbackImage;
    }
  };
};

interface Hotel {
  id: string;
  name: string;
  location: string;
  description: string;
  rating: number;
  price: number;
  price_label: string;
  price_breakdown?: any;
  loyalty_member_price?: number;
  amenities: string[];
  image_url: string;
  available_rooms: number;
  category: string;
}

interface HotelResponse {
  hotels: Hotel[];
  total_count: number;
  price_display_strategy: string;
  real_time_availability: boolean;
  loyalty_program_enabled: boolean;
}

interface BookingResponse {
  booking_id: string;
  hotel_id: string;
  status: string;
  confirmation_number?: string;
  total_price: number;
  created_at: string;
}

function App() {
  const sale = useFliptBoolean("sale", false, entityId, {});
  const themeImage = useFliptSelector(loadTheme("theme"));
  const [showHotels, setShowHotels] = useState(false);
  const [hotels, setHotels] = useState<Hotel[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hotelResponse, setHotelResponse] = useState<HotelResponse | null>(
    null,
  );
  const [bookingInProgress, setBookingInProgress] = useState<string | null>(
    null,
  );
  const [bookingResult, setBookingResult] = useState<BookingResponse | null>(
    null,
  );
  const [showBookingModal, setShowBookingModal] = useState(false);
  const [showConfirmationModal, setShowConfirmationModal] = useState(false);
  const [selectedHotel, setSelectedHotel] = useState<Hotel | null>(null);

  const fetchHotels = async () => {
    setLoading(true);
    setError(null);
    try {
      const today = new Date();
      const checkin = new Date(today);
      checkin.setDate(today.getDate() + 7);
      const checkout = new Date(checkin);
      checkout.setDate(checkin.getDate() + 2);
      const din = checkin.toISOString().split("T")[0];
      const dout = checkout.toISOString().split("T")[0];
      const response = await fetch(
        `/api/hotels?entity_id=${entityId}&guests=2&checkin=${din}&checkout=${dout}`,
      );
      if (!response.ok) {
        throw new Error("Failed to fetch hotels");
      }
      const data: HotelResponse = await response.json();
      setHotels(data.hotels);
      setHotelResponse(data);
      setShowHotels(true);
    } catch (err) {
      console.error("Error fetching hotels:", err);
      setError("Unable to load hotels. Please try again later.");
    } finally {
      setLoading(false);
    }
  };

  const handleExploreClick = () => {
    if (!showHotels) {
      fetchHotels();
    } else {
      setShowHotels(false);
    }
  };

  const handleBookingClick = (hotel: Hotel) => {
    setSelectedHotel(hotel);
    setShowConfirmationModal(true);
  };

  const handleConfirmBooking = async () => {
    if (!selectedHotel) return;

    setShowConfirmationModal(false);
    setBookingInProgress(selectedHotel.id);
    setError(null);
    try {
      // Create booking request
      const today = new Date();
      const checkin = new Date(today);
      checkin.setDate(today.getDate() + 7);
      const checkout = new Date(checkin);
      checkout.setDate(checkin.getDate() + 2);

      const bookingRequest = {
        hotel_id: selectedHotel.id,
        checkin: checkin.toISOString().split("T")[0],
        checkout: checkout.toISOString().split("T")[0],
        guests: 2,
        guest_name: "John Doe",
        guest_email: "john.doe@example.com",
      };

      const response = await fetch(
        `/api/hotels/${selectedHotel.id}/book?entity_id=${entityId}`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify(bookingRequest),
        },
      );

      if (!response.ok) {
        throw new Error("Failed to book hotel");
      }

      const data: BookingResponse = await response.json();
      setBookingResult(data);
      setShowBookingModal(true);
    } catch (err) {
      console.error("Error booking hotel:", err);
      setError("Unable to complete booking. Please try again later.");
    } finally {
      setBookingInProgress(null);
      setSelectedHotel(null);
    }
  };

  const handleCancelBooking = () => {
    setShowConfirmationModal(false);
    setSelectedHotel(null);
  };

  const closeBookingModal = () => {
    setShowBookingModal(false);
    setBookingResult(null);
  };

  return (
    <>
      {sale && (
        <div className="bg-yellow-300 text-black p-4 text-center font-bold">
          Season Sale! Book your dream vacation now!
        </div>
      )}
      {!showHotels ? (
        <div
          className="h-full bg-cover bg-center"
          style={{
            backgroundImage: "url(" + themeImage + ")",
          }}
        >
          <header className="flex justify-between items-center p-6 bg-white shadow text-gray-600">
            <div className="text-2xl font-bold"> TravelCo </div>
            <nav></nav>
          </header>
          <section className="m-auto h-3/5 w-2/5 flex flex-col justify-end items-center text-white text-center">
            <h1 className="text-4xl font-bold mb-4">
              Your Next Adventure Awaits
            </h1>
            <button
              onClick={handleExploreClick}
              disabled={loading}
              className="bg-white text-black px-6 py-3 font-semibold rounded shadow-xl"
            >
              {loading ? "Loading..." : "Explore Now"}
            </button>
          </section>
        </div>
      ) : (
        <div className="h-full ">
          <header className="flex justify-between items-center p-6 bg-white shadow text-gray-600">
            <div className="text-2xl font-bold"> TravelCo </div>
            <nav>
              <button
                onClick={handleExploreClick}
                className="px-3 hover-underline"
              >
                ← Back
              </button>
            </nav>
          </header>

          <div className="hotels-container p-6">
            <div className="max-w-7xl m-auto">
              <div className="mb-6">
                <h2 className="text-3xl font-bold text-gray-800 mb-2">
                  Popular Hotels
                </h2>
                {hotelResponse && (
                  <div className="flex gap-4 text-sm text-gray-600">
                    <span className="badge">
                      Price Strategy: {hotelResponse.price_display_strategy}
                    </span>
                    {hotelResponse.loyalty_program_enabled && (
                      <span className="badge badge-success">
                        💎 Loyalty Program Active
                      </span>
                    )}
                    {hotelResponse.real_time_availability && (
                      <span className="badge badge-info">
                        🔄 Real-time Availability
                      </span>
                    )}
                  </div>
                )}
              </div>

              {error && (
                <div className="error-message p-4 mb-4 bg-red-100 text-red-700 rounded">
                  {error}
                </div>
              )}

              <div className="hotels-grid">
                {hotels.map((hotel) => (
                  <div key={hotel.id} className="hotel-card">
                    <img
                      src={hotel.image_url}
                      alt={hotel.name}
                      className="hotel-image"
                    />
                    <div className="hotel-content p-4">
                      <div className="flex justify-between items-start mb-2">
                        <h3 className="text-xl font-bold text-gray-800">
                          {hotel.name}
                        </h3>
                        <div className="rating">⭐ {hotel.rating}</div>
                      </div>
                      <p className="text-sm text-gray-600 mb-2">
                        📍 {hotel.location}
                      </p>
                      <p className="text-sm text-gray-700 mb-3">
                        {hotel.description}
                      </p>
                      <div className="amenities mb-3">
                        {hotel.amenities.slice(0, 3).map((amenity, idx) => (
                          <span key={idx} className="amenity-tag">
                            {amenity}
                          </span>
                        ))}
                        {hotel.amenities.length > 3 && (
                          <span className="amenity-tag">
                            +{hotel.amenities.length - 3} more
                          </span>
                        )}
                      </div>
                      <div className="hotel-footer">
                        <div>
                          <div className="price-container">
                            <span className="price">${hotel.price}</span>
                            <span className="price-label text-sm text-gray-600">
                              {hotel.price_label}
                            </span>
                          </div>
                          {hotel.loyalty_member_price && (
                            <div className="loyalty-price">
                              💎 Member: ${hotel.loyalty_member_price}
                            </div>
                          )}
                        </div>
                        <button
                          className="book-btn"
                          onClick={() => handleBookingClick(hotel)}
                          disabled={bookingInProgress === hotel.id}
                        >
                          {bookingInProgress === hotel.id
                            ? "Booking..."
                            : "Book Now"}
                        </button>
                      </div>
                      {hotel.available_rooms <= 5 && (
                        <div className="urgency-tag">
                          ⚠️ Only {hotel.available_rooms} rooms left!
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}

      {showConfirmationModal && selectedHotel && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-8 max-w-md w-full m-auto">
            <div className="text-center">
              <h2 className="text-2xl font-bold text-gray-800 mb-4">
                Confirm Your Booking
              </h2>
              <div className="text-left mb-6 text-gray-600">
                <div className="bg-gray-50 rounded p-4 mb-4">
                  <h3 className="font-bold text-lg mb-2">
                    {selectedHotel.name}
                  </h3>
                  <p className="text-sm mb-1">📍 {selectedHotel.location}</p>
                  <p className="text-sm mb-1">
                    ⭐ {selectedHotel.rating} rating
                  </p>
                  <p className="text-sm mb-2">👥 2 guests • 2 nights</p>
                  <div className="border-t pt-2 mt-2">
                    <p className="font-bold text-lg">
                      ${selectedHotel.price}{" "}
                      <span className="text-sm font-normal">
                        {selectedHotel.price_label}
                      </span>
                    </p>
                    {selectedHotel.loyalty_member_price && (
                      <p className="text-sm">
                        💎 Member Price: ${selectedHotel.loyalty_member_price}
                      </p>
                    )}
                  </div>
                </div>
                <p className="text-sm text-gray-600 text-center">
                  Are you sure you want to proceed with this booking?
                </p>
              </div>
              <div className="flex gap-3 justify-center">
                <button
                  onClick={handleCancelBooking}
                  className="px-6 py-2 rounded font-semibold bg-gray-200 text-gray-700 hover:bg-gray-300"
                >
                  Cancel
                </button>
                <button
                  onClick={handleConfirmBooking}
                  className="px-6 py-2 rounded font-semibold bg-blue-600 text-white hover:bg-blue-700"
                >
                  Confirm Booking
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {showBookingModal && bookingResult && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-8 max-w-md w-full m-auto">
            <div className="text-center">
              {bookingResult.status === "confirmed" ? (
                <>
                  <div className="text-6xl mb-4">✅</div>
                  <h2 className="text-2xl font-bold text-gray-800 mb-2">
                    Booking Confirmed!
                  </h2>
                  <p className="text-gray-600 mb-4">
                    Your reservation has been confirmed instantly.
                  </p>
                  <div className="bg-gray-50 rounded p-4 mb-4 text-left">
                    <p className="text-sm text-gray-600 mb-1">
                      Booking ID: {bookingResult.booking_id}
                    </p>
                    <p className="text-sm text-gray-600 mb-1">
                      Confirmation: {bookingResult.confirmation_number}
                    </p>
                    <p className="text-sm text-gray-600">
                      Total Price: ${bookingResult.total_price}
                    </p>
                  </div>
                </>
              ) : (
                <>
                  <div className="text-6xl mb-4">⏳</div>
                  <h2 className="text-2xl font-bold text-gray-800 mb-2">
                    Booking Pending
                  </h2>
                  <p className="text-gray-600 mb-4">
                    Your booking request has been submitted and is pending
                    confirmation.
                  </p>
                  <div className="bg-gray-50 rounded p-4 mb-4 text-left">
                    <p className="text-sm text-gray-600 mb-1">
                      Booking ID: {bookingResult.booking_id}
                    </p>
                    <p className="text-sm text-gray-600">
                      Total Price: ${bookingResult.total_price}
                    </p>
                  </div>
                  <p className="text-sm text-gray-500">
                    You'll receive a confirmation email within 24 hours.
                  </p>
                </>
              )}
              <button
                onClick={closeBookingModal}
                className="mt-6 bg-blue-600 text-white px-6 py-2 rounded font-semibold hover:bg-blue-700"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

export default App;
