#!/usr/bin/env bash
# OpenRisk — restore from a backup tarball produced by scripts/backup.sh.
# ⚠️ This OVERWRITES the current database, secrets and .env.
#
#   ./scripts/restore.sh path/to/openrisk-backup-YYYY...tar.gz
set -euo pipefail

[ $# -ge 1 ] || { echo "usage: $0 <backup.tar.gz>"; exit 1; }
BACKUP="$1"
[ -f "$BACKUP" ] || { echo "no such file: $BACKUP"; exit 1; }

HERE="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_DIR="$HERE/deploy/selfhost"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

tar -C "$WORK" -xzf "$BACKUP"
cd "$COMPOSE_DIR"
if docker compose version >/dev/null 2>&1; then DC="docker compose"; else DC="docker-compose"; fi

echo "[restore] restoring secrets + .env ..."
cp -a "$WORK/.env" .env
rm -rf secrets && cp -a "$WORK/secrets" secrets

echo "[restore] starting database ..."
$DC up -d db
# Wait for it to accept connections.
DB_USER="$(grep -E '^DB_USER=' .env | cut -d= -f2)"; DB_USER="${DB_USER:-openrisk}"
DB_NAME="$(grep -E '^DB_NAME=' .env | cut -d= -f2)"; DB_NAME="${DB_NAME:-openrisk}"
for i in $(seq 1 30); do
  $DC exec -T db pg_isready -U "$DB_USER" >/dev/null 2>&1 && break
  sleep 2
done

echo "[restore] loading database dump ..."
$DC exec -T db psql -U "$DB_USER" -d "$DB_NAME" < "$WORK/database.sql"

if [ -f "$WORK/exports.tgz" ]; then
  echo "[restore] restoring data exports ..."
  $DC run --rm -T -v "$WORK:/backup" --entrypoint sh backend -c \
    'mkdir -p /app/uploads/exports && cd /app/uploads/exports && tar xzf /backup/exports.tgz' >/dev/null 2>&1 || true
fi

echo "[restore] bringing the full stack up ..."
$DC up -d
echo "[restore] ✅ done."
