# backend/Dockerfile
# Multi-stage build pour des images finales légères (Qualité Industrielle)
#
# Build context: the REPOSITORY ROOT (`docker build -f backend/Dockerfile .`),
# not backend/. The SQL migrations live in /migrations at the root and the image
# must carry them: with DATABASE_URL set, the server runs them at boot and
# refuses to start when it cannot find them (#1001). backend/Dockerfile.dockerignore
# keeps the context down to backend/ and migrations/.
#
# 1. Build stage
# Float to the latest 1.26.x patch so the built binary's stdlib picks up Go
# security fixes (Trivy flags stdlib CVEs pinned to an exact patch, e.g. the
# crypto/tls + net/url advisories fixed across 1.25.5–1.25.11).
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
# Compile le binaire unique statiquement
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /openrisk ./cmd/server

# 2. Final stage
FROM alpine:latest
WORKDIR /app
# Installe les certificats pour les appels HTTPS (si besoin)
RUN apk --no-cache add ca-certificates openssl
COPY --from=builder /openrisk /usr/local/bin/openrisk
COPY backend/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
# The SQL migration layer. MIGRATIONS_DIR is absolute so the lookup does not
# depend on the working directory the container is started in.
COPY migrations/ /app/migrations/
ENV MIGRATIONS_DIR=/app/migrations
EXPOSE 8080
# The entrypoint generates and reuses an RS256 keypair for JWT signing.
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
