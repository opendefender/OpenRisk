 Integration Test Execution Guide

 Overview

This guide covers running the comprehensive integration test suite for OpenRisk, which validates:

-  Database connectivity and migrations
-  Unit tests (fast path)
-  Integration tests (HTTP handlers)
-  Code coverage analysis
-  Docker container orchestration

 Prerequisites

- Docker & Docker Compose (v.+)
- Go .+
- PostgreSQL client tools (optional, for direct DB access)

 Quick Start

 Option : Automated Test Suite (Recommended)

bash
 Run full integration tests with all checks
./scripts/run-integration-tests.sh

 Run with verbose output
./scripts/run-integration-tests.sh --verbose

 Run and keep containers alive for debugging
./scripts/run-integration-tests.sh --keep-containers


 Option : Using Make Commands

bash
 Run all tests (unit + integration)
make test

 Run only unit tests (fast, ~- seconds)
make test-unit

 Run only integration tests (requires docker)
make test-integration


 Option : Manual Testing

bash
 Start test infrastructure
docker-compose up -d test_db

 Wait for database
sleep 

 Run tests directly
cd backend
export DATABASE_URL="postgres://test:test@localhost:/openrisk_test"
go test -v -tags=integration ./internal/handlers

 Cleanup
docker-compose down


 Test Structure

 Unit Tests

Located in _test.go files throughout the codebase.

bash
 Quick unit tests (no build tags)
go test -v -short ./...

 Shows:
- Domain model validations
- Service layer logic
- Permission system
- Token generation/verification


 Integration Tests

Located in _integration_test.go files, built with //go:build integration.

bash
 Full integration tests
go test -v -tags=integration ./internal/handlers

 Tests:
- Risk CRUD operations via HTTP
- Token lifecycle management
- Database persistence
- JSON serialization


 Test Files Reference

 Backend Tests


backend/
 internal/
    core/domain/
       permission_test.go ( tests)
       role_test.go (domain RBAC)
       audit_log_test.go
       token_test.go (+ tests)
    services/
       auth_service_test.go
       permission_service_test.go ( tests)
       token_service_test.go (+ tests)
       audit_service_test.go
    middleware/
       permission_test.go ( tests)
       auth_test.go ( tests)
       ratelimit_test.go
       tokenauth_test.go ( tests)
    handlers/
        risk_handler_integration_test.go ( tests)
        token_flow_integration_test.go ( tests)
        auth_handler_test.go
        token_handler_test.go ( tests)
        test_helpers.go (DB setup)
 adapters/
    thehive/
        client_test.go ( tests)
 workers/
     sync_engine_test.go ( tests)


 Frontend Tests


frontend/src/
 pages/
    Login.test.tsx ( tests)
    Register.test.tsx ( tests)
 components/
    CreateRiskModal.test.tsx
    EditRiskModal.test.tsx
    ... (component tests)
 hooks/
    useRiskStore.test.ts (hook tests)
 __tests__/ (integration tests)


 Running Specific Test Suites

 Test Permissions & RBAC

bash
cd backend

 Permission domain tests
go test -v ./internal/core/domain -run TestPermission

 Permission service tests
go test -v ./internal/services -run TestPermissionService

 Permission middleware tests
go test -v ./internal/middleware -run TestRequirePermissions


 Test Token Management

bash
cd backend

 Token domain tests
go test -v ./internal/core/domain -run TestToken

 Token service tests
go test -v ./internal/services -run TestTokenService

 Token handler tests
go test -v ./internal/handlers -run TestToken

 Token middleware tests
go test -v ./internal/middleware -run TestTokenAuth


 Test Risk Operations

bash
cd backend

 Risk handler integration tests (requires -tags=integration)
go test -v -tags=integration ./internal/handlers -run TestCreateRisk
go test -v -tags=integration ./internal/handlers -run TestUpdateRisk
go test -v -tags=integration ./internal/handlers -run TestDeleteRisk


 Test Authentication

bash
cd backend

 Auth handler tests
go test -v ./internal/handlers -run TestAuth

 Auth service tests
go test -v ./internal/services -run TestAuthService

 Auth middleware tests
go test -v ./internal/middleware -run TestAuth


 Frontend Tests

bash
cd frontend

 Run all frontend tests
npm test

 Run specific test file
npm test -- Login.test.tsx

 Run with coverage
npm test -- --coverage


 Coverage Analysis

 View Coverage Report

bash
cd backend

 Generate coverage for all packages
go test -coverprofile=coverage.out ./...

 View coverage summary
go tool cover -func=coverage.out

 Generate HTML report
go tool cover -html=coverage.out -o coverage.html

 Open in browser
open coverage.html   macOS
xdg-open coverage.html   Linux
start coverage.html   Windows


 Coverage Targets by Package

| Package | Target | Current |
|---------|--------|---------|
| services | %+ | ~% |
| handlers | %+ | ~% |
| middleware | %+ | ~% |
| domain | %+ | ~% |
| adapters | %+ | ~% |

 Troubleshooting Integration Tests

 "Database failed to start"

bash
 Check if port  is already in use
lsof -i :

 Kill the conflicting process
kill - <PID>

 Or use a different port (edit docker-compose.yaml)


 "Connection refused" errors

bash
 Ensure Docker daemon is running
docker ps

 Check test database is up
docker-compose ps test_db

 View database logs
docker-compose logs test_db


 "Permission denied" on script

bash
 Make script executable
chmod +x scripts/run-integration-tests.sh

 Run again
./scripts/run-integration-tests.sh


 Test timeout errors

bash
 Increase timeout for slow machines
cd backend
go test -v -tags=integration -timeout s ./internal/handlers

 Or set globally
export TIMEOUT=s
go test -v -tags=integration -timeout=$TIMEOUT ./...


 Database state issues

bash
 Force fresh database
docker-compose down -v test_db
docker-compose up -d test_db
sleep 

 Re-run tests
go test -v -tags=integration ./...


 CI/CD Integration

 GitHub Actions

See .github/workflows/ci.yml for automated test runs:

yaml
- name: Run Integration Tests
  run: ./scripts/run-integration-tests.sh --verbose


 Local Pre-commit Hook

bash
!/bin/bash
 .git/hooks/pre-commit

echo "Running tests before commit..."

 Quick validation
cd backend && go test -short ./... || exit 
cd ../frontend && npm test -- --coverage || exit 

echo " All tests passed!"


 Performance Optimization

 Parallel Test Execution

bash
 Run tests in parallel ( workers)
go test -v -parallel  ./...

 Or let Go auto-detect
go test -v -parallel  ./...


 Skip Slow Tests

bash
 Run only fast tests
go test -v -short ./...

 Run specific packages
go test -v ./internal/services


 Caching

bash
 Clear Go test cache
go clean -testcache

 Rebuild all tests
go test -v -count= ./...


 Best Practices

. Run tests frequently - Before committing, after changes
. Keep tests isolated - Each test should be independent
. Use table-driven tests - For better coverage and maintainability
. Mock external services - Use test fixtures instead of real APIs
. Clean up resources - Ensure CleanupTestDB() is called
. Check coverage - Aim for %+ on critical paths
. Commit coverage reports - Track coverage trends over time

 Next Steps

After tests pass:

. Deploy to staging: See docs/DEPLOYMENT.md
. Load testing: See docs/PERFORMANCE.md
. Security scanning: See docs/SECURITY.md

---

Need Help?

- Check GitHub Issues: https://github.com/opendefender/openrisk/issues
- Review test output: integration_test.log
- Check Docker logs: docker-compose logs
