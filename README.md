# Flipt v2 Demo - TravelCo

A comprehensive demonstration of Flipt v2 feature management platform showcasing a travel booking application with both frontend (React) and backend (Python) services using feature flags.

## Overview

This demo represents **TravelCo**, a fictional travel company's booking platform that uses Flipt feature flags to control various aspects of the user experience and backend functionality.

### Architecture

```
┌──────────────────────────────────────────────────────────┐
│                         Browser                          │
│              http://localhost:4000                       │
└────────────────────────┬─────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────┐
│                    Nginx (Port 80)                       │
│  - Serves webapp static files                            │
│  - Proxies /api/* → hotel-service:8000                   │
│  - Proxies /internal/v1/* → flipt:8080                   │
└─────────────┬──────────────────────┬─────────────────────┘
              │                      │
              │                      │
    ┌─────────▼────────┐   ┌─────────▼────────┐
    │   Hotel Service  │   │   Flipt v2       │
    │  Python/FastAPI  │──►│  (Port 8080)     │
    │   (Port 8000)    │   │                  │
    └─────────┬────────┘   └──┬────┬──────┬───┘
              │               │    │      │
              │               │    │      │
              │               │    │      └───────┐
              │               │    │              │
              │               │    │    ┌─────────▼─────┐
              │               │    │    │    Gitea      │
              │               │    │    │  (Port 3000)  │
              │               │    │    │Feature Flags  │
              │               │    │    └───────────────┘
              │               │    │
              │               │    └────┐
              │               │         │
              │               │    ┌────▼──────────┐
              │               │    │  Prometheus   │
              │               │    │  (Port 9090)  │
              │               │    │   Metrics &   │
              │               │    │   Analytics   │
              │               │    └───────────────┘
              │               │
              │          ┌────▼──────────┐
              │          │    Jaeger     │
              └──────────►  (Port 16686) │
                         │  Distributed  │
                         │    Tracing    │
                         └───────────────┘
```

## Services

### 1. Webapp (React + TypeScript)

- **Port**: 4000
- **Technology**: React 19, Vite, TailwindCSS
- **Flipt Client**: `@flipt-io/flipt-client-react`
- **Feature Flags**:
  - `sale` (boolean): Show/hide seasonal sale banner
  - `theme` (variant): Dynamic hero background based on season
- **User Experience**:
  - Browse hotels with dynamic pricing and availability
  - Confirmation dialog before booking to prevent accidental reservations
  - Real-time booking status with confirmation numbers
- **Telemetry**: OpenTelemetry metrics exported to Prometheus

### 2. Hotel Service (Python + FastAPI)

- **Port**: 8000
- **Technology**: Python 3.12, FastAPI, Uvicorn
- **Flipt Client**: `flipt` (Python SDK)
- **Feature Flags**:
  - `price-display-strategy` (variant): Control price presentation
    - `per-night`: Show price per night
    - `total`: Show total price
    - `with-fees`: Show price with fees breakdown
    - `dynamic`: Show dynamic pricing with savings
  - `real-time-availability` (boolean): Enable live room availability
  - `loyalty-program` (boolean): Show loyalty discounts (10% off)
  - `instant-booking` (boolean): Immediate vs pending confirmations
  - `similar-hotels` (boolean): Show hotel recommendations
- **Telemetry**: Full OpenTelemetry integration (traces + metrics)

### 3. Flipt (Feature Management)

- **Port**: 8080
- **Version**: v2
- **Features**:
  - Git-based feature flag storage (via Gitea)
  - Analytics storage in Prometheus
  - Authentication via OIDC (Gitea)
  - Environment: `onoffinc`
  - Namespace: `default`

### 4. Gitea (Git Server)

- **Port**: 3000
- **Purpose**: Git-based storage for feature flags
- **Credentials**: `admin:password`
- **Repository**: `onoffinc/features`

### 5. Jaeger (Distributed Tracing)

- **Port**: 16686
- **Purpose**: Collect and visualize traces from all services
- **Protocol**: OTLP over HTTP

### 6. Prometheus (Metrics & Analytics)

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
- **Hotel API**: <http://localhost:8000> - Hotel service REST API
- **Hotel API Docs**: <http://localhost:8000/docs> - Interactive API documentation
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

## Feature Flags Configuration

All feature flags are defined in `gitea/features.yaml`:

### Boolean Flags

- `sale`: Seasonal sale banner
- `real-time-availability`: Live room availability updates
- `loyalty-program`: Loyalty member discounts
- `instant-booking`: Instant confirmation flow
- `similar-hotels`: Hotel recommendations

### Variant Flags

- `theme`: Seasonal hero backgrounds (city, beach, mountain, snowboard)
- `price-display-strategy`: Price presentation (per-night, total, with-fees, dynamic)

### Segments

- `winter`, `spring`, `summer`, `fall`: Seasonal segments based on month
- `premium-users`: Premium tier users
- `budget-users`: Budget-conscious users

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
4. Commit changes
5. Flipt polls and updates automatically (30s interval)

## Key Learnings

This demo showcases:

1. **Multi-language Support**: React (frontend) and Python (backend) both using Flipt
2. **Multiple Flag Types**: Boolean and variant flags with different use cases
3. **Segmentation**: Context-based targeting (seasonal, user tier)
4. **Git-based Storage**: Feature flags as code with version control
5. **Full Observability**: Traces, metrics, and analytics integration
6. **Real-world Use Cases**:
   - A/B testing (price strategies)
   - Progressive rollouts (instant booking)
   - Seasonal targeting (themes)
   - Premium features (loyalty program)

## License

MIT
