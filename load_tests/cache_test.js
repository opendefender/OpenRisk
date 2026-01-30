import http from 'k/http';
import { check, group, sleep } from 'k';
import { Rate, Trend, Counter, Gauge } from 'k/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const successRate = new Rate('success');
const duration = new Trend('request_duration');
const cacheHitRate = new Rate('cache_hits');
const throughput = new Counter('http_requests');

// Options configuration
export const options = {
  stages: [
    { duration: 'm', target:  },   // Ramp up to  users
    { duration: 'm', target:  },   // Stay at  users (warm cache)
    { duration: 'm', target:  },  // Ramp up to  users
    { duration: 'm', target:  },  // Stay at  users (sustained)
    { duration: 'm', target:  },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p()<'],    // th percentile < ms
    'http_req_duration{staticAsset:yes}': ['p()<'],
    errors: ['count<'],               // Fewer than  errors
    'http_req_duration{scenario:get_risks}': ['p()<'], // Risk endpoint < ms
  },
  ext: {
    loadimpact: {
      projectID: ,
      name: 'OpenRisk Performance Test'
    }
  }
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:/api/v';

export function setup() {
  // Setup - runs once at the beginning
  const res = http.post(${BASE_URL}/auth/login, {
    email: 'admin@openrisk.local',
    password: 'admin'
  });

  check(res, {
    'login successful': (r) => r.status === ,
  });

  const data = res.json();
  return {
    token: data.token,
    timestamp: new Date().toISOString()
  };
}

export default function (data) {
  const headers = {
    'Authorization': Bearer ${data.token},
    'Content-Type': 'application/json'
  };

  // Group : Risk List Endpoints (High Cache Impact)
  group('Scenario: List Risks (Cached)', function() {
    const res = http.get(${BASE_URL}/risks?page=&limit=, { headers });
    
    const cacheHit = res.headers['X-Cache'] === 'HIT';
    cacheHitRate.add(cacheHit ?  : );
    duration.add(res.timings.duration, { scenario: 'get_risks' });
    throughput.add();
    
    check(res, {
      'status is ': (r) => r.status === ,
      'has risks': (r) => r.json('data').length > ,
      'response time < ms': (r) => r.timings.duration < ,
    });
    
    errorRate.add(res.status !==  ?  : );
    successRate.add(res.status ===  ?  : );
    
    sleep(.);
  });

  // Group : Dashboard Statistics (High Cache Impact)
  group('Scenario: Dashboard Stats (Cached)', function() {
    const res = http.get(${BASE_URL}/stats, { headers });
    
    const cacheHit = res.headers['X-Cache'] === 'HIT';
    cacheHitRate.add(cacheHit ?  : );
    duration.add(res.timings.duration, { scenario: 'get_stats' });
    throughput.add();
    
    check(res, {
      'status is ': (r) => r.status === ,
      'has stats': (r) => r.json('data') !== null,
      'response time < ms': (r) => r.timings.duration < ,
    });
    
    errorRate.add(res.status !==  ?  : );
    successRate.add(res.status ===  ?  : );
    
    sleep(.);
  });

  // Group : Risk Matrix (Static, High Cache Impact)
  group('Scenario: Risk Matrix (Cached)', function() {
    const res = http.get(${BASE_URL}/stats/risk-matrix, { headers });
    
    const cacheHit = res.headers['X-Cache'] === 'HIT';
    cacheHitRate.add(cacheHit ?  : );
    duration.add(res.timings.duration, { scenario: 'risk_matrix' });
    throughput.add();
    
    check(res, {
      'status is ': (r) => r.status === ,
      'has matrix data': (r) => r.json('data') !== null,
      'response time < ms': (r) => r.timings.duration < ,
    });
    
    errorRate.add(res.status !==  ?  : );
    successRate.add(res.status ===  ?  : );
    
    sleep(.);
  });

  // Group : Search Risks (Cache Miss First, Then Hit)
  group('Scenario: Search Risks (Dynamic)', function() {
    const queries = ['critical', 'high', 'pending', 'active', 'completed'];
    const query = queries[Math.floor(Math.random()  queries.length)];
    
    const res = http.get(${BASE_URL}/risks?search=${query}, { headers });
    
    const cacheHit = res.headers['X-Cache'] === 'HIT';
    cacheHitRate.add(cacheHit ?  : );
    duration.add(res.timings.duration, { scenario: 'search_risks' });
    throughput.add();
    
    check(res, {
      'status is ': (r) => r.status === ,
      'response time < ms': (r) => r.timings.duration < ,
    });
    
    errorRate.add(res.status !==  ?  : );
    successRate.add(res.status ===  ?  : );
    
    sleep(.);
  });

  // Group : Trend Data (Cached)
  group('Scenario: Trends (Cached)', function() {
    const res = http.get(${BASE_URL}/stats/trends, { headers });
    
    const cacheHit = res.headers['X-Cache'] === 'HIT';
    cacheHitRate.add(cacheHit ?  : );
    duration.add(res.timings.duration, { scenario: 'trends' });
    throughput.add();
    
    check(res, {
      'status is ': (r) => r.status === ,
      'has trend data': (r) => r.json('data') !== null,
    });
    
    errorRate.add(res.status !==  ?  : );
    successRate.add(res.status ===  ?  : );
    
    sleep(.);
  });

  // Group : Specific Risk by ID (Cached)
  group('Scenario: Get Risk by ID (Cached)', function() {
    // First get a list to get an ID
    const listRes = http.get(${BASE_URL}/risks?limit=, { headers });
    
    if (listRes.status === ) {
      const risks = listRes.json('data');
      if (risks.length > ) {
        const riskId = risks[].id;
        
        const res = http.get(${BASE_URL}/risks/${riskId}, { headers });
        
        const cacheHit = res.headers['X-Cache'] === 'HIT';
        cacheHitRate.add(cacheHit ?  : );
        duration.add(res.timings.duration, { scenario: 'get_risk_by_id' });
        throughput.add();
        
        check(res, {
          'status is ': (r) => r.status === ,
          'has risk data': (r) => r.json('data.id') === riskId,
          'response time < ms': (r) => r.timings.duration < ,
        });
        
        errorRate.add(res.status !==  ?  : );
        successRate.add(res.status ===  ?  : );
      }
    }
    
    sleep(.);
  });

  sleep();
}

export function teardown(data) {
  // Teardown - runs once at the end
  console.log(Test completed at: ${new Date().toISOString()});
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
  
  summary.push(${indent}Test Summary:);
  summary.push(${indent}=============);
  
  if (data.metrics) {
    if (data.metrics.http_requests) {
      summary.push(${indent}Total Requests: ${data.metrics.http_requests.value});
    }
    if (data.metrics.errors) {
      summary.push(${indent}Error Count: ${data.metrics.errors.value});
    }
    if (data.metrics.success) {
      summary.push(${indent}Success Count: ${data.metrics.success.value});
    }
    if (data.metrics.request_duration) {
      const stats = data.metrics.request_duration.values;
      summary.push(${indent}Response Times:);
      summary.push(${indent}  Min: ${stats.min || }ms);
      summary.push(${indent}  Max: ${stats.max || }ms);
      summary.push(${indent}  Avg: ${stats.avg || }ms);
    }
  }
  
  return summary.join('\n');
}
