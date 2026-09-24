# CI/CD Pipeline Documentation

## Overview

OpenRisk includes a comprehensive CI/CD pipeline using GitHub Actions to ensure code quality, test coverage, and automated deployment.

## Pipeline Stages

### 1. **Linting** (Parallel)
- **Backend**: `golangci-lint` - Static analysis for Go code
- **Frontend**: `ESLint` - JavaScript/TypeScript linting
- **Frontend**: TypeScript type checking (`tsc`)

### 2. **Unit Tests** (Parallel)
- **Backend**: `go test -v ./...` with coverage reporting
- **Frontend**: Jest tests with coverage reporting

### 3. **Integration Tests**
- Requires test database (PostgreSQL 15)
- Runs full handler integration tests
- Tests database interactions

### 4. **Build**
- Backend binary compilation
- Frontend build (`npm run build`)

### 5. **Docker Image Build & Push**
- Multi-stage build (backend + frontend)
- Pushes to GitHub Container Registry (GHCR)
- Only on main/stag branch

## Running Locally

### Prerequisites
```bash
# Backend
- Go 1.21+
- golangci-lint
- PostgreSQL 15+

# Frontend
- Node.js 18+
- npm

# Docker
- Docker & Docker Compose
```

### Unit Tests
```bash
# Backend
cd backend && go test -v ./...

# Frontend
cd frontend && npm test

# Both with Makefile
make test
```

### Integration Tests
```bash
# Requires docker-compose
./scripts/run-integration-tests.sh

# Or with make
make test-integration
```

### Linting
```bash
# All
make lint

# Backend only
cd backend && golangci-lint run ./...

# Frontend only
cd frontend && npm run lint
```

### Docker Build
```bash
# Build image
make docker-build

# Or with Docker directly
docker build -t openrisk:latest .

# Run container
docker run -p 8080:8080 openrisk:latest
```

## End-to-end tests (Playwright)

Located in `.github/workflows/e2e.yml`. Two jobs:

- **E2E (chromium + Mobile Chrome)** runs on every push and PR to `master`/`develop`, and
  on the nightly schedule.
  - **On pushes and PRs** it runs only the curated blocking set listed in
    `tests/e2e/pr-gate.txt`, within 25 minutes.
  - **On the schedule** it runs the whole suite, with a 120-minute budget.

  This is owner decision D-043 (option B). A spec joins the blocking set once it passes
  in CI on both projects. It is held out only with an issue number, and re-admitted when
  that issue makes it green. A path in the list that does not exist fails the job.
- **E2E nightly (firefox + webkit)** runs on the schedule only.

### What the PR job does

1. **Services:** it starts PostgreSQL 16 and Redis 7 as job services.
2. **Backend:** it builds `backend/` and generates an **ephemeral RS256 key pair** in
   `$RUNNER_TEMP/e2e-keys`, whose paths go to the backend as `RSA_PRIVATE_KEY_PATH` /
   `RSA_PUBLIC_KEY_PATH`. The backend refuses to boot without RS256 keys: before #297 this
   job gave it none, and it panicked before a single test ran. The pair is never committed
   and never printed, and it is outside `tests/e2e/.artifacts`, the only uploaded path.
3. **Readiness:** it starts the backend on `:8080` and the Vite dev server on `:5173`, then
   waits for both. A backend that never becomes healthy fails the job.
4. **Tests:** it runs the specs listed in `tests/e2e/pr-gate.txt` with
   `--project=chromium --project="Mobile Chrome"`, or every spec on the schedule. The CI
   settings are one worker and two retries (`playwright.config.ts`).

`tests/e2e/global-setup.ts` does the rest before any spec:

- **Seed:** it runs `scripts/seed-e2e.mjs`, which also enrols the admin in MFA and records
  the TOTP secret in `tests/e2e/.seed-ids.json`.
- **Sessions:** it signs each persona in through the API (`tests/e2e/support/auth.ts`).
  That helper completes the MFA challenge with a built-in RFC 6238 TOTP, and stores the
  real HttpOnly session cookies as a `storageState` in `tests/e2e/.auth/`.
- **Product tour:** the `storageState` marks the tour as seen
  (`openrisk_tour_seen_v1`), because its coach-mark card otherwise sits over the screens
  the specs drive.

**Sign-in budget.** `/auth/login` and `/auth/register` allow **15 requests per IP every
5 minutes** (`backend/cmd/server/main.go`, `authRateLimit`), and the whole suite runs from
one IP. A spec should sign in once per worker, not once per test:
`journey.members.spec.ts` memoises its admin API context for that reason. A test failing
with `login failed … 429 Rate limit exceeded` has hit this budget, not a login bug.

### Running one spec locally, as CI does

Ports 5432 and 6379 are often taken by host services, so this uses other ports:

```bash
# Services
docker run -d --name or-e2e-pg -e POSTGRES_USER=openrisk -e POSTGRES_PASSWORD=openrisk \
  -e POSTGRES_DB=openrisk_e2e -p 55432:5432 postgres:16-alpine
docker run -d --name or-e2e-redis -p 56379:6379 redis:7-alpine

# One admin password for this session, shared by the backend and the harness.
# There is no default (#485).
export OR_E2E_ADMIN_PASSWORD="$(openssl rand -hex 24)"

# Throwaway key pair, outside the repository
mkdir -p /tmp/or-e2e-keys
( umask 077 && openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out /tmp/or-e2e-keys/private.pem )
openssl rsa -in /tmp/or-e2e-keys/private.pem -pubout -out /tmp/or-e2e-keys/public.pem

# Backend (:8080), from backend/
export DATABASE_URL="postgres://openrisk:openrisk@localhost:55432/openrisk_e2e?sslmode=disable" \
  DB_HOST=localhost DB_PORT=55432 DB_USER=openrisk DB_PASSWORD=openrisk DB_NAME=openrisk_e2e \
  REDIS_HOST=localhost REDIS_PORT=56379 APP_ENV=test PORT=8080 \
  CORS_ORIGINS=http://localhost:5173 \
  INITIAL_ADMIN_PASSWORD="$OR_E2E_ADMIN_PASSWORD" MIGRATIONS_DIR=../migrations \
  RSA_PRIVATE_KEY_PATH=/tmp/or-e2e-keys/private.pem RSA_PUBLIC_KEY_PATH=/tmp/or-e2e-keys/public.pem
go build -o /tmp/or-e2e-openrisk ./cmd/server && /tmp/or-e2e-openrisk &

# Frontend (:5173), from the repository root
npm --prefix frontend run dev -- --port 5173 --strictPort &

# One spec, both PR projects, one worker, as CI
export E2E_NO_WEBSERVER=1 E2E_BASE_URL=http://localhost:5173 E2E_API_URL=http://localhost:8080/api/v1 \
  E2E_ADMIN_EMAIL=admin@opendefender.io E2E_ADMIN_PASSWORD="$OR_E2E_ADMIN_PASSWORD"
npx playwright test tests/e2e/journey.members.spec.ts --project=chromium --project="Mobile Chrome" --workers=1

# The whole PR blocking set, as the PR job selects it
npx playwright test $(grep -Ev '^[[:space:]]*(#|$)' tests/e2e/pr-gate.txt) \
  --project=chromium --project="Mobile Chrome" --workers=1
```

Several runs in a row exhaust the sign-in budget above. Wait five minutes, or reset the
limiter with `docker exec or-e2e-redis redis-cli FLUSHALL`. Only do that on this
throwaway Redis.

## GitHub Actions Workflow

Located in `.github/workflows/ci.yml`

### Triggers
- `push` to: `main`, `stag`, `develop`
- `pull_request` to: `main`, `stag`, `develop`

### Environment Variables
- `REGISTRY`: `ghcr.io`
- `IMAGE_NAME`: `${{ github.repository }}`

### Secrets Required
- `GITHUB_TOKEN` - Automatically provided by GitHub

### Build Matrix
Runs on `ubuntu-latest`

### Status Checks
- Must pass all checks before merge
- Coverage reports uploaded to Codecov
- Docker image pushed to GHCR on main/stag

## Coverage Goals

- **Backend**: Target 60%+ coverage
- **Frontend**: Target 50%+ coverage
- Coverage reports available on Codecov

## Docker Image Details

### Build Process
1. **Stage 1**: Go builder - Compiles backend binary
2. **Stage 2**: Node builder - Builds frontend (React + Vite)
3. **Stage 3**: Alpine runtime - Final production image

### Image Tags
- `main` branch → `latest`
- `stag` branch → `stag`
- `develop` branch → `develop`
- Git tags → Semantic version

### Registry
- GitHub Container Registry (GHCR)
- URL: `ghcr.io/alex-dembele/openrisk`

### Image Size
- ~150MB (optimized Alpine base)
- Non-root user (openrisk:openrisk)
- Health check enabled

## Troubleshooting

### Tests failing locally but passing in CI
- Ensure Go 1.21+ installed
- Run `go mod tidy` in backend
- Check database connection (for integration tests)

### Docker build fails
- Ensure backend/go.mod is valid: `go mod verify`
- Check frontend/package.json for errors
- Verify Dockerfile syntax: `docker build --no-cache .`

### Integration tests timeout
- Ensure Docker is running
- Check available disk space (needs ~5GB)
- Verify test_db service health: `docker-compose logs test_db`

## Next Steps

1. Configure Codecov integration for coverage tracking
2. Add performance benchmarks to CI
3. Implement security scanning (trivy, snyk)
4. Add SAST (SonarQube, CodeQL)
5. Implement artifact retention policies

## Related Files
- `.github/workflows/ci.yml` - CI pipeline configuration
- `Dockerfile` - Multi-stage container build
- `docker-compose.yaml` - Local development environment
- `Makefile` - Development task automation
- `scripts/run-integration-tests.sh` - Local integration test runner
