.PHONY: help build test lint clean docker-build docker-up docker-down migrate seed dev install setup

help:
	@echo "╔════════════════════════════════════════════════════════════════╗"
	@echo "║         OpenRisk Development Commands                          ║"
	@echo "╚════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "  🚀 QUICK START"
	@echo "     make setup                - Initial setup (install dependencies)"
	@echo "     make dev                  - Start full development environment"
	@echo "     make docker-up            - Start all Docker services"
	@echo ""
	@echo "  🔨 BUILD & COMPILE"
	@echo "     make build                - Build backend binary"
	@echo "     make frontend-build       - Build frontend for production"
	@echo ""
	@echo "  🧪 TESTING"
	@echo "     make test                 - Run all backend tests with coverage"
	@echo "     make test-unit            - Run unit tests only (fast)"
	@echo "     make test-integration     - Run integration tests (slow)"
	@echo "     make frontend-test        - Run frontend tests"
	@echo ""
	@echo "  📝 CODE QUALITY"
	@echo "     make lint                 - Run all linters"
	@echo "     make lint-backend         - Run backend linter (golangci-lint)"
	@echo "     make lint-frontend        - Run frontend linter (ESLint)"
	@echo "     make format               - Format code (gofmt + prettier)"
	@echo ""
	@echo "  🐳 DOCKER"
	@echo "     make docker-up            - Start all containers"
	@echo "     make docker-down          - Stop all containers"
	@echo "     make docker-build         - Build Docker image"
	@echo "     make docker-logs          - View live container logs"
	@echo "     make docker-status        - Show container status"
	@echo "     make docker-clean         - Remove containers and volumes"
	@echo ""
	@echo "  💾 DATABASE"
	@echo "     make migrate              - Run database migrations"
	@echo "     make migrate-rollback     - Rollback last migration"
	@echo "     make seed                 - Seed database with sample data"
	@echo "     make db-shell             - Open database shell (psql)"
	@echo "     make db-test-shell        - Open test database shell"
	@echo ""
	@echo "  🧹 CLEANUP"
	@echo "     make clean                - Clean build artifacts"
	@echo "     make clean-all            - Clean everything (DESTRUCTIVE)"
	@echo ""

# ============================================================================
# SETUP & INSTALLATION
# ============================================================================

setup: install frontend-install
	@echo "✅ Setup complete! Run 'make dev' to start developing."

install:
	@echo "📦 Installing backend dependencies..."
	cd backend && go mod download
	@echo "✅ Backend dependencies installed"

frontend-install:
	@echo "📦 Installing frontend dependencies..."
	cd frontend && npm ci
	@echo "✅ Frontend dependencies installed"

# ============================================================================
# BUILD & COMPILE
# ============================================================================

build:
	@echo "🔨 Building backend binary..."
	cd backend && CGO_ENABLED=0 go build -o openrisk ./cmd/server
	@echo "✅ Backend binary built: backend/openrisk"

frontend-build:
	@echo "🔨 Building frontend for production..."
	cd frontend && npm run build
	@echo "✅ Frontend built: frontend/dist/"

# ============================================================================
# TESTING
# ============================================================================

test:
	@echo "🧪 Running all backend tests with coverage..."
	cd backend && go test -v -coverprofile=coverage.out ./...
	@echo ""
	@echo "📊 Coverage report:"
	cd backend && go tool cover -func=coverage.out | tail -1

test-unit:
	@echo "🧪 Running unit tests (fast)..."
	cd backend && go test -v -short ./...

test-integration:
	@echo "🧪 Running integration tests..."
	docker-compose up -d test_db
	@sleep 2
	cd backend && DATABASE_URL="postgres://test:test@localhost:5435/openrisk_test" go test -v -tags=integration ./...
	docker-compose stop test_db

frontend-test:
	@echo "🧪 Running frontend tests..."
	cd frontend && npm run test

# ============================================================================
# CODE QUALITY & FORMATTING
# ============================================================================

lint-backend:
	@echo "📝 Running backend linter..."
	cd backend && golangci-lint run ./...

lint-frontend:
	@echo "📝 Running frontend linter..."
	cd frontend && npm run lint

lint: lint-backend lint-frontend
	@echo "✅ All linting passed!"

format:
	@echo "🎨 Formatting code..."
	cd backend && gofmt -s -w .
	cd frontend && npm run format
	@echo "✅ Code formatted!"

# ============================================================================
# DOCKER MANAGEMENT
# ============================================================================

docker-up:
	@echo "🐳 Starting all containers..."
	docker-compose up -d
	@echo "✅ Containers started"
	@echo ""
	@echo "   Frontend:  http://localhost:5173"
	@echo "   Backend:   http://localhost:8080"
	@echo "   Postgres:  localhost:5434 (openrisk)"
	@echo "   Redis:     localhost:6379"

docker-down:
	@echo "🐳 Stopping all containers..."
	docker-compose down

docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t openrisk:latest .
	@echo "✅ Docker image built"

docker-logs:
	@echo "📋 Showing live container logs..."
	docker-compose logs -f

docker-status:
	@echo "📊 Container status:"
	docker-compose ps

docker-clean:
	@echo "🗑️  Removing containers and volumes (DESTRUCTIVE)..."
	docker-compose down -v
	@echo "✅ Cleanup complete"

# ============================================================================
# DATABASE MANAGEMENT
# ============================================================================

migrate:
	@echo "📊 Running database migrations..."
	docker-compose up -d db
	@sleep 2
	cd backend && go run ./cmd/server migrate up
	@echo "✅ Migrations complete"

migrate-rollback:
	@echo "⏮️  Rolling back last migration..."
	docker-compose up -d db
	@sleep 2
	cd backend && go run ./cmd/server migrate down
	@echo "✅ Rollback complete"

seed:
	@echo "🌱 Seeding database with sample data..."
	cd backend && go run ./cmd/server seed
	@echo "✅ Database seeded"

db-shell:
	@echo "🔌 Connecting to production database..."
	docker-compose exec db psql -U openrisk -d openrisk

db-test-shell:
	@echo "🔌 Connecting to test database..."
	docker-compose exec test_db psql -U test -d openrisk_test

# ============================================================================
# DEVELOPMENT
# ============================================================================

dev:
	@echo "🚀 Starting development environment..."
	@echo ""
	@echo "   Starting Docker services (db, redis)..."
	docker-compose up -d db redis test_db
	@sleep 2
	@echo ""
	@echo "   Frontend will be available at: http://localhost:5173"
	@echo "   Backend API at:                http://localhost:8080/api/v1"
	@echo ""
	@echo "   Press Ctrl+C to stop. Use 'make docker-down' to stop Docker services."
	@echo ""
	@sleep 1
	@echo "Starting backend (in background)..."
	cd backend && go run ./cmd/server &
	@sleep 2
	@echo ""
	@echo "Starting frontend..."
	cd frontend && npm run dev

dev-docker:
	@echo "🐳 Starting full development environment in Docker..."
	docker-compose up

dev-logs:
	@echo "📋 Following all container logs..."
	docker-compose logs -f

# ============================================================================
# CLEANUP
# ============================================================================

clean:
	@echo "🧹 Cleaning build artifacts..."
	cd backend && go clean
	rm -f backend/openrisk
	rm -f backend/coverage.out
	rm -rf frontend/dist
	@echo "✅ Cleanup complete"

clean-all: clean docker-clean
	@echo "🗑️  Full cleanup complete (DESTRUCTIVE)"

.DEFAULT_GOAL := help
