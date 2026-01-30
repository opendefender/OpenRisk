 Load Testing Guide - Phase  Performance Optimization

 Overview

This guide provides comprehensive instructions for running k load tests to validate the performance improvements from Phase  Priority  caching optimization.

 Prerequisites

 Software Requirements
- k: Performance testing tool (https://k.io)
- Node.js: For load test script execution
- OpenRisk Backend: Running and accessible (default: http://localhost:)
- Test Account: Valid credentials for authentication

 Installation

 Install k (macOS)
bash
brew install k


 Install k (Linux - Ubuntu/Debian)
bash
sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com: --recv-keys CADCEADDCCDACD
echo "deb https://dl.k.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k.list
sudo apt-get update
sudo apt-get install k


 Install k (Windows)
powershell
choco install k


 Verify Installation
bash
k version


 Quick Start

 . Basic Load Test
bash
cd load_tests
k run cache_test.js


 . Custom Configuration
bash
 Set base URL
k run --env BASE_URL=http://staging.openrisk.local:/api/v cache_test.js

 Increase virtual users
k run --vus  --duration m cache_test.js

 Custom iterations
k run --iterations  cache_test.js


 Test Scenarios

 Scenario : Baseline (No Cache)
Purpose: Establish performance baseline without caching

Configuration:
- Duration:  minutes
- Virtual Users: 
- Focus: First request (cache misses)

Command:
bash
k run \
  --stage 'm:' \
  cache_test.js


Expected Results:
- Response Time: ~ms average
- Cache Hit Rate: %
- Throughput: ~ req/s

 Scenario : Warm Cache
Purpose: Validate performance with warmed cache

Configuration:
- Duration:  minutes
- Virtual Users: 
- Focus: Repeated requests (cache hits)

Command:
bash
k run \
  --stage 'm:' \
  --stage 'm:' \
  --stage 'm:' \
  cache_test.js


Expected Results:
- Response Time: ~ms average
- Cache Hit Rate: %+
- Throughput: ~ req/s

 Scenario : Peak Load
Purpose: Test system under heavy concurrent load

Configuration:
- Duration:  minutes
- Virtual Users:  (peak)
- Focus: Mixed endpoints

Command:
bash
k run \
  --stage 'm:' \
  --stage 'm:' \
  --stage 'm:' \
  --stage 'm:' \
  cache_test.js


Expected Results:
- Response Time P: < ms
- Throughput: >  req/s
- Error Rate: < %

 Running Tests

 Simple One-Liner Tests
bash
  users for  minutes
k run --vus  --duration m cache_test.js

  users for  minutes
k run --vus  --duration m cache_test.js

 Custom iterations (run test N times)
k run --iterations  cache_test.js


 Advanced Configuration
bash
 Ramp up to  users over  minutes, stay for  minutes, ramp down over  minutes
k run \
  --stage 'm:' \
  --stage 'm:' \
  --stage 'm:' \
  cache_test.js


 Environment Variable Configuration
bash
 Override test parameters
k run \
  --env BASE_URL=http://staging:/api/v \
  --env TIMEOUT= \
  cache_test.js


 Metrics Interpretation

 Key Metrics

| Metric | Good | Warning | Critical |
|--------|------|---------|----------|
| Request Duration (avg) | < ms | -ms | > ms |
| Request Duration (P) | < ms | -ms | > ms |
| Request Duration (P) | < ms | -ms | > ms |
| Error Rate | < .% | .-% | > % |
| Cache Hit Rate | > % | -% | < % |
| Throughput (req/s) | >  | - | <  |

 Cache Hit Rate Analysis

Low Hit Rate (< %)
- Indicates cache configuration issues
- Check: TTL values too short?
- Check: Query parameters changing too frequently?
- Action: Review cache_integration.go TTL settings

Medium Hit Rate (-%)
- Cache working but not optimal
- Likely causes:
  - Pagination parameters changing
  - User-specific data not cached
  - Cache invalidation too aggressive
- Action: Analyze which endpoints have low hit rate

High Hit Rate (> %)
- Cache performing well
- Focus on response time improvements
- Monitor memory usage in Redis

 Response Time Analysis

Without Cache:
- Typical: -ms
- Database query: -ms
- JSON serialization: -ms
- Network overhead: -ms

With Cache:
- Typical: -ms
- Cache lookup: < ms
- JSON serialization: -ms
- Network overhead: -ms

 Throughput Analysis

Calculate Expected Throughput:

Throughput = (Concurrent Users  ) / Response Time (seconds)

Example:
-  users   / . seconds = , req/s (cached)
-  users   / . seconds = , req/s (uncached)


 Results Analysis

 Reading k Output


checks.....................: %    
data_received..............:  MB 
data_sent..................:  MB 
http_req_blocked...........: avg=µs min=µs med=µs max=µs p()=µs p()=µs
http_req_connecting........: avg=µs  min=µs  med=µs  max=µs p()=µs   p()=µs
http_req_duration..........: avg=ms  min=ms  med=ms max=ms p()=ms  p()=ms
http_req_receiving.........: avg=ms   min=ms  med=ms  max=ms  p()=ms  p()=ms
http_req_sending...........: avg=ms   min=ms  med=ms  max=ms  p()=ms  p()=ms
http_req_tls_handshaking...: avg=µs   min=µs  med=µs  max=µs   p()=µs   p()=µs
http_req_waiting...........: avg=ms  min=ms  med=ms max=ms p()=ms  p()=ms
http_requests..............:  
iteration_duration.........: avg=s    min=.s med=s   max=s    p()=.s  p()=.s
iterations.................:  
vus..........................:  
vus_max......................:  


Key Takeaways:
- http_req_duration: Total request time
- p(): th percentile ( out of  requests faster than this)
- checks: Test assertions pass/fail
- http_requests: Total requests completed

 Comparison: Before vs After Caching

 Test Setup
Run the same test without cache, then with cache enabled:

bash
 Baseline (no cache)
echo "Running baseline test..."
k run --vus  --duration m cache_test.js > results_baseline.txt

 After cache integration
echo "Running cache test..."
k run --vus  --duration m cache_test.js > results_cached.txt


 Expected Comparison

| Metric | Baseline | Cached | Improvement |
|--------|----------|--------|-------------|
| Avg Response Time | ms | ms | % ↓ |
| P Response Time | ms | ms | % ↓ |
| P Response Time | ms | ms | % ↓ |
| Throughput |  req/s |  req/s | x ↑ |
| Cache Hit Rate | % | %+ | New |

 Exporting Results

 JSON Export
bash
k run \
  --out json=results.json \
  cache_test.js


 CSV Export (with extension)
bash
k run \
  --out csv=results.csv \
  cache_test.js


 Analyze JSON Results
bash
 Count successful requests
jq '.metrics[] | select(.type == "check") | select(.data.name == "status is ") | .data.passes' results.json | wc -l

 Average request duration
jq '[.metrics[] | select(.type == "trend") | select(.name == "http_req_duration") | .data.values[]] | add / length' results.json


 Integration with Monitoring

 View Live Metrics in Grafana

. Start monitoring stack:
bash
cd deployment
docker-compose -f docker-compose-monitoring.yaml up -d


. Access Grafana:

http://localhost: (admin/admin)


. Run test and watch dashboard:
bash
cd load_tests
k run cache_test.js &   Run in background
 Watch metrics in Grafana


 Expected Dashboard Updates

During test execution:
- Redis Operations Rate: Increases as requests hit cache
- Cache Hit Ratio: Should increase to %+
- Query Performance: Should decrease (faster with cache)
- DB Connections: Should decrease (less pool usage)

 Troubleshooting

 Connection Refused
Error: Connection refused

Solution:
. Verify backend is running: curl http://localhost:/api/v/health
. Check firewall: sudo ufw allow 
. Verify BASE_URL: k run --env BASE_URL=http://localhost:/api/v cache_test.js

 Authentication Failed
Error:  Unauthorized

Solution:
. Verify test account exists in database
. Check credentials in cache_test.js setup() function
. Run manual login test:
bash
curl -X POST http://localhost:/api/v/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@openrisk.local","password":"admin"}'


 Slow Response Times (No Improvement)
Symptom: Response times not improving after cache integration

Diagnosis:
. Check cache hit rate is > %
. Verify Redis is running: redis-cli -a redis PING
. Check if caching is actually integrated into routes
. Verify cache invalidation isn't too aggressive

Solution:
. Review main.go integration
. Check cache_integration.go is being used
. Verify wrapper methods are applied to endpoints
. Monitor Redis keys: redis-cli -a redis KEYS '' | wc -l

 Out of Memory
Error: OOM: Out of memory

Solution:
. Reduce VUS (virtual users): k run --vus  cache_test.js
. Reduce duration: k run --duration m cache_test.js
. Increase system memory or swap

 Best Practices

 Before Load Testing
- [ ] Ensure backend is stable (no recent deployments)
- [ ] Clear cache data: redis-cli -a redis FLUSHALL
- [ ] Verify database has test data
- [ ] Check system resources (CPU, memory)
- [ ] Disable background jobs/maintenance

 During Load Testing
- [ ] Monitor system metrics (CPU, memory, connections)
- [ ] Watch for error spikes
- [ ] Note response time trends
- [ ] Observe cache hit rate increases

 After Load Testing
- [ ] Export results for comparison
- [ ] Document performance metrics
- [ ] Note any issues or anomalies
- [ ] Review Grafana dashboard for patterns
- [ ] Plan optimization if targets not met

 Advanced Usage

 Custom Threshold Checks
javascript
import { check } from 'k';

check(res, {
  'response time < ms': (r) => r.timings.duration < ,
  'status is  or ': (r) => r.status ===  || r.status === ,
  'cache hit': (r) => r.headers['X-Cache'] === 'HIT',
});


 Rate Limiting
bash
 Limit to  requests per second
k run --rps  cache_test.js


 Custom Payload
javascript
const payload = JSON.stringify({
  title: Risk ${__VU}-${__ITER},
  description: 'Test risk created by k',
  impact: ,
  probability: ,
});

const res = http.post(${BASE_URL}/risks, payload, { headers });


 Performance Baselines

 Recommended Targets

| Scenario | Metric | Target |
|----------|--------|--------|
| Cache Hit Rate | Percentage | > % |
| Response Time P | milliseconds | < ms |
| Response Time P | milliseconds | < ms |
| Error Rate | percentage | < .% |
| Throughput | req/s | >  |
| DB Connections | count | <  |
| Redis Memory | MB | < MB |

 Acceptance Criteria

Phase  Optimization is SUCCESSFUL if:
-  Cache hit rate ≥ %
-  Response time P < ms
-  Throughput >  req/s
-  Error rate < .%
-  DB connections < 
-  All  alerts firing correctly

 Further Reading

- [k Documentation](https://k.io/docs/)
- [k HTTP Requests](https://k.io/docs/javascript-api/k-http/)
- [k Metrics](https://k.io/docs/using-k/metrics/)
- [Performance Testing Best Practices](https://k.io/docs/testing-guides/best-practices/)
- [OpenRisk API Reference](../docs/API_REFERENCE.md)

 Support

For issues or questions:
. Check troubleshooting section above
. Review k documentation
. Check OpenRisk backend logs
. Verify Grafana dashboard metrics
. Contact DevOps team
