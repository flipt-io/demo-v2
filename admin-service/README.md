# Admin Service

A Go-based admin service for managing hotel bookings with Flipt feature flags integration.

## Features

- **Booking Management**: View, approve, and reject hotel bookings
- **Flipt Integration**: Uses `flipt-client-go` with streaming support for real-time flag updates
- **Feature Flags**:
  - `auto-approval`: Boolean flag for automatic booking approval
  - `approval-tier`: Variant flag for multi-level approval workflows (standard, premium, vip)
- **OpenTelemetry**: Full observability with distributed tracing and metrics
- **RESTful API**: Simple HTTP API for booking operations

## API Endpoints

### Booking Operations

#### Get All Bookings

```bash
GET /api/bookings?status=pending
```

Returns all bookings, optionally filtered by status (`pending`, `confirmed`, `rejected`).

#### Get Booking Details

```sh
GET /api/bookings/{id}
```

Returns details for a specific booking.

#### Approve Booking

```sh
POST /api/bookings/{id}/approve
```

Approves a pending booking. The service:

1. Fetches the booking from hotel-service
2. Evaluates Flipt feature flags to determine:
   - Whether to auto-approve based on the `auto-approval` flag
   - The approval tier (standard/premium/vip) based on the `approval-tier` flag
3. Generates a confirmation number
4. Updates the booking status to `confirmed` in hotel-service via PATCH

Response includes the booking details, auto-approval status, approval tier, and confirmation number.

#### Reject Booking

```sh
POST /api/bookings/{id}/reject
Content-Type: application/json

{
  "reason": "Insufficient availability"
}
```

Rejects a pending booking with a reason. Updates the booking status to `rejected` in hotel-service via PATCH.

### Feature Flag Status

#### Get Flag Status

```sh
GET /api/flags?entity_id=admin
```

Returns current status of feature flags for the given entity.

### Health Check

```sh
GET /health
```

Returns service health status.

## Configuration

Environment variables:

- `FLIPT_URL`: Flipt server URL (default: `http://flipt:8080`)
- `FLIPT_NAMESPACE`: Flipt namespace (default: `admin`)
- `FLIPT_ENVIRONMENT`: Flipt environment (default: `onoffinc`)
- `HOTEL_SERVICE_URL`: Hotel service URL (default: `http://hotel-service:8000`)
- `PORT`: Service port (default: `8001`)

**Note:** The admin service uses the `admin` namespace in Flipt, separate from the `default` namespace used by webapp and hotel service. This allows for isolated feature flag management for admin-specific functionality.

## Example Usage

```bash
# List pending bookings
curl http://localhost:8001/api/bookings?status=pending

# View specific booking
curl http://localhost:8001/api/bookings/BK-001

# Approve a booking
curl -X POST http://localhost:8001/api/bookings/BK-001/approve

# Reject a booking
curl -X POST http://localhost:8001/api/bookings/BK-002/reject \
  -H "Content-Type: application/json" \
  -d '{"reason": "Payment declined"}'

# Check feature flag status
curl http://localhost:8001/api/flags
```

## Observability

### Metrics

The service exports the following metrics to Prometheus:

- `admin_booking_approvals_total`: Counter for booking approvals
- `admin_booking_views_total`: Counter for booking views

### Traces

All operations are traced using OpenTelemetry and sent to Jaeger. View traces at:

- Jaeger UI: <http://localhost:16686>
- Select service: `admin-service`

Trace spans include:

- Flag evaluations with context
- Booking operations
- Auto-approval decisions
- Approval tier assignments

## Feature Flag Configuration

Admin feature flags are defined in the `admin` namespace (see `gitea/admin-features.yaml`):

### Boolean Flag: `auto-approval`

Controls automatic approval of bookings based on criteria.

```yaml
namespace:
  key: admin
  name: Admin
flags:
  - key: auto-approval
    name: Auto Approval
    description: Automatically approve bookings that meet criteria
    type: BOOLEAN_FLAG_TYPE
    enabled: true
```

### Variant Flag: `approval-tier`

Determines the approval tier for bookings.

```yaml
flags:
  - key: approval-tier
    name: Approval Tier
    description: Multi-level approval workflow
    type: VARIANT_FLAG_TYPE
    enabled: true
    variants:
      - key: standard
        name: Standard Approval
      - key: premium
        name: Premium Approval
      - key: vip
        name: VIP Approval
```

### Segments

The admin namespace includes segments for targeting specific booking types:

- `trusted-bookings`: Low-risk bookings (total_price ≤ $500)
- `high-value-bookings`: High-value bookings (total_price ≥ $1000)
- `premium-hotels`: Premium or luxury hotel category bookings

## Architecture

```
┌─────────────────┐
│  Admin Service  │
│   (Go/HTTP)     │
│   Port: 8001    │
└────────┬────────┘
         │
         ├──────► Flipt (Streaming)
         │        - auto-approval flag
         │        - approval-tier flag
         │
         ├──────► Hotel Service (REST)
         │        - GET /api/bookings/{id}
         │        - PATCH /api/bookings/{id}
         │
         ├──────► Jaeger (Traces)
         │
         └──────► Prometheus (Metrics)
```

## Development

```bash
# Format code
go fmt ./...

# Run tests
go test ./...

# Build
go build -o admin-service .
```

## License

MIT
