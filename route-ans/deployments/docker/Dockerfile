# ANS Resolution Server Dockerfile
# Multi-stage build for minimal production image

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
ARG VERSION=dev
ARG BUILD_TIME
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -w -s" \
    -o /app/bin/route-ans \
    ./cmd/resolver

# Runtime stage
FROM alpine:3.21

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 ans && \
    adduser -u 1000 -G ans -s /bin/sh -D ans

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/route-ans /app/route-ans

# Copy default config
COPY configs/resolver.yaml /etc/ans/resolver.yaml

# Create directories for TLS and trust files
RUN mkdir -p /etc/ans/tls /etc/ans/trust && \
    chown -R ans:ans /app /etc/ans

# Switch to non-root user
USER ans

# Expose ports
# 8080 - HTTP API
# 9090 - gRPC API (future)
# 9091 - Metrics
EXPOSE 8080 9090 9091

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["/app/route-ans"]
CMD ["--config", "/etc/ans/resolver.yaml"]
