# Build stage
FROM golang:1.23-alpine AS builder

# Add necessary build tools
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" \
    -o /app/main cmd/main.go

# Final stage
FROM alpine:3.19

# Add non root user
RUN adduser -D -g '' appuser

# Install CA certificates
RUN apk --no-cache add ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main .

# Copy any additional required files (like config files)
COPY --from=builder /app/config ./config

# Copy email templates
COPY --from=builder /app/internal/email_templates ./internal/email_templates

# Use non root user
USER appuser

# Expose port
EXPOSE 3000

# Set environment variables
ENV GIN_MODE=release

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || exit 1

# Run the application
CMD ["./main"]