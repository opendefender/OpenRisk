 CI/CD Pipeline Documentation

 Overview

OpenRisk includes a comprehensive CI/CD pipeline using GitHub Actions to ensure code quality, test coverage, and automated deployment.

 Pipeline Stages

 . Linting (Parallel)
- Backend: golangci-lint - Static analysis for Go code
- Frontend: ESLint - JavaScript/TypeScript linting
- Frontend: TypeScript type checking (tsc)

 . Unit Tests (Parallel)
- Backend: go test -v ./... with coverage reporting
- Frontend: Jest tests with coverage reporting

 . Integration Tests
- Requires test database (PostgreSQL )
- Runs full handler integration tests
- Tests database interactions

 . Build
- Backend binary compilation
- Frontend build (npm run build)

 . Docker Image Build & Push
- Multi-stage build (backend + frontend)
- Pushes to GitHub Container Registry (GHCR)
- Only on main/stag branch

 Running Locally

 Prerequisites
bash
 Backend
- Go .+
- golangci-lint
- PostgreSQL +

 Frontend
- Node.js +
- npm

 Docker
- Docker & Docker Compose


 Unit Tests
bash
 Backend
cd backend && go test -v ./...

 Frontend
cd frontend && npm test

 Both with Makefile
make test


 Integration Tests
bash
 Requires docker-compose
./scripts/run-integration-tests.sh

 Or with make
make test-integration


 Linting
bash
 All
make lint

 Backend only
cd backend && golangci-lint run ./...

 Frontend only
cd frontend && npm run lint


 Docker Build
bash
 Build image
make docker-build

 Or with Docker directly
docker build -t openrisk:latest .

 Run container
docker run -p : openrisk:latest


 GitHub Actions Workflow

Located in .github/workflows/ci.yml

 Triggers
- push to: main, stag, develop
- pull_request to: main, stag, develop

 Environment Variables
- REGISTRY: ghcr.io
- IMAGE_NAME: ${{ github.repository }}

 Secrets Required
- GITHUB_TOKEN - Automatically provided by GitHub

 Build Matrix
Runs on ubuntu-latest

 Status Checks
- Must pass all checks before merge
- Coverage reports uploaded to Codecov
- Docker image pushed to GHCR on main/stag

 Coverage Goals

- Backend: Target %+ coverage
- Frontend: Target %+ coverage
- Coverage reports available on Codecov

 Docker Image Details

 Build Process
. Stage : Go builder - Compiles backend binary
. Stage : Node builder - Builds frontend (React + Vite)
. Stage : Alpine runtime - Final production image

 Image Tags
- main branch → latest
- stag branch → stag
- develop branch → develop
- Git tags → Semantic version

 Registry
- GitHub Container Registry (GHCR)
- URL: ghcr.io/alex-dembele/openrisk

 Image Size
- ~MB (optimized Alpine base)
- Non-root user (openrisk:openrisk)
- Health check enabled

 Troubleshooting

 Tests failing locally but passing in CI
- Ensure Go .+ installed
- Run go mod tidy in backend
- Check database connection (for integration tests)

 Docker build fails
- Ensure backend/go.mod is valid: go mod verify
- Check frontend/package.json for errors
- Verify Dockerfile syntax: docker build --no-cache .

 Integration tests timeout
- Ensure Docker is running
- Check available disk space (needs ~GB)
- Verify test_db service health: docker-compose logs test_db

 Next Steps

. Configure Codecov integration for coverage tracking
. Add performance benchmarks to CI
. Implement security scanning (trivy, snyk)
. Add SAST (SonarQube, CodeQL)
. Implement artifact retention policies

 Related Files
- .github/workflows/ci.yml - CI pipeline configuration
- Dockerfile - Multi-stage container build
- docker-compose.yaml - Local development environment
- Makefile - Development task automation
- scripts/run-integration-tests.sh - Local integration test runner
