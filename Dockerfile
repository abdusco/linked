FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy build script
COPY scripts/build.sh /tmp/build.sh
RUN chmod +x /tmp/build.sh

# Build the application with version info
ARG VERSION=dev
ARG BUILD_TIME=unknown
RUN /tmp/build.sh ${VERSION} ${BUILD_TIME}

FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS and wget for health checks
RUN apk --no-cache add ca-certificates wget

# Copy the binary from builder
COPY --from=builder /app/linked .

# Expose port
EXPOSE 8080

# Set default environment variables
ENV PORT=8080
ENV DB_PATH=/data/linked.db

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Labels for container registry
LABEL org.opencontainers.image.title="Link Shortener"
LABEL org.opencontainers.image.description="Fast link shortener with analytics"
LABEL org.opencontainers.image.source="https://github.com/abdusco/linked"
LABEL org.opencontainers.image.documentation="https://github.com/abdusco/linked#readme"

# Run the application
CMD ["./linked"]

