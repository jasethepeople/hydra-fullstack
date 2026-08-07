# Multi-stage build for HYDRA Fullstack
# Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server binary with CGO enabled for SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o server cmd/server/main.go

# Stage 2: Runtime
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite-libs

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/server .

# Copy web assets
COPY --from=builder /app/web ./web

# Create uploads directory
RUN mkdir -p uploads/memorial

# Expose ports
EXPOSE 8080 9090 50051

# Environment defaults
ENV PORT=8080
ENV METRICS_PORT=9090
ENV ENV=production
ENV DB_WAL_ENABLED=true
ENV MEMORIAL_SEASONAL=true
ENV MEMORIAL_WEBSOCKET=true
ENV MEMORIAL_CANVAS=true

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./server"]
