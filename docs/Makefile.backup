.PHONY: help build test lint clean docker-build docker-up docker-down migrate seed

help:
	@echo "OpenRisk Development Commands"
	@echo "=============================="
	@echo "make build             - Build backend binary"
	@echo "make test              - Run all tests"
	@echo "make test-unit         - Run unit tests only"
	@echo "make test-integration  - Run integration tests (requires docker)"
	@echo "make lint              - Run linters (golangci-lint, eslint)"
	@echo "make lint-backend      - Run backend linter"
	@echo "make lint-frontend     - Run frontend linter"
	@echo "make clean             - Clean build artifacts"
	@echo "make docker-build      - Build Docker image"
	@echo "make docker-up         - Start containers (docker-compose)"
	@echo "make docker-down       - Stop containers"
	@echo "make migrate           - Run database migrations"
	@echo "make seed              - Seed database with sample data"
	@echo "make dev               - Start development server (requires go and node)"

# Backend
build:
	cd backend && CGO_ENABLED=0 go build -o openrisk ./cmd/server

test:
	cd backend && go test -v -coverprofile=coverage.out ./...

test-unit:
	cd backend && go test -v -short ./...

test-integration:
	docker-compose up -d test_db
	sleep 2
	cd backend && DATABASE_URL="postgres://test:test@localhost:5435/openrisk_test" go test -v -tags=integration ./...
	docker-compose down

lint-backend:
	cd backend && golangci-lint run ./...

lint-frontend:
	cd frontend && npm run lint

lint: lint-backend lint-frontend

clean:
	cd backend && go clean
	rm -f backend/openrisk
	rm -f backend/coverage.out

# Frontend
frontend-install:
	cd frontend && npm ci

frontend-dev:
	cd frontend && npm run dev

frontend-build:
	cd frontend && npm run build

frontend-test:
	cd frontend && npm run test

# Docker
docker-build:
	docker build -t openrisk:latest .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Database
migrate:
	@echo "Running migrations..."
	docker-compose up -d db
	@sleep 2
	cd backend && go run ./cmd/server

seed:
	cd backend && go run ./cmd/migrate seed

# Development
dev:
	@echo "Starting development environment..."
	docker-compose up -d db redis
	@sleep 2
	@echo "Starting backend..."
	cd backend && go run ./cmd/server &
	@echo "Starting frontend..."
	cd frontend && npm run dev

.DEFAULT_GOAL := help
