# Multi-stage build for optimized Go binary
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates (needed for private repos and HTTPS)
RUN apk add --no-cache git ca-certificates tzdata

# Create appuser for security
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
RUN go mod verify

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o slack-ai-bot .

# Final stage: minimal runtime image
FROM scratch

# Import ca-certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Import timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Import user and group files
COPY --from=builder /etc/passwd /etc/passwd

# Copy the binary
COPY --from=builder /app/slack-ai-bot /slack-ai-bot

# Use non-root user
USER appuser

# Expose health check port (if we add HTTP endpoints)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/slack-ai-bot", "--health-check"] || exit 1

# Run the binary
ENTRYPOINT ["/slack-ai-bot"]
