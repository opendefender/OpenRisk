# Load Testing Guide - Phase 5 Performance Optimization

## Overview

This guide provides comprehensive instructions for running k6 load tests to validate the performance improvements from Phase 5 Priority #4 caching optimization.

## Prerequisites

### Software Requirements
- **k6**: Performance testing tool (https://k6.io)
- **Node.js**: For load test script execution
- **OpenRisk Backend**: Running and accessible (default: http://localhost:3000)
- **Test Account**: Valid credentials for authentication

### Installation

#### Install k6 (macOS)
```bash
brew install k6
```

#### Install k6 (Linux - Ubuntu/Debian)
```bash
sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update
sudo apt-get install k6
```

#### Install k6 (Windows)
```powershell
choco install k6
```

#### Verify Installation
```bash
k6 version
```

## Quick Start

### 1. Basic Load Test
```bash
cd load_tests
k6 run cache_test.js
```

### 2. Custom Configuration
```bash
# Set base URL
k6 run --env BASE_URL=http://staging.openrisk.local:3000/api/v1 cache_test.js

# Increase virtual users
k6 run --vus 20 --duration 5m cache_test.js

# Custom iterations
k6 run --iterations 100 cache_test.js
```

## Test Scenarios

### Scenario 1: Baseline (No Cache)
**Purpose**: Establish performance baseline without caching

**Configuration**:
- Duration: 2 minutes
- Virtual Users: 5
- Focus: First request (cache misses)

**Command**:
```bash
k6 run \
  --stage '2m:5' \
  cache_test.js
```

**Expected Results**:
- Response Time: ~150ms average
- Cache Hit Rate: 0%
- Throughput: ~500 req/s

### Scenario 2: Warm Cache
**Purpose**: Validate performance with warmed cache

**Configuration**:
- Duration: 4 minutes
- Virtual Users: 10
- Focus: Repeated requests (cache hits)

**Command**:
```bash
k6 run \
  --stage '1m:5' \
  --stage '2m:10' \
  --stage '1m:0' \
  cache_test.js
```

**Expected Results**:
- Response Time: ~15ms average
- Cache Hit Rate: 75%+
- Throughput: ~2000 req/s

### Scenario 3: Peak Load
**Purpose**: Test system under heavy concurrent load

**Configuration**:
- Duration: 5 minutes
- Virtual Users: 50 (peak)
- Focus: Mixed endpoints

**Command**:
```bash
k6 run \
  --stage '1m:10' \
  --stage '2m:50' \
  --stage '1m:25' \
  --stage '1m:0' \
  cache_test.js
```

**Expected Results**:
- Response Time P95: < 100ms
- Throughput: > 1000 req/s
- Error Rate: < 1%

## Running Tests

### Simple One-Liner Tests
```bash
# 5 users for 2 minutes
k6 run --vus 5 --duration 2m cache_test.js

# 20 users for 5 minutes
k6 run --vus 20 --duration 5m cache_test.js

# Custom iterations (run test N times)
k6 run --iterations 1000 cache_test.js
```

### Advanced Configuration
```bash
# Ramp up to 50 users over 5 minutes, stay for 5 minutes, ramp down over 2 minutes
k6 run \
  --stage '5m:50' \
  --stage '5m:50' \
  --stage '2m:0' \
  cache_test.js
```

### Environment Variable Configuration
```bash
# Override test parameters
k6 run \
  --env BASE_URL=http://staging:3000/api/v1 \
  --env TIMEOUT=30 \
  cache_test.js
```

## Metrics Interpretation

### Key Metrics

| Metric | Good | Warning | Critical |
|--------|------|---------|----------|
| Request Duration (avg) | < 50ms | 50-200ms | > 200ms |
| Request Duration (P95) | < 100ms | 100-300ms | > 300ms |
| Request Duration (P99) | < 200ms | 200-500ms | > 500ms |
| Error Rate | < 0.1% | 0.1-1% | > 1% |
| Cache Hit Rate | > 75% | 50-75% | < 50% |
| Throughput (req/s) | > 1000 | 500-1000 | < 500 |

### Cache Hit Rate Analysis

**Low Hit Rate (< 50%)**
- Indicates cache configuration issues
- Check: TTL values too short?
- Check: Query parameters changing too frequently?
- Action: Review cache_integration.go TTL settings

**Medium Hit Rate (50-75%)**
- Cache working but not optimal
- Likely causes:
  - Pagination parameters changing
  - User-specific data not cached
  - Cache invalidation too aggressive
- Action: Analyze which endpoints have low hit rate

**High Hit Rate (> 75%)**
- Cache performing well
- Focus on response time improvements
- Monitor memory usage in Redis

### Response Time Analysis

**Without Cache**:
- Typical: 150-300ms
- Database query: 80-100ms
- JSON serialization: 20-30ms
- Network overhead: 30-50ms

**With Cache**:
- Typical: 10-30ms
- Cache lookup: < 5ms
- JSON serialization: 5-10ms
- Network overhead: 5-15ms

### Throughput Analysis

**Calculate Expected Throughput**:
```
Throughput = (Concurrent Users * 60) / Response Time (seconds)

Example:
- 10 users * 60 / 0.015 seconds = 40,000 req/s (cached)
- 10 users * 60 / 0.150 seconds = 4,000 req/s (uncached)
```

## Results Analysis

### Reading k6 Output

```
checks.....................: 100% ✓ 1000 ✗ 0
data_received..............: 45 MB ✓
data_sent..................: 2 MB ✓
http_req_blocked...........: avg=100µs min=50µs med=90µs max=500µs p(90)=150µs p(95)=180µs
http_req_connecting........: avg=50µs  min=0µs  med=0µs  max=200µs p(90)=0µs   p(95)=0µs
http_req_duration..........: avg=25ms  min=5ms  med=15ms max=150ms p(90)=40ms  p(95)=55ms
http_req_receiving.........: avg=8ms   min=1ms  med=8ms  max=50ms  p(90)=15ms  p(95)=20ms
http_req_sending...........: avg=5ms   min=1ms  med=5ms  max=25ms  p(90)=10ms  p(95)=12ms
http_req_tls_handshaking...: avg=0µs   min=0µs  med=0µs  max=0µs   p(90)=0µs   p(95)=0µs
http_req_waiting...........: avg=12ms  min=2ms  med=10ms max=100ms p(90)=20ms  p(95)=25ms
http_requests..............: 10000 ✓
iteration_duration.........: avg=2s    min=1.5s med=2s   max=5s    p(90)=2.2s  p(95)=2.5s
iterations.................: 10000 ✓
vus..........................: 10 ✓
vus_max......................: 10 ✓
```

**Key Takeaways**:
- `http_req_duration`: Total request time
- `p(95)`: 95th percentile (90 out of 100 requests faster than this)
- `checks`: Test assertions pass/fail
- `http_requests`: Total requests completed

## Comparison: Before vs After Caching

### Test Setup
Run the same test without cache, then with cache enabled:

```bash
# Baseline (no cache)
echo "Running baseline test..."
k6 run --vus 10 --duration 2m cache_test.js > results_baseline.txt

# After cache integration
echo "Running cache test..."
k6 run --vus 10 --duration 2m cache_test.js > results_cached.txt
```

### Expected Comparison

| Metric | Baseline | Cached | Improvement |
|--------|----------|--------|-------------|
| Avg Response Time | 150ms | 15ms | 90% ↓ |
| P95 Response Time | 250ms | 45ms | 82% ↓ |
| P99 Response Time | 500ms | 100ms | 80% ↓ |
| Throughput | 400 req/s | 1600 req/s | 4x ↑ |
| Cache Hit Rate | 0% | 75%+ | New |

## Exporting Results

### JSON Export
```bash
k6 run \
  --out json=results.json \
  cache_test.js
```

### CSV Export (with extension)
```bash
k6 run \
  --out csv=results.csv \
  cache_test.js
```

### Analyze JSON Results
```bash
# Count successful requests
jq '.metrics[] | select(.type == "check") | select(.data.name == "status is 200") | .data.passes' results.json | wc -l

# Average request duration
jq '[.metrics[] | select(.type == "trend") | select(.name == "http_req_duration") | .data.values[]] | add / length' results.json
```

## Integration with Monitoring

### View Live Metrics in Grafana

1. **Start monitoring stack**:
```bash
cd deployment
docker-compose -f docker-compose-monitoring.yaml up -d
```

2. **Access Grafana**:
```
http://localhost:3001 (admin/admin)
```

3. **Run test and watch dashboard**:
```bash
cd load_tests
k6 run cache_test.js &  # Run in background
# Watch metrics in Grafana
```

### Expected Dashboard Updates

During test execution:
- **Redis Operations Rate**: Increases as requests hit cache
- **Cache Hit Ratio**: Should increase to 75%+
- **Query Performance**: Should decrease (faster with cache)
- **DB Connections**: Should decrease (less pool usage)

## Troubleshooting

### Connection Refused
**Error**: `Connection refused`

**Solution**:
1. Verify backend is running: `curl http://localhost:3000/api/v1/health`
2. Check firewall: `sudo ufw allow 3000`
3. Verify BASE_URL: `k6 run --env BASE_URL=http://localhost:3000/api/v1 cache_test.js`

### Authentication Failed
**Error**: `401 Unauthorized`

**Solution**:
1. Verify test account exists in database
2. Check credentials in cache_test.js setup() function
3. Run manual login test:
```bash
curl -X POST http://localhost:3000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@openrisk.local","password":"admin123"}'
```

### Slow Response Times (No Improvement)
**Symptom**: Response times not improving after cache integration

**Diagnosis**:
1. Check cache hit rate is > 75%
2. Verify Redis is running: `redis-cli -a redis123 PING`
3. Check if caching is actually integrated into routes
4. Verify cache invalidation isn't too aggressive

**Solution**:
1. Review main.go integration
2. Check cache_integration.go is being used
3. Verify wrapper methods are applied to endpoints
4. Monitor Redis keys: `redis-cli -a redis123 KEYS '*' | wc -l`

### Out of Memory
**Error**: `OOM: Out of memory`

**Solution**:
1. Reduce VUS (virtual users): `k6 run --vus 5 cache_test.js`
2. Reduce duration: `k6 run --duration 1m cache_test.js`
3. Increase system memory or swap

## Best Practices

### Before Load Testing
- [ ] Ensure backend is stable (no recent deployments)
- [ ] Clear cache data: `redis-cli -a redis123 FLUSHALL`
- [ ] Verify database has test data
- [ ] Check system resources (CPU, memory)
- [ ] Disable background jobs/maintenance

### During Load Testing
- [ ] Monitor system metrics (CPU, memory, connections)
- [ ] Watch for error spikes
- [ ] Note response time trends
- [ ] Observe cache hit rate increases

### After Load Testing
- [ ] Export results for comparison
- [ ] Document performance metrics
- [ ] Note any issues or anomalies
- [ ] Review Grafana dashboard for patterns
- [ ] Plan optimization if targets not met

## Advanced Usage

### Custom Threshold Checks
```javascript
import { check } from 'k6';

check(res, {
  'response time < 100ms': (r) => r.timings.duration < 100,
  'status is 200 or 304': (r) => r.status === 200 || r.status === 304,
  'cache hit': (r) => r.headers['X-Cache'] === 'HIT',
});
```

### Rate Limiting
```bash
# Limit to 1000 requests per second
k6 run --rps 1000 cache_test.js
```

### Custom Payload
```javascript
const payload = JSON.stringify({
  title: `Risk ${__VU}-${__ITER}`,
  description: 'Test risk created by k6',
  impact: 3,
  probability: 4,
});

const res = http.post(`${BASE_URL}/risks`, payload, { headers });
```

## Performance Baselines

### Recommended Targets

| Scenario | Metric | Target |
|----------|--------|--------|
| Cache Hit Rate | Percentage | > 75% |
| Response Time P95 | milliseconds | < 100ms |
| Response Time P99 | milliseconds | < 200ms |
| Error Rate | percentage | < 0.1% |
| Throughput | req/s | > 1000 |
| DB Connections | count | < 30 |
| Redis Memory | MB | < 500MB |

### Acceptance Criteria

**Phase 5 Optimization is SUCCESSFUL if**:
- ✅ Cache hit rate ≥ 75%
- ✅ Response time P95 < 100ms
- ✅ Throughput > 1000 req/s
- ✅ Error rate < 0.1%
- ✅ DB connections < 30
- ✅ All 4 alerts firing correctly

## Further Reading

- [k6 Documentation](https://k6.io/docs/)
- [k6 HTTP Requests](https://k6.io/docs/javascript-api/k6-http/)
- [k6 Metrics](https://k6.io/docs/using-k6/metrics/)
- [Performance Testing Best Practices](https://k6.io/docs/testing-guides/best-practices/)
- [OpenRisk API Reference](../docs/API_REFERENCE.md)

## Support

For issues or questions:
1. Check troubleshooting section above
2. Review k6 documentation
3. Check OpenRisk backend logs
4. Verify Grafana dashboard metrics
5. Contact DevOps team
