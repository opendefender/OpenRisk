import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend, Counter, Gauge } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const successRate = new Rate('success');
const duration = new Trend('request_duration');
const cacheHitRate = new Rate('cache_hits');
const throughput = new Counter('http_requests');

// Options configuration
export const options = {
  stages: [
    { duration: '1m', target: 5 },   // Ramp up to 5 users
    { duration: '2m', target: 5 },   // Stay at 5 users (warm cache)
    { duration: '1m', target: 10 },  // Ramp up to 10 users
    { duration: '2m', target: 10 },  // Stay at 10 users (sustained)
    { duration: '1m', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<100'],    // 95th percentile < 100ms
    'http_req_duration{staticAsset:yes}': ['p(99)<250'],
    errors: ['count<100'],               // Fewer than 100 errors
    'http_req_duration{scenario:get_risks}': ['p(99)<150'], // Risk endpoint < 150ms
  },
  ext: {
    loadimpact: {
      projectID: 3356643,
      name: 'OpenRisk Performance Test'
    }
  }
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000/api/v1';

export function setup() {
  // Setup - runs once at the beginning
  const res = http.post(`${BASE_URL}/auth/login`, {
    email: 'admin@openrisk.local',
    password: 'admin123'
  });

  check(res, {
    'login successful': (r) => r.status === 200,
  });

  const data = res.json();
  return {
    token: data.token,
    timestamp: new Date().toISOString()
  };
}

export default function (data) {
  const headers = {
    'Authorization': `Bearer ${data.token}`,
    'Content-Type': 'application/json'
  };

  // Group 1: Risk List Endpoints (High Cache Impact)
  group('Scenario: List Risks (Cached)', function() {
    const res = http.get(`${BASE_URL}/risks?page=0&limit=20`, { headers });
    
    const cacheHit = res.headers['X-Cache'] === 'HIT';
    cacheHitRate.add(cacheHit ? 1 : 0);
    duration.add(res.timings.duration, { scenario: 'get_risks' });
    throughput.add(1);
    
    check(res, {
      'status is 200': (r) => r.status === 200,
      'has risks': (r) => r.json('data').length > 0,
      'response time < 100ms': (r) => r.timings.duration < 100,
    });
    
    errorRate.add(res.status !== 200 ? 1 : 0);
    successRate.add(res.status === 200 ? 1 : 0);
    
    sleep(0.5);
  });

  // Group 2: Dashboard Statistics (High Cache Impact)
  group('Scenario: Dashboard Stats (Cached)', function() {
    const res = http.get(`${BASE_URL}/stats`, { headers });
    
    const cacheHit = res.headers['X-Cache'] === 'HIT';
    cacheHitRate.add(cacheHit ? 1 : 0);
    duration.add(res.timings.duration, { scenario: 'get_stats' });
    throughput.add(1);
    
    check(res, {
      'status is 200': (r) => r.status === 200,
      'has stats': (r) => r.json('data') !== null,
      'response time < 100ms': (r) => r.timings.duration < 100,
    });
    
    errorRate.add(res.status !== 200 ? 1 : 0);
    successRate.add(res.status === 200 ? 1 : 0);
    
    sleep(0.5);
  });

  // Group 3: Risk Matrix (Static, High Cache Impact)
  group('Scenario: Risk Matrix (Cached)', function() {
    const res = http.get(`${BASE_URL}/stats/risk-matrix`, { headers });
    
    const cacheHit = res.headers['X-Cache'] === 'HIT';
    cacheHitRate.add(cacheHit ? 1 : 0);
    duration.add(res.timings.duration, { scenario: 'risk_matrix' });
    throughput.add(1);
    
    check(res, {
      'status is 200': (r) => r.status === 200,
      'has matrix data': (r) => r.json('data') !== null,
      'response time < 100ms': (r) => r.timings.duration < 100,
    });
    
    errorRate.add(res.status !== 200 ? 1 : 0);
    successRate.add(res.status === 200 ? 1 : 0);
    
    sleep(0.5);
  });

  // Group 4: Search Risks (Cache Miss First, Then Hit)
  group('Scenario: Search Risks (Dynamic)', function() {
    const queries = ['critical', 'high', 'pending', 'active', 'completed'];
    const query = queries[Math.floor(Math.random() * queries.length)];
    
    const res = http.get(`${BASE_URL}/risks?search=${query}`, { headers });
    
    const cacheHit = res.headers['X-Cache'] === 'HIT';
    cacheHitRate.add(cacheHit ? 1 : 0);
    duration.add(res.timings.duration, { scenario: 'search_risks' });
    throughput.add(1);
    
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 150ms': (r) => r.timings.duration < 150,
    });
    
    errorRate.add(res.status !== 200 ? 1 : 0);
    successRate.add(res.status === 200 ? 1 : 0);
    
    sleep(0.3);
  });

  // Group 5: Trend Data (Cached)
  group('Scenario: Trends (Cached)', function() {
    const res = http.get(`${BASE_URL}/stats/trends`, { headers });
    
    const cacheHit = res.headers['X-Cache'] === 'HIT';
    cacheHitRate.add(cacheHit ? 1 : 0);
    duration.add(res.timings.duration, { scenario: 'trends' });
    throughput.add(1);
    
    check(res, {
      'status is 200': (r) => r.status === 200,
      'has trend data': (r) => r.json('data') !== null,
    });
    
    errorRate.add(res.status !== 200 ? 1 : 0);
    successRate.add(res.status === 200 ? 1 : 0);
    
    sleep(0.5);
  });

  // Group 6: Specific Risk by ID (Cached)
  group('Scenario: Get Risk by ID (Cached)', function() {
    // First get a list to get an ID
    const listRes = http.get(`${BASE_URL}/risks?limit=1`, { headers });
    
    if (listRes.status === 200) {
      const risks = listRes.json('data');
      if (risks.length > 0) {
        const riskId = risks[0].id;
        
        const res = http.get(`${BASE_URL}/risks/${riskId}`, { headers });
        
        const cacheHit = res.headers['X-Cache'] === 'HIT';
        cacheHitRate.add(cacheHit ? 1 : 0);
        duration.add(res.timings.duration, { scenario: 'get_risk_by_id' });
        throughput.add(1);
        
        check(res, {
          'status is 200': (r) => r.status === 200,
          'has risk data': (r) => r.json('data.id') === riskId,
          'response time < 100ms': (r) => r.timings.duration < 100,
        });
        
        errorRate.add(res.status !== 200 ? 1 : 0);
        successRate.add(res.status === 200 ? 1 : 0);
      }
    }
    
    sleep(0.5);
  });

  sleep(1);
}

export function teardown(data) {
  // Teardown - runs once at the end
  console.log(`Test completed at: ${new Date().toISOString()}`);
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
  };
}

function textSummary(data, options) {
  options = options || {};
  
  const indent = options.indent || '';
  const summary = [];
  
  summary.push(`${indent}Test Summary:`);
  summary.push(`${indent}=============`);
  
  if (data.metrics) {
    if (data.metrics.http_requests) {
      summary.push(`${indent}Total Requests: ${data.metrics.http_requests.value}`);
    }
    if (data.metrics.errors) {
      summary.push(`${indent}Error Count: ${data.metrics.errors.value}`);
    }
    if (data.metrics.success) {
      summary.push(`${indent}Success Count: ${data.metrics.success.value}`);
    }
    if (data.metrics.request_duration) {
      const stats = data.metrics.request_duration.values;
      summary.push(`${indent}Response Times:`);
      summary.push(`${indent}  Min: ${stats.min || 0}ms`);
      summary.push(`${indent}  Max: ${stats.max || 0}ms`);
      summary.push(`${indent}  Avg: ${stats.avg || 0}ms`);
    }
  }
  
  return summary.join('\n');
}
