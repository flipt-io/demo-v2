# Hotel Service API

A Python FastAPI-based hotel availability and booking service that demonstrates Flipt v2 feature flags with OpenTelemetry integration.

## Features

- **Hotel Search**: Search hotels by location with feature-flag controlled pricing strategies
- **Availability Checking**: Real-time availability checks (feature flag controlled)
- **Booking**: Hotel booking with instant confirmation (feature flag controlled)
- **Similar Hotels**: Recommendation engine (feature flag controlled)
- **OpenTelemetry**: Full observability with traces and metrics exported to Jaeger and Prometheus

## Feature Flags

This service uses the following Flipt feature flags:

### Boolean Flags

1. **real-time-availability** (default: true)
   - Controls whether to show real-time room availability updates
   - When enabled: Shows live availability with timestamps
   - When disabled: Shows cached availability data

2. **loyalty-program** (default: false)
   - Controls whether to display loyalty program benefits
   - When enabled: Shows 10% discount for hotels with rating >= 4.5
   - Metrics: Track loyalty program engagement

3. **instant-booking** (default: false)
   - Controls booking confirmation flow
   - When enabled: Immediate booking confirmation
   - When disabled: Booking goes to "pending" status
   - Useful for A/B testing conversion rates

4. **similar-hotels** (default: false)
   - Controls similar hotel recommendations
   - When enabled: Shows up to 3 similar hotels
   - Tracks engagement with recommendations

### Variant Flags

1. **price-display-strategy** (default: "per-night")
   - Controls how prices are displayed to users
   - Variants:
     - `per-night`: Shows price per night
     - `total`: Shows total price for stay
     - `with-fees`: Shows total with taxes and fees breakdown
     - `dynamic`: Shows dynamic pricing with savings indicator
   - Use for A/B testing which strategy converts best

## API Endpoints

### Search Hotels
```bash
GET /api/hotels?location=Miami&checkin=2024-03-01&checkout=2024-03-05&guests=2&entity_id=user-123
```

### Check Availability
```bash
GET /api/hotels/hotel-1/availability?checkin=2024-03-01&checkout=2024-03-05&guests=2&entity_id=user-123
```

### Book Hotel
```bash
POST /api/hotels/hotel-1/book?entity_id=user-123
Content-Type: application/json

{
  "hotel_id": "hotel-1",
  "checkin": "2024-03-01",
  "checkout": "2024-03-05",
  "guests": 2,
  "guest_name": "John Doe",
  "guest_email": "john@example.com"
}
```

### Get Similar Hotels
```bash
GET /api/hotels/hotel-1/similar?entity_id=user-123
```

### Get Popular Hotels
```bash
GET /api/hotels/popular?region=Florida
```

## Metrics

The service exports the following custom metrics:

- `hotel_searches_total`: Total number of hotel searches
- `hotel_availability_checks_total`: Total availability checks
- `hotel_bookings_total`: Total bookings created
- `feature_flag_evaluations_total`: Feature flag evaluation count
- `price_display_strategy_usage`: Histogram of price strategy usage

All metrics include relevant labels for filtering and analysis.

## Tracing

Every request is traced with OpenTelemetry, including:
- HTTP request spans (via FastAPI instrumentation)
- Feature flag evaluation spans
- Custom business logic spans

Traces include attributes for:
- Hotel IDs
- Entity IDs (users)
- Feature flag values
- Search parameters

## Development

### Local Setup

```bash
# Install dependencies
pip install -r requirements.txt

# Run locally (ensure Flipt and Jaeger are running)
uvicorn main:app --reload --host 0.0.0.0 --port 8000
```

### Environment Variables

- `FLIPT_URL`: Flipt server URL (default: http://flipt:8080)
- `FLIPT_NAMESPACE`: Flipt namespace (default: default)
- `FLIPT_ENVIRONMENT`: Flipt environment (default: onoffinc)
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP endpoint (default: http://jaeger:4318)
- `OTEL_SERVICE_NAME`: Service name for traces (default: hotel-service)

### Docker

```bash
# Build
docker build -t hotel-service .

# Run
docker run -p 8000:8000 \
  -e FLIPT_URL=http://flipt:8080 \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318 \
  hotel-service
```

## Testing Feature Flags

You can test different feature flag configurations by:

1. **Using different entity IDs**: Pass different `entity_id` query parameters to see flag variations
2. **Using context**: Flags can be segmented by context like `hotel_category`, `guests`, etc.
3. **Flipt UI**: Modify flags in real-time at http://localhost:8080

## Integration with Demo

This service integrates with the TravelCo demo webapp by:

1. Providing hotel search backend functionality
2. Sharing the same Flipt instance and feature flags
3. Exporting metrics to the same Prometheus instance
4. Sending traces to the same Jaeger instance
5. Demonstrating multi-language Flipt client usage (Python vs React)

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────┐
│   Webapp    │────▶│ Hotel Service│────▶│  Flipt  │
│  (React)    │     │   (Python)   │     │   v2    │
└─────────────┘     └──────────────┘     └─────────┘
                           │                    
                           ├──────▶ Jaeger (traces)
                           │
                           └──────▶ Prometheus (metrics)
```

## License

MIT
