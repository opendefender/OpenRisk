# backend/Dockerfile
# Multi-stage build pour des images finales légères (Qualité Industrielle)
# 1. Build stage
# Float to the latest 1.25.x patch so the built binary's stdlib picks up Go
# security fixes (Trivy flags stdlib CVEs pinned to an exact patch, e.g. the
# crypto/tls + net/url advisories fixed across 1.25.5–1.25.11).
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Compile le binaire unique statiquement
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /openrisk ./cmd/server

# 2. Final stage
FROM alpine:latest
WORKDIR /app
# Installe les certificats pour les appels HTTPS (si besoin)
RUN apk --no-cache add ca-certificates
COPY --from=builder /openrisk /usr/local/bin/openrisk
EXPOSE 8080
# Le binaire contient tout : Auth, Worker, API (Simplicité Radicale)
ENTRYPOINT ["/usr/local/bin/openrisk"]