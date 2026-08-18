# Self-hosting OpenRisk

Self-hosting is a first-class product and our main acquisition channel. The goal:
**one command, works on the first try, zero manual steps.** A monthly CI job
([`.github/workflows/selfhost-install.yml`](../.github/workflows/selfhost-install.yml))
provisions a fresh Ubuntu 24.04 VM and runs this exact flow end-to-end.

## Requirements

- Docker Engine + Docker Compose plugin
- `openssl` (for secret/key generation)
- ~2 GB RAM, 2 vCPU, 10 GB disk to start
- Verified clean on **Ubuntu 24.04** and **Debian 12**

## Install (one command)

```bash
curl -fsSL https://raw.githubusercontent.com/opendefender/OpenRisk/master/scripts/install.sh | bash
```

Or from a clone:

```bash
git clone https://github.com/opendefender/OpenRisk.git
cd OpenRisk
./scripts/install.sh
```

The installer:

1. checks Docker + Compose,
2. generates an **RS256 keypair** (`deploy/selfhost/secrets/`) — the backend
   requires this and its absence is the classic "manual step" we remove,
3. writes a `.env` with **strong random secrets** (DB password, MFA/scanner/audit
   keys),
4. builds and starts postgres + redis + backend + frontend,
5. waits for `/api/v1/health` and prints the URLs.

It is **idempotent**: re-running keeps your `.env` and keys.

App: `http://localhost:3000` · API: `http://localhost:8080/api/v1`.

### Configuration

Everything lives in `deploy/selfhost/.env` (template: `.env.example`). Notable
optional keys:

- **Payments:** `STRIPE_SECRET_KEY`, `NOTCHPAY_PUBLIC_KEY`, `CINETPAY_API_KEY` +
  `CINETPAY_SITE_ID`. Empty ⇒ Free plan + manual/sales upgrades (no fake URLs).
- **Telemetry:** `OPENRISK_TELEMETRY=off` hard-disables telemetry (see
  [TELEMETRY.md](TELEMETRY.md)). Default: opt-in from the app.
- **Public URL / CORS:** `APP_BASE_URL`, `CORS_ORIGINS`, `VITE_API_URL` when you
  put OpenRisk behind a real domain / reverse proxy.

## Upgrade

```bash
cd OpenRisk
git pull
./scripts/backup.sh                 # always back up first
cd deploy/selfhost
docker compose pull || true         # if using prebuilt images
docker compose up -d --build        # rebuild + restart; migrations run on boot
```

Schema migrations (GORM AutoMigrate + SQL migrations) run automatically on
backend start. Your `.env` and `secrets/` are preserved across upgrades.

## Backup & restore

```bash
./scripts/backup.sh                 # → backups/openrisk-backup-<stamp>.tar.gz
./scripts/restore.sh backups/openrisk-backup-<stamp>.tar.gz
```

The tarball contains the Postgres dump, the `secrets/` (encryption keys) and the
generated data exports. **Store it off-box** — it holds your keys. Restore
overwrites the current database and secrets.

## ARM64 (AWS Graviton, Apple Silicon, Raspberry Pi)

Base images (`postgres`, `redis`, `alpine`, `golang`) are multi-arch, so the
stack builds and runs natively on ARM64 — `./scripts/install.sh` works unchanged
on a Graviton VM. To build and publish a multi-arch image:

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -t openrisk/backend:latest ./backend --push
docker buildx build --platform linux/amd64,linux/arm64 \
  -t openrisk/frontend:latest ./frontend --push
```

## Kubernetes (Helm)

A maintained chart lives in [`helm/openrisk`](../helm/openrisk) with
`values-dev.yaml` / `values-staging.yaml` / `values-prod.yaml`. It schedules on
ARM64 nodes when you provide ARM64 images (above). Provide the same secrets
(`RSA_*`, `MFA_ENCRYPTION_KEY`, `SCANNER_CREDENTIAL_KEY`, `AUDIT_EXPORT_KEY`) and
optional payment/telemetry env via the chart's `values` / a `Secret`.

## Troubleshooting

- **Backend restarts / "RSA keys required":** the `secrets/` keypair is missing —
  re-run `./scripts/install.sh` (it regenerates only what's absent).
- **Port already in use:** set `BACKEND_PORT` / `FRONTEND_PORT` in `.env`.
- **Logs:** `cd deploy/selfhost && docker compose logs -f backend`.
