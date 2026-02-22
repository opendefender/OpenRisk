// Performance Testing Guide - k6 Load Testing Setup
// Location: docs/PERFORMANCE_TESTING.md

# Performance Testing with k6

## Overview

This guide covers setting up and running load tests using k6 to measure OpenRisk's performance under load.

## Installation

```bash
# Install k6 (macOS)
brew install k6

# Install k6 (Linux)
sudo apt-get install k6

# Install k6 (Windows)
choco install k6
```

## Quick Start

### 1. Start OpenRisk Backend

```bash
cd backend
go run ./cmd/server
```

### 2. Run Baseline Load Test

```bash
cd load_tests
k6 run performance_baseline.js --vus 10 --duration 30s
```

### 3. Run with Docker

```bash
docker run -it --rm \
  --network openrisk_network \
  -v /path/to/load_tests:/scripts \
  loadimpact/k6:latest \
  run /scripts/performance_baseline.js
```

## Load Testing Scripts

### performance_baseline.js

Tests basic performance under load:

```bash
k6 run load_tests/performance_baseline.js
```

**Stages:**
- 0-30s: Ramp up to 10 concurrent users
- 30s-2m: Ramp up to 50 concurrent users
- 2m-4m: Maintain 50 concurrent users
- 4m-4m30s: Ramp down to 0 users

**Thresholds:**
- 95th percentile response time < 500ms
- 99th percentile response time < 1000ms
- Error rate < 10%

**Metrics Tracked:**
- HTTP request duration
- Success/error rate
- Active connections
- Throughput

## Custom Load Tests

Create custom test scenarios:

```javascript
// custom_test.js
import http from 'k6/http';
import { check } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 100 },
    { duration: '3m', target: 100 },
    { duration: '1m', target: 0 },
  ],
};

export default function () {
  let response = http.get('http://localhost:8080/api/v1/risks');
  check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });
}
```

Run it:
```bash
k6 run custom_test.js
```

## Performance Optimization Checklist

- [ ] Database indexes created (migration 0009)
- [ ] N+1 query patterns fixed using QueryOptimizer
- [ ] Redis caching configured and integrated
- [ ] Connection pooling configured
- [ ] Database query timeouts set
- [ ] Proper pagination implemented
- [ ] Response caching headers set
- [ ] Load test baseline established

## Interpreting Results

### Successful Run
```
✓ http_req_duration..........: avg=150ms p(95)=350ms p(99)=800ms
✓ http_req_failed...........: 0.00%
✓ errors....................: 0.00%
✓ successful_requests.......: 4500
```

### Performance Issues to Watch

1. **High Response Times**
   - Check database query performance
   - Review N+1 query patterns
   - Verify indexes are used

2. **High Error Rate**
   - Check connection pool limits
   - Review timeout settings
   - Check database connection health

3. **Memory Leaks**
   - Monitor process memory during test
   - Check for goroutine leaks
   - Review connection handling

## Continuous Performance Testing

### GitHub Actions Integration

```yaml
name: Performance Tests
on: [push, pull_request]
jobs:
  load-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: grafana/k6-action@v0.3.0
        with:
          filename: load_tests/performance_baseline.js
          cloud: true
```

## Performance Targets

| Metric | Target | Current |
|--------|--------|---------|
| Response Time (p95) | < 500ms | - |
| Response Time (p99) | < 1000ms | - |
| Error Rate | < 0.1% | - |
| Throughput | > 100 req/s | - |
| Concurrent Users | 100+ | - |

## Troubleshooting

### Redis Connection Issues

```bash
# Check Redis is running
redis-cli ping

# View Redis info
redis-cli info stats
```

### Database Connection Errors

```bash
# Check database connection pooling
# Review backend logs for connection pool exhaustion
grep "max connections" logs/backend.log
```

### Out of Memory

```bash
# Monitor memory usage
k6 run --duration 5m --memory-limit 512mb load_tests/performance_baseline.js

# Or check system memory
free -h
```

## Next Steps

1. Establish performance baselines
2. Identify bottlenecks
3. Optimize identified areas
4. Retest and compare results
5. Set performance SLOs
6. Integrate into CI/CD pipeline
