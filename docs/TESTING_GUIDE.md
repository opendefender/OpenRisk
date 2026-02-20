# Testing Guide - OpenRisk Phase 5

Complete guide for running integration tests, E2E tests, performance benchmarking, and security testing.

## Table of Contents

1. [Integration Tests](#integration-tests)
2. [E2E Tests with Playwright](#e2e-tests-with-playwright)
3. [Performance Benchmarking](#performance-benchmarking)
4. [Security Testing](#security-testing)
5. [Docker Compose Testing](#docker-compose-testing)
6. [CI/CD Integration](#cicd-integration)

---

## Integration Tests

Integration tests verify that components work together correctly with real databases and services.

### Setup

```bash
# Install dependencies
go mod download

# Set environment variables
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5435
export TEST_DB_USER=test
export TEST_DB_PASSWORD=test
export TEST_DB_NAME=openrisk_test
```

### Running Integration Tests

```bash
# Run all integration tests
go test -v ./tests/integration_test.go ./tests/mocks.go -timeout 30m

# Run specific test
go test -v -run TestRiskCRUD ./tests/integration_test.go -timeout 10m

# With coverage
go test -v -cover ./tests/integration_test.go -timeout 30m
```

### Test Coverage

**Risk Management:**
- ✅ Create risk
- ✅ Retrieve risk
- ✅ Update risk
- ✅ Delete risk
- ✅ List with pagination

**Relationships:**
- ✅ Mitigation CRUD
- ✅ Asset associations
- ✅ Custom fields storage

**Advanced Operations:**
- ✅ Bulk operations
- ✅ Query performance (< 100ms indexed queries)
- ✅ Concurrent operations (10 parallel requests)
- ✅ Transaction rollback

**Data Integrity:**
- ✅ Audit log creation
- ✅ Cascading deletes
- ✅ Foreign key constraints

---

## E2E Tests with Playwright

End-to-end tests verify user workflows in real browsers.

### Setup

```bash
# Install Playwright
npm install --save-dev @playwright/test

# Install browsers
npx playwright install

# Create test environment file
cat > .env.test << EOF
E2E_BASE_URL=http://localhost:5173
API_URL=http://localhost:8080
TEST_EMAIL=test@example.com
TEST_PASSWORD=password123
EOF
```

### Running E2E Tests

```bash
# Run all E2E tests
npx playwright test

# Run specific test file
npx playwright test tests/e2e.spec.ts

# Run in headed mode (see browser)
npx playwright test --headed

# Run on specific browser
npx playwright test --project=chromium
npx playwright test --project=firefox
npx playwright test --project=webkit

# Run with debug
npx playwright test --debug

# Generate HTML report
npx playwright show-report
```

### Test Scenarios

**Authentication:**
- ✅ Display login form
- ✅ Validate email format
- ✅ Reject invalid credentials
- ✅ Successful login and redirect
- ✅ SSO provider buttons visible

**Risk Management:**
- ✅ Display risk list
- ✅ Create new risk with form validation
- ✅ View risk details
- ✅ Update risk properties
- ✅ Delete with confirmation
- ✅ Pagination controls work

**Custom Fields:**
- ✅ Navigate to custom fields
- ✅ Create custom field with type selection
- ✅ Edit existing field
- ✅ Delete field with confirmation
- ✅ Field options management

**Bulk Operations:**
- ✅ Access bulk operations dashboard
- ✅ View job list
- ✅ Track operation progress
- ✅ Filter by status
- ✅ View operation logs

**Performance:**
- ✅ Dashboard loads < 3s
- ✅ Risk list (100 items) < 5s
- ✅ Rapid navigation < 10s total
- ✅ No layout shift during loading

**Mobile Responsiveness:**
- ✅ Works on iPhone 12 (375px)
- ✅ Works on Pixel 5 (393px)
- ✅ Touch interactions functional
- ✅ Forms readable on mobile

**Error Handling:**
- ✅ Network error displays message
- ✅ Server error handled gracefully
- ✅ Timeout shows appropriate message
- ✅ Form validation errors shown

### Debugging E2E Tests

```bash
# Generate detailed trace
npx playwright test --trace on

# View trace
npx playwright show-trace trace.zip

# Enable verbose logging
DEBUG=pw:api npx playwright test

# Single test isolation
npx playwright test tests/e2e.spec.ts -g "should create new risk"
```

---

## Performance Benchmarking

Measure and track performance metrics across operations.

### Setup

```bash
# Ensure test database is running
docker-compose -f docker-compose.test.yaml up test_db test_redis -d

# Set environment
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5436
export TEST_DB_USER=test
export TEST_DB_PASSWORD=test
export TEST_DB_NAME=openrisk_test
```

### Running Benchmarks

```bash
# Run all benchmarks
go test -v -bench=. ./tests/performance_benchmark_test.go -timeout 30m

# Run specific benchmark
go test -v -bench=BenchmarkRiskCreation ./tests/performance_benchmark_test.go

# With CPU profile
go test -bench=. -cpuprofile=cpu.prof ./tests/performance_benchmark_test.go
go tool pprof cpu.prof

# With memory profile
go test -bench=. -memprofile=mem.prof ./tests/performance_benchmark_test.go
go tool pprof mem.prof

# Benchstat comparison (before/after optimization)
go test -bench=. ./tests/performance_benchmark_test.go > before.txt
# [Make changes]
go test -bench=. ./tests/performance_benchmark_test.go > after.txt
benchstat before.txt after.txt
```

### Key Metrics

**Target Performance (After Optimization):**

| Operation | Target | Status |
|-----------|--------|--------|
| Risk Creation | > 100 ops/sec | ✅ |
| Risk Retrieval | > 500 ops/sec | ✅ |
| Risk Update | > 100 ops/sec | ✅ |
| List with Preload | > 50 ops/sec | ✅ |
| Cache Get | > 1000 ops/sec | ✅ |
| Bulk Insert | > 10 ops/sec (100 items) | ✅ |
| Concurrent Reads | > 100 ops/sec | ✅ |
| Query Filtering | > 50 ops/sec | ✅ |

### Interpreting Results

```
BenchmarkRiskCreation-8       1000    1234567 ns/op
                              ↑       ↑
                              Runs    Time per operation (nanoseconds)

1,000 runs × 1.23ms = ~1.23s total
Ops/second = 1,000,000 / 1,234 ≈ 810 ops/sec
```

---

## Security Testing

Verify protection against common vulnerabilities.

### Running Security Tests

```bash
# Setup test backend
docker-compose -f docker-compose.test.yaml up test_backend -d

# Run security tests
go test -v ./tests/security_test.go -timeout 30m

# With verbose output
go test -v -run TestCSRFProtection ./tests/security_test.go
```

### Test Coverage

**Authentication & Authorization:**
- ✅ CSRF token validation
- ✅ Missing authentication returns 401
- ✅ Invalid tokens rejected
- ✅ Expired tokens rejected
- ✅ Token format validation

**Input Validation:**
- ✅ SQL injection prevention
- ✅ XSS payload escaping
- ✅ Required field validation
- ✅ Type validation (score 0-100)
- ✅ Email format validation

**Data Protection:**
- ✅ Sensitive fields not exposed (passwords, secrets)
- ✅ PII properly masked
- ✅ Rate limiting enforced
- ✅ Path traversal blocked

**HTTP Security:**
- ✅ X-Content-Type-Options: nosniff
- ✅ X-Frame-Options: DENY
- ✅ X-XSS-Protection header
- ✅ Strict-Transport-Security
- ✅ Content-Security-Policy
- ✅ Referrer-Policy

**CORS Configuration:**
- ✅ Valid origins allowed
- ✅ Invalid origins blocked
- ✅ Credentials validation
- ✅ Preflight requests handled

### Security Checklist

```
[ ] All endpoints require authentication
[ ] CSRF tokens validated on state-changing operations
[ ] Input sanitized and validated
[ ] SQL queries use parameterized statements
[ ] Sensitive data encrypted at rest
[ ] HTTPS enforced in production
[ ] Security headers present
[ ] Rate limiting configured
[ ] Audit logs enabled
[ ] No hardcoded secrets in code
[ ] Dependencies regularly updated
```

---

## Docker Compose Testing

Run entire test suite with isolated services.

### Quick Start

```bash
# Start all test services
docker-compose -f docker-compose.test.yaml up -d

# Wait for services to be healthy
docker-compose -f docker-compose.test.yaml ps
# All services should show (healthy)

# View service logs
docker-compose -f docker-compose.test.yaml logs -f test_backend
docker-compose -f docker-compose.test.yaml logs -f test_frontend
```

### Running Tests

```bash
# Run integration tests in container
docker-compose -f docker-compose.test.yaml run integration_tests

# Run security tests
docker-compose -f docker-compose.test.yaml run security_tests

# Run performance tests
docker-compose -f docker-compose.test.yaml run performance_tests

# Run E2E tests
docker-compose -f docker-compose.test.yaml run e2e_tests

# Run load tests
docker-compose -f docker-compose.test.yaml run load_tests
```

### Complete Test Sequence

```bash
# 1. Start infrastructure
docker-compose -f docker-compose.test.yaml up -d test_db test_redis

# 2. Wait for databases to be ready
sleep 10

# 3. Start services
docker-compose -f docker-compose.test.yaml up -d test_backend test_frontend

# 4. Run all tests
docker-compose -f docker-compose.test.yaml run integration_tests
docker-compose -f docker-compose.test.yaml run security_tests
docker-compose -f docker-compose.test.yaml run performance_tests
docker-compose -f docker-compose.test.yaml run e2e_tests
docker-compose -f docker-compose.test.yaml run load_tests

# 5. Cleanup
docker-compose -f docker-compose.test.yaml down -v
```

### Service URLs

- Frontend: http://localhost:5174
- Backend API: http://localhost:8081
- Test Database: localhost:5436
- Test Redis: localhost:6380

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Complete Test Suite

on: [push, pull_request]

jobs:
  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15-alpine
        env:
          POSTGRES_DB: openrisk_test
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
      redis:
        image: redis:7-alpine
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: go test -v ./tests/integration_test.go -timeout 30m

  security-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15-alpine
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: go test -v ./tests/security_test.go -timeout 30m

  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
      - run: npx playwright install
      - run: npm run build
      - run: npx playwright test

  performance-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: go test -v -bench=. ./tests/performance_benchmark_test.go -timeout 30m
```

---

## Troubleshooting

### Integration Tests Fail

```bash
# Check test database is running
docker-compose up test_db -d

# Verify connection
psql -h localhost -p 5435 -U test -d openrisk_test -c "SELECT 1"

# Check environment variables
echo $TEST_DB_HOST $TEST_DB_PORT
```

### E2E Tests Timeout

```bash
# Increase timeout
npx playwright test --timeout 60000

# Check frontend is running
curl http://localhost:5173

# Increase wait times in test
page.waitForLoadState('networkidle', { timeout: 10000 })
```

### Performance Tests Show Slow Results

```bash
# Check database performance
SELECT * FROM pg_stat_statements ORDER BY mean_exec_time DESC;

# Verify indexes are created
SELECT indexname FROM pg_indexes WHERE tablename = 'risks';

# Run ANALYZE
ANALYZE;
```

### Security Tests Fail

```bash
# Check backend logs
docker-compose logs test_backend

# Verify API is responding
curl -i http://localhost:8081/health

# Test specific endpoint
curl -X GET http://localhost:8081/api/v1/risks \
  -H "Authorization: Bearer test-token"
```

---

## Best Practices

1. **Isolation**: Each test suite runs in isolated environment
2. **Cleanup**: Tests clean up their data after execution
3. **Repeatability**: Tests produce same results on repeated runs
4. **Performance**: Tests complete within reasonable time
5. **Clarity**: Clear test names and assertion messages
6. **Documentation**: Keep this guide updated with new tests

---

## Next Steps

After tests pass:
1. Review code coverage report
2. Address any security warnings
3. Optimize slow operations
4. Document any edge cases found
5. Merge to staging branch for further testing

