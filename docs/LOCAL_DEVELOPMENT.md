 OpenRisk Local Development Setup Guide

This guide will help you set up OpenRisk for local development using Docker Compose.

 Prerequisites

- Docker (version .+)
- Docker Compose (version .+)
- Git
- Go .+ (optional, for native backend development)
- Node.js + (optional, for native frontend development)

 Quick Start (Recommended)

 . Clone the Repository

bash
git clone https://github.com/opendefender/openrisk.git
cd openrisk


 . Configure Environment

bash
 Copy the example environment file
cp .env.example .env

 Edit the .env file if needed (default values work for local development)
 The defaults are configured to work with docker-compose out of the box


 . Start All Services

bash
 Start all services (database, redis, backend, frontend)
docker-compose up -d

 Watch logs (optional)
docker-compose logs -f


 . Verify Services

bash
 Check all services are running
docker-compose ps

 Expected output:
 NAME                    STATUS
 openrisk_db             Up (healthy)
 openrisk_test_db       Up (healthy)
 openrisk_redis         Up (healthy)
 openrisk_backend       Up (healthy)
 openrisk_frontend      Up (healthy)


 . Access the Application

- Frontend: http://localhost:
- Backend API: http://localhost:/api/v
- Health Check: http://localhost:/api/v/health

 Service Breakdown

 Database Services

Production Database (openrisk_db)
- Type: PostgreSQL 
- Host: localhost
- Port: 
- Credentials: openrisk / openrisk
- Database: openrisk
- Connection: postgres://openrisk:openrisk@localhost:/openrisk

Test Database (openrisk_test_db)
- Type: PostgreSQL 
- Host: localhost
- Port: 
- Credentials: test / test
- Database: openrisk_test
- Connection: postgres://test:test@localhost:/openrisk_test

 Cache Service

Redis (openrisk_redis)
- Type: Redis 
- Host: localhost
- Port: 
- Database:  (default)
- No password (development mode)

 Application Services

Backend (openrisk_backend)
- Type: Go Fiber API
- Port: 
- Endpoints: /api/v/
- Auto-runs migrations on startup
- Health check: /api/v/health

Frontend (openrisk_frontend)
- Type: React + TypeScript (Vite)
- Port: 
- Assets: http://localhost:
- API calls to: http://localhost:/api/v

 Common Commands

 View Service Status

bash
 List running containers
docker-compose ps

 View logs
docker-compose logs -f

 View specific service logs
docker-compose logs -f backend
docker-compose logs -f frontend
docker-compose logs -f db


 Start/Stop Services

bash
 Start all services
docker-compose up -d

 Stop all services
docker-compose down

 Stop and remove volumes (WARNING: deletes data)
docker-compose down -v

 Restart a specific service
docker-compose restart backend
docker-compose restart frontend


 Database Management

bash
 Connect to production database
psql -U openrisk -h localhost -p  -d openrisk

 Connect to test database
psql -U test -h localhost -p  -d openrisk_test

 View running migrations
docker-compose exec db psql -U openrisk -d openrisk -c \
  "SELECT  FROM schema_migrations ORDER BY version;"


 Building & Rebuilding

bash
 Rebuild all services
docker-compose build

 Rebuild specific service
docker-compose build backend
docker-compose build frontend

 Rebuild and restart
docker-compose up -d --build


 Development Workflows

 Option A: Local Development with Docker Services

Run the application in Docker while using native tools for development:

bash
 Start only infrastructure services (DB, Redis)
docker-compose up -d db test_db redis

 Run backend locally
cd backend
go run ./cmd/server

 In another terminal, run frontend locally
cd frontend
npm install
npm run dev


Advantages:
- Hot-reload for code changes
- Better IDE debugging
- Faster feedback loop

 Option B: Fully Containerized

Run everything in Docker:

bash
 Start all services
docker-compose up -d

 View application
 Frontend: http://localhost:
 Backend: http://localhost:


Advantages:
- Exact production-like environment
- No dependency conflicts
- Easy to share setup with team

 Option C: Mixed Mode for Debugging

Run specific services in foreground for debugging:

bash
 Terminal : Database services only
docker-compose up db redis

 Terminal : Run backend with debugging
cd backend
DEBUG=true go run ./cmd/server

 Terminal : Run frontend
cd frontend
npm run dev


 Database Migrations

Migrations run automatically when the backend starts. To manually run migrations:

bash
 Run all pending migrations
docker-compose exec backend openrisk migrate up

 Rollback last migration
docker-compose exec backend openrisk migrate down

 Check migration status
docker-compose exec db psql -U openrisk -d openrisk -c \
  "SELECT  FROM schema_migrations;"


 Testing

 Unit Tests

bash
 Run all backend tests
docker-compose exec backend go test -v ./...

 Run specific package tests
docker-compose exec backend go test -v ./internal/handlers

 Run with coverage
docker-compose exec backend go test -coverprofile=coverage.out ./...

 Run frontend tests
docker-compose exec frontend npm test


 Integration Tests

bash
 Run integration tests (requires test_db running)
docker-compose exec backend go test -v -tags=integration ./...

 Or use the helper script
./scripts/run-integration-tests.sh


 Troubleshooting

 Services Won't Start

bash
 Check logs for errors
docker-compose logs

 Verify Docker is running
docker ps

 Check port availability (, , , ,  must be free)
lsof -i :
lsof -i :
lsof -i :
lsof -i :
lsof -i :


 Database Connection Errors

bash
 Test database connectivity
docker-compose exec backend pg_isready -h db -p 

 Check database logs
docker-compose logs db

 Recreate database
docker-compose down -v
docker-compose up -d


 Frontend Not Connecting to Backend

bash
 Check backend is running
curl http://localhost:/api/v/health

 Check CORS headers (should allow localhost:)
curl -H "Origin: http://localhost:" \
     -H "Access-Control-Request-Method: GET" \
     -v http://localhost:/api/v/health

 Verify VITE_API_URL in .env is correct
cat .env | grep VITE_API_URL


 Container Health Checks Failing

bash
 View detailed container status
docker-compose ps

 Check specific container logs
docker-compose logs backend
docker-compose logs frontend

 Restart unhealthy container
docker-compose restart backend


 Environment Variables

See .env.example for all available configuration options:

env
 Database
DB_HOST=db
DB_PORT=
DB_USER=openrisk
DB_PASSWORD=openrisk
DB_NAME=openrisk

 Server
PORT=
JWT_SECRET=your-secret-key-change-in-production

 CORS
CORS_ORIGINS=http://localhost:,http://localhost:

 Redis
REDIS_URL=redis://redis:/

 Frontend
VITE_API_URL=http://localhost:/api


 Performance Tips

 Reduce Memory Usage

bash
 Stop test database if not needed
docker-compose stop test_db

 Remove unused volumes
docker volume prune


 Speed Up Initial Setup

bash
 Build images with BuildKit (faster)
DOCKER_BUILDKIT= docker-compose build

 Use layer caching
docker-compose build --progress=plain


 Monitor Resource Usage

bash
 View container resource usage
docker stats

 View detailed container info
docker-compose exec backend ps aux


 Making Changes to Code

 Backend Changes

. Edit files in backend/ directory
. If using native development, restart the Go server:
   bash
    Hot-reload happens automatically with go run
   
. If using Docker, restart the service:
   bash
   docker-compose restart backend
    Or rebuild if dependencies changed
   docker-compose up -d --build backend
   

 Frontend Changes

. Edit files in frontend/ directory
. If using native development (npm run dev), changes hot-reload automatically
. If using Docker with Vite dev server:
   bash
    Changes should hot-reload automatically
   docker-compose logs -f frontend
   

 Useful Make Commands

bash
 View all available commands
make help

 Build backend binary
make build

 Run tests
make test
make test-unit
make test-integration

 Lint code
make lint
make lint-backend
make lint-frontend

 Docker commands
make docker-up
make docker-down
make docker-logs

 Database
make migrate
make seed

 Development
make dev   Starts all services


 Next Steps

. Configure Integration Services (TheHive, OpenCTI, etc.)
   - Update values in .env
   - See docs/ for specific integration guides

. Explore API Documentation
   - OpenAPI Spec: docs/openapi.yaml
   - API Reference: docs/API_REFERENCE.md

. Run Integration Tests
   - ./scripts/run-integration-tests.sh
   - See docs/CI_CD.md for details

. Deploy to Staging
   - See docs/ for deployment guides

 Support

- Issues: GitHub Issues
- Documentation: See docs/ directory
- Examples: See dev/fixtures/ for sample data
- API Docs: docs/openapi.yaml and docs/API_REFERENCE.md

---

Happy Developing! 
