# Multi-stage build
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -o rate-limiter ./cmd/rate-limiter

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite-libs

WORKDIR /app

# Copy binary and config
COPY --from=builder /app/rate-limiter .
COPY configs/config.yaml ./configs/

# Create data directory
RUN mkdir -p /app/data

EXPOSE 8080

# Volume for persistent data
VOLUME ["/app/data", "/app/configs"]

ENTRYPOINT ["./rate-limiter"]
CMD ["--config", "./configs/config.yaml"]