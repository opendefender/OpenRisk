#!/usr/bin/env bash
# A throwaway, isolated copy of the stack for one issue (#1294), so an agent can verify a
# change live before it pushes. It never touches the owner's stacks (rule 6.2): its
# Compose project, ports, volumes and env file are its own, and the env file lives in a
# scratchpad directory, the only place `down -v` is allowed by the guard.
#
#   scripts/runner/live-stack.sh up   <issue>   build and start, wait until healthy
#   scripts/runner/live-stack.sh test <issue>   run the E2E PR-gate specs against it
#   scripts/runner/live-stack.sh down <issue>   remove it, volumes included
#   scripts/runner/live-stack.sh urls <issue>   print its URLs
set -euo pipefail
cmd=${1:?up|test|down|urls}; n=${2:?issue number}
[[ "$n" =~ ^[0-9]+$ ]] || { echo "issue must be a number" >&2; exit 2; }
ROOT="$(git rev-parse --show-toplevel)"
SCR="${OPENRISK_SCRATCH:-$HOME/.local/state/openrisk-runner/scratchpad}"
mkdir -p "$SCR"; chmod 700 "$SCR"
ENV="$SCR/live-$n.env"; OVR="$SCR/live-$n.override.yaml"
P="openrisk-live-$n"
# Ports from the issue number, away from dev (5173/8080…) and staging (3001/8082…).
o=$(( (n % 400) * 10 + 20000 ))
FE=$((o+1)); BE=$((o+2)); DB=$((o+3)); TDB=$((o+4)); RD=$((o+5))

if [ ! -f "$ENV" ]; then
  umask 077
  { echo "DB_PASSWORD=$(openssl rand -hex 16)"; echo "INITIAL_ADMIN_PASSWORD=Live-$(openssl rand -hex 12)!"
    echo "OPENRISK_DEV_VOLUMES=$P"; echo "OPENRISK_DEV_FRONTEND_PORT=$FE"; echo "OPENRISK_DEV_BACKEND_PORT=$BE"
    echo "OPENRISK_DEV_DB_PORT=$DB"; echo "OPENRISK_DEV_TEST_DB_PORT=$TDB"; echo "OPENRISK_DEV_REDIS_PORT=$RD"
    echo "CORS_ORIGINS=http://localhost:$FE"; } > "$ENV"
  # APP_BASE_URL: links the backend builds (invitations, resets) must point at THIS stack, never
  # at the owner's dev frontend on :5173 (found on the first live run, #1294).
  printf 'services:\n  backend:\n    environment:\n      INITIAL_ADMIN_PASSWORD: ${INITIAL_ADMIN_PASSWORD}\n      APP_BASE_URL: http://localhost:%s\n' "$FE" > "$OVR"
fi
C=(docker compose -p "$P" -f "$ROOT/docker-compose.yaml" -f "$OVR" --env-file "$ENV")

case "$cmd" in
  up)   "${C[@]}" up -d --build --wait --wait-timeout 600 >/dev/null
        curl -fsS -o /dev/null "http://localhost:$BE/api/v1/health" && echo "live stack $P up: frontend http://localhost:$FE, API http://localhost:$BE/api/v1" ;;
  test) set -a; . "$ENV"; set +a
        cd "$ROOT"
        [ -d node_modules/@playwright/test ] || npm ci --silent --no-audit --no-fund
        npx playwright install chromium >/dev/null
        mapfile -t specs < <(grep -Ev '^[[:space:]]*(#|$)' tests/e2e/pr-gate.txt)
        E2E_NO_WEBSERVER=1 E2E_BASE_URL="http://localhost:$FE" E2E_API_URL="http://localhost:$BE/api/v1" \
        E2E_ADMIN_EMAIL=admin@opendefender.io E2E_ADMIN_PASSWORD="$INITIAL_ADMIN_PASSWORD" \
          npx playwright test --project=chromium "${specs[@]}" --reporter=line ;;
  down) "${C[@]}" down -v --remove-orphans >/dev/null 2>&1 || true; rm -f "$ENV" "$OVR"; echo "live stack $P removed" ;;
  urls) echo "frontend http://localhost:$FE  API http://localhost:$BE/api/v1" ;;
  *) echo "unknown command $cmd" >&2; exit 2 ;;
esac
