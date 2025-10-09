# Flipt v2 Demo - TravelCo

A comprehensive demonstration of Flipt v2 feature management platform showcasing a travel booking application with webapp (React), hotel (Python) and admin services (Go) using feature flags with both client-side and server-side SDK patterns.

> [!WARNING]
> This is a demonstration project designed to showcase Flipt's feature flag capabilities and integration patterns. It is **not intended for production use** and does not follow production best practices (e.g., authentication, data persistence, error handling, security measures, etc.). The goal is to demonstrate various ways to integrate and use Flipt across different programming languages and architectures.

## Overview

This demo represents **TravelCo**, a fictional travel company's booking platform that uses Flipt feature flags to control various aspects of the user experience and service functionality.

**SDK Architecture:**

- **Client-Side SDKs** (Webapp & Admin Service): Evaluate flags directly in the browser/client, fetching flag state and evaluating locally for low-latency decisions
- **Server-Side SDK** (Hotel Service): Evaluates flags on the server via direct API calls to Flipt, suitable for backend services and sensitive business logic

### Architecture

```
┌──────────────────────────────────────────────────────────┐
│                         Browser                          │
│         Webapp (Client-Side SDK - React)                 │
└────────────────────────┬─────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────┐
│                    Nginx (Port 4000)                     │
│  - Serves webapp static files                            │
│  - Proxies /api/* → hotel-service:8000                   │
│  - Proxies /internal/v1/* → flipt:8080                   │
└─────────────┬──────────────────────┬─────────────────────┘
              │                      │
              │                      │
    ┌─────────▼────────┐   ┌─────────▼────────┐
    │   Hotel Service  │   │   Flipt v2       │
    │  Python/FastAPI  │──►│  (Port 8080)     │◄────┐
    │  (Server SDK)    │   │                  │     │
    │   (Port 8000)    │   │                  │     │
    └─────────┬────────┘   └──┬────┬──────┬───┘     │
              │               │    │      │         │
              ▲               │    │      │         │
    ┌─────────┴────────┐      │    │      └───────┐ │
    │  Admin Service   │      │    │              │ │
    │    Go/HTTP       │──────┘    │    ┌─────────▼─┴───┐
    │ (Client SDK)     │           │    │    Gitea      │
    │   (Port 8001)    │           │    │  (Port 3000)  │
    └─────────┬────────┘           │    │Feature Flags  │
              │                    │    └───────────────┘
              │                    │
              │                    └────┐
              │                         │
              │                    ┌────▼──────────┐
              │                    │  Prometheus   │
              │                    │  (Port 9090)  │
              │                    │   Metrics &   │
              │                    │   Analytics   │
              │                    └───────────────┘
              │
              │                ┌────────────┐
              └────────────────►   Jaeger   │
                               │(Port 16686)│
                               │Distributed │
                               │  Tracing   │
                               └────────────┘
```

## Services

### 1. Webapp (React)

- **Port**: 4000
- **Technology**: React 19, Vite, TypeScript
- **Flipt Client**: `@flipt-io/flipt-client-react` (client-side SDK)
- **SDK Type**: Client-side - Evaluates flags in the browser for instant UI updates without reloads
- **Feature Flags**:
  - `sale` (boolean): Show/hide promotional banner
  - `theme` (variant): Dynamic hero background based on season
- **User Experience**:
  - Browse hotels with dynamic pricing and availability
  - Confirmation dialog before booking to prevent accidental reservations
  - Real-time booking status with confirmation numbers
- **Telemetry**: OpenTelemetry metrics exported to Prometheus

### 2. Hotel Service (Python)

- **Port**: 8000
- **Technology**: Python 3.12, FastAPI, Uvicorn
- **Flipt Client**: `flipt` (Python SDK - server-side)
- **SDK Type**: Server-side - Makes API calls to Flipt for each evaluation, suitable for backend business logic
- **API Documentation**: OpenAPI/Swagger UI at root endpoint (`/`)
- **Feature Flags**:
  - `price-display-strategy` (variant): Control price presentation
    - `per-night`: Show price per night
    - `total`: Show total price
    - `with-fees`: Show price with fees breakdown
    - `dynamic`: Show dynamic pricing with savings
  - `real-time-availability` (boolean): Enable live room availability
  - `loyalty-program` (boolean): Show loyalty discounts (10% off)
  - `instant-booking` (boolean): Immediate vs pending confirmations
- **Performance Optimization**: Uses Flipt batch evaluation API to evaluate `real-time-availability` and `loyalty-program` flags in a single request, reducing network overhead
- **Telemetry**: Full OpenTelemetry integration (traces + metrics)

### 3. Admin Service (Go)

- **Port**: 8001
- **Technology**: Go 1.25
- **Flipt Client**: `flipt-client-go` (client-side SDK with streaming support)
- **SDK Type**: Client-side - Fetches and caches flag state locally, with streaming updates for near real-time flag changes
- **API Documentation**: OpenAPI/Swagger UI at root endpoint (`/`)
- **Feature Flags**:
  - `auto-approval` (boolean): Automatically approve low-risk bookings
  - `approval-tier` (variant): Multi-level approval workflows
    - `standard`: Standard approval process
    - `premium`: Premium approval for higher-value bookings
    - `vip`: VIP approval for luxury bookings
- **Features**:
  - View all bookings (pending, confirmed, rejected)
  - Approve/reject bookings with feature flag evaluation
  - Automatically update booking status in hotel-service
  - Generate confirmation numbers for approved bookings
  - Real-time flag updates via streaming (5-second polling)
  - Intelligent approval routing based on booking value and hotel category
- **Telemetry**: Full OpenTelemetry integration (traces + metrics)

### 4. Flipt (Feature Management)

- **Port**: 8080
- **Version**: v2
- **Features**:
  - Git-based feature flag storage (via Gitea)
  - Analytics storage in Prometheus
  - Authentication via OIDC (Gitea)
  - Environment: `onoffinc`
  - Namespace: `default`

### 5. Gitea (Git Server)

- **Port**: 3000
- **Purpose**: Git-based storage for feature flags
- **Credentials**: `admin:password`
- **Repository**: `onoffinc/features`

### 6. Jaeger (Distributed Tracing)

- **Port**: 16686
- **Purpose**: Collect and visualize traces from all services
- **Protocol**: OTLP over HTTP

### 7. Prometheus (Metrics & Analytics)

- **Port**: 9090
- **Purpose**: Store metrics and analytics data
- **Credentials**: `admin:password`
- **Features**:
  - OTLP receiver enabled
  - Flipt analytics storage
  - Custom service metrics

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Make (optional)

### Start the Demo

```sh
# Build all services
make build

# Start all services
make start

# Or using docker-compose directly
docker compose up -d
```

### Stop the Demo

```sh
make stop

# Or using docker-compose directly
docker compose down -v
```

## Accessing the Demo

Once started, you can access:

- **Webapp**: <http://localhost:4000> - TravelCo booking site
- **Hotel API**: <http://localhost:8000> - Hotel service REST API with interactive Swagger UI documentation
- **Admin Service**: <http://localhost:8001> - Admin booking management API with interactive Swagger UI documentation
- **Flipt UI**: <http://localhost:8080> - Feature flag management
- **Gitea**: <http://localhost:3000> - Git repository for flags
- **Jaeger**: <http://localhost:16686> - Distributed tracing
- **Prometheus**: <http://localhost:9090> - Metrics and analytics

**Default Credentials**: `admin:password`

## Demo Scenarios

### Scenario 1: Seasonal Sale Banner

1. Open the webapp at <http://localhost:4000>
2. Notice if the seasonal sale banner appears at the top
3. Go to Flipt UI at <http://localhost:8080>
4. Toggle the `sale` boolean flag on/off
5. Check the webapp to see the banner appear or disappear
6. Use this to control promotional campaigns

### Scenario 2: Seasonal Theming

1. Open the webapp at <http://localhost:4000>
2. Notice the hero background changes based on the current month
3. Go to Flipt UI and modify the `theme` flag rules
4. See the changes reflected immediately in the webapp

### Scenario 3: Price Display A/B Testing

1. Use the hotel service API
2. Search hotels with different entity IDs
3. Notice different price display strategies
4. In Flipt UI, change the `price-display-strategy` variant distributions
5. Search again in webapp and observe changes

### Scenario 4: Progressive Feature Rollout

1. In Flipt UI, set `instant-booking` to disabled
2. Book a hotel via webapp - you'll see a confirmation dialog
3. Confirm the booking - status will be "pending"
4. Enable `instant-booking` flag in Flipt
5. Make another booking - status will be "confirmed"
6. View the rollout in Jaeger traces

### Scenario 5: Loyalty Program Launch

1. Disable `loyalty-program` flag initially
2. Search hotels - no loyalty discounts shown
3. Enable flag for specific user segments
4. Search again - premium hotels show 10% discount

### Scenario 6: Batch Flag Evaluation Performance

1. Open Jaeger at <http://localhost:16686>
2. Search for traces from the `hotel-service`
3. Find a hotel search trace and expand it
4. Notice the `feature_flag.batch_evaluation` span that evaluates both `loyalty-program` and `real-time-availability` flags in a single API call
5. Compare this to individual evaluations - batch evaluation reduces network overhead
6. In Flipt UI, toggle both flags on/off
7. Search hotels again and observe in Jaeger how both flags are evaluated together efficiently

### Scenario 7: Admin Booking Approval Workflow

1. Book a hotel via webapp at <http://localhost:4000> (without instant-booking enabled)
2. Open Admin Service at <http://localhost:8001/api/bookings?status=pending>
3. View pending bookings from the hotel service
4. Go to Flipt UI and configure `auto-approval` and `approval-tier` flags
5. Set approval rules based on booking value (e.g., auto-approve under $500)
6. Approve a booking via `POST /api/bookings/{id}/approve`
7. Booking status is updated to "confirmed" with a confirmation number
8. View traces in Jaeger showing flag evaluation, approval flow, and PATCH update
9. Check the booking status via hotel service: `GET /api/bookings/{id}`

### Scenario 7: Admin Booking Approval Workflow

1. Book a hotel via webapp at <http://localhost:4000> (without instant-booking enabled)
2. Open Admin Service at <http://localhost:8001/api/bookings?status=pending>
3. View pending bookings from the hotel service
4. Go to Flipt UI and configure `auto-approval` and `approval-tier` flags
5. Set approval rules based on booking value (e.g., auto-approve under $500)
6. Approve a booking via `POST /api/bookings/{id}/approve`
7. Booking status is updated to "confirmed" with a confirmation number
8. View traces in Jaeger showing flag evaluation, approval flow, and PATCH update
9. Check the booking status via hotel service: `GET /api/bookings/{id}`

### Scenario 8: Multi-tier Approval Strategy

1. In Flipt UI, configure `approval-tier` variant rules
2. Set segments: high-value bookings (>$1000) → VIP tier
3. Set segments: premium hotels → Premium tier
4. Default bookings → Standard tier
5. Create bookings with different price points and hotels
6. Approve bookings via admin service to see tier assignment
7. Monitor in Prometheus: `admin_booking_approvals_total` by tier
8. Review traces in Jaeger to see how context affects tier evaluation

## Feature Flags Configuration

All feature flags are defined in `gitea/features.yaml`:

### Boolean Flags

- `sale`: Seasonal sale banner
- `real-time-availability`: Live room availability updates
- `loyalty-program`: Loyalty member discounts
- `instant-booking`: Instant confirmation flow
- `auto-approval`: Automatic booking approval for low-risk bookings

### Variant Flags

- `theme`: Seasonal hero backgrounds (city, beach, mountain, snowboard)
- `price-display-strategy`: Price presentation (per-night, total, with-fees, dynamic)
- `approval-tier`: Multi-level approval (standard, premium, vip)

### Segments

- `winter`, `spring`, `summer`, `fall`: Seasonal segments based on month
- `premium-users`: Premium tier users
- `budget-users`: Budget-conscious users
- `trusted-bookings`: Low-risk bookings (≤$500)
- `high-value-bookings`: High-value bookings (≥$1000)
- `premium-hotels`: Premium/luxury hotel bookings

## Observability

### Metrics in Prometheus

Visit <http://localhost:9090> and query:

```promql
# Hotel service metrics
hotel_searches_total
hotel_availability_checks_total
hotel_bookings_total
feature_flag_evaluations_total
price_display_strategy_usage_bucket

# Admin service metrics
admin_booking_approvals_total
admin_booking_views_total

# Flipt metrics
flipt_evaluations_requests_total
flipt_evaluations_results_total
```

### Traces in Jaeger

Visit <http://localhost:16686>:

1. Select service
2. Click "Find Traces"
3. Explore request flows and feature flag evaluations
4. View span details including flag values and timings

### Flipt Analytics

Visit <http://localhost:8080>:

1. Navigate to Analytics section
2. View flag evaluation counts
3. See variant distribution breakdowns
4. Analyze flag performance over time

### Modifying Feature Flags

#### Option 1: Via Flipt UI (Recommended)

1. Go to <http://localhost:8080>
2. Navigate to flags
3. Edit flag rules, variants, or segments
4. Changes are synced to Git automatically

#### Option 2: Via Git

1. Go to Gitea: <http://localhost:3000>
2. Navigate to `onoffinc/features` repository
3. Edit `features.yaml`
4. Commit and push changes
5. Flipt polls and updates automatically (30s interval)

## Key Learnings

This demo showcases:

1. **Multi-language Support**: React (frontend), Python (backend), and Go (admin service) all using Flipt
2. **Client-Side vs Server-Side SDKs**:
   - **Client-Side** (Webapp & Admin): Fetch flag state and evaluate locally for low latency, with streaming updates
   - **Server-Side** (Hotel Service): Make API calls to Flipt for each evaluation, ideal for backend services
3. **Multiple Flag Types**: Boolean and variant flags with different use cases
4. **Segmentation**: Context-based targeting (seasonal, user tier, booking value)
5. **Git-based Storage**: Feature flags as code with version control
6. **Full Observability**: Traces, metrics, and analytics integration
7. **Streaming Updates**: Go client with real-time flag synchronization (5-second polling)
8. **Performance Optimization**: Batch evaluation API to reduce network overhead (Python service evaluates multiple boolean flags in a single request)
9. **Real-world Use Cases**:
   - A/B testing (price strategies)
   - Progressive rollouts (instant booking)
   - Seasonal targeting (themes)
   - Premium features (loyalty program)
   - Intelligent approval routing (admin service)
   - Multi-tier workflows (approval tiers)
   - Efficient flag evaluation patterns (batch API)
   - Different SDK patterns for different architectural needs

## License

MIT
