#!/usr/bin/env bash
# Run the live cross-tenant suite against a booted server on Postgres (#1319).
#
# Two tenants are registered through the real API and tenant A's token is pointed
# at tenant B's ids across every probed route (backend/internal/security/
# crosstenant). The server is the real binary, booted by scripts/ci/build-schema.sh
# in its keep-running mode, so the suite sees the production middleware chain, the
# production route table and the Postgres-only behaviour of the queries.
#
# It runs in its own job, never inside `go test -tags=integration ./...`: that run
# starts no server, so every live test would skip and look green.
#
# Test-only environment (D-129): the suite sends roughly 1.5k requests from one IP
# and one tenant, far over the default limits (300/min per IP, 1200/min per
# tenant), so the per-minute limits are raised and the auth limiters are off
# (RATE_LIMIT_AUTH_DISABLED is a non-production knob, #588). The tenant guard stays
# on `enforce` (build-schema.sh sets it) and the MFA policy stays as production
# ships it: the suite does not relax what it is testing.
#
# Input (environment): the same as build-schema.sh (DATABASE_URL, DB_*, REDIS_*,
# PORT), plus
#   CROSS_TENANT_LOG   where the server's boot log is copied (for the CI artifact);
#                      default ${TMPDIR:-/tmp}/cross-tenant-boot.log
#
# Exit status: 0 only when the suite passed AND the boot log holds no tenantguard
# line (a query that reached the database without a tenant filter, #808). The boot
# log holds no secrets: the key pair is throwaway and never printed.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PORT="${PORT:-8080}"
LOG_OUT="${CROSS_TENANT_LOG:-${TMPDIR:-/tmp}/cross-tenant-boot.log}"

WORK="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/cross-tenant.XXXXXX")"
chmod 700 "$WORK"
cleanup() {
  if [ -s "$WORK/openrisk.pid" ]; then
    kill "$(cat "$WORK/openrisk.pid")" 2>/dev/null || true
  fi
  if [ -f "$WORK/boot.log" ]; then cp "$WORK/boot.log" "$LOG_OUT" || true; fi
  # $WORK is a mktemp directory created above, never empty.
  rm -rf -- "${WORK:?}"
}
trap cleanup EXIT

export PORT
export RATE_LIMIT_IP_PER_MINUTE="${RATE_LIMIT_IP_PER_MINUTE:-100000}"
export RATE_LIMIT_TENANT_PER_MINUTE="${RATE_LIMIT_TENANT_PER_MINUTE:-100000}"
export RATE_LIMIT_AUTH_DISABLED=true
export BUILD_SCHEMA_KEEP_RUNNING=1
export BUILD_SCHEMA_WORKDIR="$WORK"

# One boot builds the schema and leaves the server running.
"$ROOT/scripts/ci/build-schema.sh"

STATUS=0
(
  cd "$ROOT/backend"
  OPENRISK_BASE_URL="http://localhost:${PORT}" \
    go test -tags crosstenant ./internal/security/crosstenant/ -run 'TestCrossTenant' -count=1 -v
) || STATUS=$?

# Guard silence: the guard logs a refusal and the handler may swallow the error,
# so the suite can pass on a route that queried without a tenant. The log is the
# evidence.
if grep -n 'tenantguard' "$WORK/boot.log"; then
  echo "the run made a query the tenant guard refused (#808): see the lines above" >&2
  STATUS=1
fi
exit "$STATUS"
