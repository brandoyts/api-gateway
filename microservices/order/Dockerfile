# ---- Build stage ----
FROM golang:1.24-alpine AS builder

# Install only required dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go module files from root (better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project (so we can build from microservices/order)
COPY . .

# Build statically linked binary with security flags
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o order-service ./microservices/order/cmd/main.go

# ---- Runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy binary only (no source, no extras)
COPY --from=builder /app/order-service /app/
COPY microservices/order/config/ /app/config/

# Run as non-root (distroless defaults to this)
USER nonroot:nonroot

EXPOSE 6002

ENTRYPOINT ["/app/order-service"]
