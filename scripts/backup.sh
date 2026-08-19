#!/usr/bin/env bash
# OpenRisk — backup the database + secrets + generated exports.
# Produces a single timestamped tarball you can move off-box.
#
#   ./scripts/backup.sh [output-dir]
set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_DIR="$HERE/deploy/selfhost"
OUT_DIR="${1:-$HERE/backups}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

cd "$COMPOSE_DIR"
if docker compose version >/dev/null 2>&1; then DC="docker compose"; else DC="docker-compose"; fi

DB_USER="$(grep -E '^DB_USER=' .env | cut -d= -f2)"; DB_USER="${DB_USER:-openrisk}"
DB_NAME="$(grep -E '^DB_NAME=' .env | cut -d= -f2)"; DB_NAME="${DB_NAME:-openrisk}"

echo "[backup] dumping database $DB_NAME ..."
$DC exec -T db pg_dump -U "$DB_USER" -d "$DB_NAME" --clean --if-exists > "$WORK/database.sql"

echo "[backup] copying secrets + .env ..."
cp -a secrets "$WORK/secrets"
cp -a .env "$WORK/.env"

echo "[backup] exporting generated data exports volume ..."
$DC run --rm -T -v "$WORK:/backup" --entrypoint sh backend -c \
  'cd /app/uploads/exports 2>/dev/null && tar czf /backup/exports.tgz . || true' >/dev/null 2>&1 || true

mkdir -p "$OUT_DIR"
TARBALL="$OUT_DIR/openrisk-backup-$STAMP.tar.gz"
tar -C "$WORK" -czf "$TARBALL" .
echo "[backup] ✅ wrote $TARBALL"
echo "[backup] store this off-box; it contains your DB and encryption keys."
