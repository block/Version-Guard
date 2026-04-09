# Version Guard Docker Image
#
# Build:
#   docker build -t version-guard -f deploy/Dockerfile .
#
# Run:
#   docker run -p 8080:8080 --env-file .env version-guard

# Build stage
FROM golang:1.24.2-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server binary
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /version-guard ./cmd/server

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 1000 version-guard
USER version-guard

WORKDIR /app

# Copy binary from builder
COPY --from=builder /version-guard /app/version-guard

# gRPC port
EXPOSE 8080

ENTRYPOINT ["/app/version-guard"]
