# Multi-stage build for OpenRisk

# Stage 1: Build backend
# Float to the latest 1.25.x patch so the binary's standard library carries Go
# security fixes; go.mod's `go` line sets the floor.
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app
RUN apk add --no-cache git make

# Copy backend code
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o openrisk ./cmd/server

# Stage 2: Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# Stage 3: Runtime
# alpine:3.18 reached end of life in May 2025 and no longer gets fixes.
FROM alpine:3.24
RUN apk upgrade --no-cache && apk add --no-cache ca-certificates curl

WORKDIR /app

# Copy backend binary from builder
COPY --from=backend-builder /app/openrisk /app/openrisk

# Copy frontend build from builder
COPY --from=frontend-builder /app/dist /app/public

# Create non-root user
RUN addgroup -g 1000 openrisk && \
    adduser -D -u 1000 -G openrisk openrisk
USER openrisk

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/v1/health || exit 1

EXPOSE 8080

CMD ["./openrisk"]
