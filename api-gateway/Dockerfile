# ---- Build stage ----
FROM golang:1.24-alpine AS builder

# Install only required dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go module files first (better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

WORKDIR /app/api-gateway

# Build statically linked binary with security flags
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o api-gateway ./cmd/main.go

# ---- Runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy binary only (no source, no extras)
COPY --from=builder /app/api-gateway /app/
COPY --from=builder /app/config /app/config

# Run as non-root (distroless defaults to this)
USER nonroot:nonroot

# Drop capabilities (extra precaution if run with Docker’s default seccomp profile)
# NOTE: Works only if container is run with --cap-drop, but documenting best practice
# No SHELL or package manager exists here (attack surface minimized)

EXPOSE 8000

ENTRYPOINT ["/app/api-gateway"]
