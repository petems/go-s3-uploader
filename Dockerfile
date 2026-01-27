# Dockerfile for go-s3-uploader
# Multi-stage build for minimal final image

FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o go-s3-uploader .

# Final stage: minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -g 1000 uploader && \
    adduser -D -u 1000 -G uploader uploader

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/go-s3-uploader /app/go-s3-uploader

# Change ownership
RUN chown -R uploader:uploader /app

# Switch to non-root user
USER uploader

# Set entrypoint
ENTRYPOINT ["/app/go-s3-uploader"]

# Default help command
CMD ["--help"]
