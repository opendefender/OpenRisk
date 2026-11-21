#!/usr/bin/env bash
# Build the database schema with one server boot (#1491).
#
# The *_Postgres integration tests read the real schema (public.mfa_secrets,
# audit_events, automation_rules, ...), which only exists once the server has
# booted: it runs AutoMigrate and then the SQL migrations. Against an empty
# database those tests fail with "relation ... does not exist". So the schema is
# built the way production builds it: boot the server once, stop it as soon as
# /api/v1/health answers.
#
# This is the single source for that step. CI's backend-integration-tests job
# and `make test-integration` both call it, so a local run means what CI means.
#
# Run it from anywhere; it boots the server from the repository root, like CI.
#
# Input (environment), defaults match the CI services:
#   DATABASE_URL, DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
#   REDIS_HOST, REDIS_PORT, REDIS_URL
#   PORT                  port the server listens on during the boot (8080)
#   BUILD_SCHEMA_TIMEOUT  seconds to wait for /health once started (90)
#   RUNNER_TEMP           work directory parent (CI sets it; else mktemp)
#
# Exit status: 0 once the schema is built; 1 when the server does not come up
# (the last 50 lines of the boot log are printed) or when the boot log holds a
# tenantguard line (#808).
#
# The RSA key pair is throwaway: generated under umask 077 in a private work
# directory, never printed, and deleted on exit.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

export DATABASE_URL="${DATABASE_URL:-postgres://test:test@localhost:5432/openrisk_test?sslmode=disable}"
export DB_HOST="${DB_HOST:-localhost}"
export DB_PORT="${DB_PORT:-5432}"
export DB_USER="${DB_USER:-test}"
export DB_PASSWORD="${DB_PASSWORD:-test}"
export DB_NAME="${DB_NAME:-openrisk_test}"
export REDIS_HOST="${REDIS_HOST:-localhost}"
export REDIS_PORT="${REDIS_PORT:-6379}"
export REDIS_URL="${REDIS_URL:-redis://${REDIS_HOST}:${REDIS_PORT}}"
export PORT="${PORT:-8080}"
TIMEOUT="${BUILD_SCHEMA_TIMEOUT:-90}"

# Fixed: the boot always runs in test mode, offline, under the tenant guard.
export APP_ENV=test
export CTI_SYNC_ENABLED=false
export PASSWORD_HIBP_DISABLED=true
export OPENRISK_DEPLOYMENT=self_hosted
# #808: the production boot path (seeds, backfills) runs under the tenant guard.
export TENANT_GUARD=enforce

WORK="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/build-schema.XXXXXX")"
chmod 700 "$WORK"
PID=""
cleanup() {
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT

# Something already answering on the port would make the health check lie.
if curl -fsS --max-time 2 "http://localhost:${PORT}/api/v1/health" >/dev/null 2>&1; then
  echo "port ${PORT} already answers /api/v1/health; stop that server or set PORT" >&2
  exit 1
fi

mkdir -m 700 "$WORK/keys"
( umask 077 && openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$WORK/keys/private.pem" 2>/dev/null )
( umask 077 && openssl rsa -in "$WORK/keys/private.pem" -pubout -out "$WORK/keys/public.pem" 2>/dev/null )
export RSA_PRIVATE_KEY_PATH="$WORK/keys/private.pem" RSA_PUBLIC_KEY_PATH="$WORK/keys/public.pem"

(cd "$ROOT/backend" && go build -o "$WORK/openrisk" ./cmd/server)

cd "$ROOT"
"$WORK/openrisk" > "$WORK/boot.log" 2>&1 &
PID=$!
START=$SECONDS
while [ $((SECONDS - START)) -lt "$TIMEOUT" ]; do
  if curl -fsS --max-time 2 "http://localhost:${PORT}/api/v1/health" >/dev/null 2>&1; then
    kill "$PID" 2>/dev/null || true; wait "$PID" || true
    PID=""
    # #808: seeds and backfills swallow their errors and log them, so the
    # tenant guard's refusals are only visible here. Any line fails the step.
    if grep -n 'tenantguard' "$WORK/boot.log"; then
      echo "the boot ran a query the tenant guard refused (#808)" >&2
      exit 1
    fi
    echo "schema built after $((SECONDS - START))s"
    exit 0
  fi
  kill -0 "$PID" 2>/dev/null || break
  sleep 1
done
tail -50 "$WORK/boot.log"
echo "the server exited or did not answer /api/v1/health within ${TIMEOUT}s, so the schema was not built" >&2
exit 1
