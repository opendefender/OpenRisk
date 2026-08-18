#!/usr/bin/env bash
# OpenRisk — one-command self-host installer.
#
# Tested on a clean Ubuntu 24.04 and Debian 12 VM (see the monthly CI job
# .github/workflows/selfhost-install.yml, which provisions a fresh runner and
# runs this end-to-end). It is idempotent: re-running keeps your existing .env
# and keys.
#
#   curl -fsSL https://raw.githubusercontent.com/opendefender/OpenRisk/master/scripts/install.sh | bash
# or, from a clone:
#   ./scripts/install.sh
set -euo pipefail

REPO_URL="${OPENRISK_REPO_URL:-https://github.com/opendefender/OpenRisk.git}"
BRANCH="${OPENRISK_BRANCH:-master}"

log()  { printf '\033[1;36m[openrisk]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[openrisk]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[openrisk]\033[0m %s\n' "$*" >&2; exit 1; }

# --- 1. Prerequisites --------------------------------------------------------
command -v docker >/dev/null 2>&1 || die "Docker is not installed. See https://docs.docker.com/engine/install/"
if docker compose version >/dev/null 2>&1; then
  DC="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  DC="docker-compose"
else
  die "Docker Compose plugin is not installed. See https://docs.docker.com/compose/install/"
fi
docker info >/dev/null 2>&1 || die "Cannot talk to the Docker daemon (is it running / do you have permission?)."

# --- 2. Locate the repo (clone if piped from curl) ---------------------------
if [ -f "deploy/selfhost/docker-compose.yml" ]; then
  ROOT="$(pwd)"
elif [ -f "$(dirname "$0")/../deploy/selfhost/docker-compose.yml" ]; then
  ROOT="$(cd "$(dirname "$0")/.." && pwd)"
else
  command -v git >/dev/null 2>&1 || die "git is required to fetch OpenRisk."
  ROOT="${OPENRISK_DIR:-$HOME/openrisk}"
  if [ ! -d "$ROOT/.git" ]; then
    log "Cloning OpenRisk into $ROOT ..."
    git clone --depth 1 --branch "$BRANCH" "$REPO_URL" "$ROOT"
  fi
fi
cd "$ROOT/deploy/selfhost"
log "Deployment directory: $(pwd)"

# --- 3. Secret generation helpers -------------------------------------------
rand_hex() { openssl rand -hex "${1:-32}"; }
rand_b64_32() { openssl rand -base64 32; }
command -v openssl >/dev/null 2>&1 || die "openssl is required to generate secrets."

# --- 4. RSA keypair for RS256 JWTs ------------------------------------------
mkdir -p secrets
if [ ! -f secrets/private.pem ]; then
  log "Generating RS256 keypair ..."
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out secrets/private.pem 2>/dev/null
  openssl rsa -in secrets/private.pem -pubout -out secrets/public.pem 2>/dev/null
  chmod 600 secrets/private.pem
else
  log "RSA keypair already present — keeping it."
fi

# --- 5. .env with strong random secrets -------------------------------------
if [ ! -f .env ]; then
  log "Creating .env with generated secrets ..."
  cp .env.example .env
  # Fill the empty required fields with generated values (portable sed).
  set_kv() { # key value
    if grep -q "^$1=" .env; then
      # Use a temp file so any char in $2 is safe.
      awk -v k="$1" -v v="$2" 'BEGIN{FS=OFS="="} $1==k{$0=k"="v} {print}' .env > .env.tmp && mv .env.tmp .env
    else
      printf '%s=%s\n' "$1" "$2" >> .env
    fi
  }
  set_kv DB_PASSWORD "$(rand_hex 24)"
  set_kv MFA_ENCRYPTION_KEY "$(rand_hex 32)"       # 32 bytes hex → 64 chars; backend takes 32 bytes
  set_kv SCANNER_CREDENTIAL_KEY "$(rand_b64_32)"
  set_kv AUDIT_EXPORT_KEY "$(rand_b64_32)"
  chmod 600 .env
else
  log ".env already present — keeping your configuration."
fi

# --- 6. Bring the stack up ---------------------------------------------------
log "Building and starting the stack (this can take a few minutes on first run) ..."
$DC up -d --build

# --- 7. Wait for backend health ---------------------------------------------
PORT="$(grep -E '^BACKEND_PORT=' .env | cut -d= -f2)"; PORT="${PORT:-8080}"
FPORT="$(grep -E '^FRONTEND_PORT=' .env | cut -d= -f2)"; FPORT="${FPORT:-3000}"
log "Waiting for the backend to become healthy on :$PORT ..."
for i in $(seq 1 60); do
  if curl -fsS "http://localhost:${PORT}/api/v1/health" >/dev/null 2>&1; then
    log "Backend is healthy."
    log "✅ OpenRisk is up."
    log "   • App:  http://localhost:${FPORT}"
    log "   • API:  http://localhost:${PORT}/api/v1"
    log "   • Logs: (cd deploy/selfhost && $DC logs -f)"
    exit 0
  fi
  sleep 3
done
warn "Backend did not report healthy in time. Inspect logs with:"
warn "  (cd deploy/selfhost && $DC logs backend)"
exit 1
