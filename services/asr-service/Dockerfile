FROM golang:1.24.7-alpine AS builder

# Install ffmpeg
RUN apk add --no-cache ffmpeg

# Set working directory
WORKDIR /app

# Copy go workspace files
COPY go.work go.work.sum ./

# Copy all service go.mod files
COPY services/asr-service/go.mod services/asr-service/go.sum ./services/asr-service/
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/
COPY services/user-service/go.mod services/user-service/go.sum ./services/user-service/
COPY services/material-service/go.mod services/material-service/go.sum ./services/material-service/
COPY services/ocr-service/go.mod services/ocr-service/go.sum ./services/ocr-service/
COPY services/quiz-service/go.mod services/quiz-service/go.sum ./services/quiz-service/
COPY gateway/go.mod gateway/go.sum ./gateway/
COPY proto/go.mod proto/go.sum ./proto/
COPY pkg/metrics/go.mod pkg/metrics/go.sum ./pkg/metrics/

# Copy proto files
COPY proto proto
COPY pkg pkg

# Download dependencies
RUN go work sync && go mod download

# Copy asr-service source code
COPY services/asr-service/ ./services/asr-service/

# Build the application
WORKDIR /app/services/asr-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o asr-service main.go

# Make executable
RUN chmod +x asr-service

# Final stage
FROM alpine:3.20

# Install ffmpeg and ca-certificates
RUN apk add --no-cache ffmpeg ca-certificates

# Create temp directories
RUN mkdir -p /tmp/asr

# Create app user
RUN adduser -D -s /bin/sh appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/services/asr-service/asr-service .

# Change ownership
RUN chown appuser:appuser /app/asr-service /tmp/asr

# Switch to app user
USER appuser

# Expose port
EXPOSE 50057

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:50057/api/v1/health || exit 1

# Start the application
CMD ["./asr-service"]