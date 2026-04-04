# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s"

# Runtime stage
FROM alpine:latest

WORKDIR /tmp

# Copy the binary from builder
COPY --from=builder /app/chaosmonkey /bin/chaosmonkey

# Run the application
CMD ["/bin/chaosmonkey"]
