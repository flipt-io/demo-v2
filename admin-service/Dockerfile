# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./
COPY hotelclient ./hotelclient
COPY openapi.json ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o admin-service .

# Runtime stage
FROM alpine:latest

WORKDIR /tmp

# Copy the binary from builder
COPY --from=builder /app/admin-service /bin/admin-service

# Expose port
EXPOSE 8001

# Run the application
CMD ["/bin/admin-service"]
