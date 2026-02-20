import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend, Counter, Gauge } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const requestDuration = new Trend('request_duration');
const successfulRequests = new Counter('successful_requests');
const activeConnections = new Gauge('active_connections');

export const options = {
  stages: [
    { duration: '30s', target: 10 },   // Ramp up to 10 users
    { duration: '1m30s', target: 50 }, // Ramp up to 50 users
    { duration: '2m', target: 50 },    // Stay at 50 users
    { duration: '30s', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    errors: ['rate<0.1'],
  },
};

const BASE_URL = 'http://localhost:8080/api/v1';
const TOKEN = __ENV.TOKEN || 'test-token';

export function setup() {
  // Setup code: authenticate and prepare test data
  console.log('Setting up load test...');
  
  return {
    token: TOKEN,
    baseURL: BASE_URL,
  };
}

export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  activeConnections.add(1);

  // Group 1: Risk CRUD Operations
  group('Risk Operations', () => {
    // Create Risk
    const createRiskPayload = JSON.stringify({
      title: `Test Risk ${Date.now()}`,
      description: 'Performance test risk',
      impact: 3,
      probability: 2,
      tags: ['test', 'performance'],
      asset_ids: [],
      frameworks: ['ISO31000'],
    });

    const createRes = http.post(`${BASE_URL}/risks`, createRiskPayload, {
      headers,
    });

    const createSuccess = check(createRes, {
      'Risk create status 201': (r) => r.status === 201,
      'Risk create response time': (r) => r.timings.duration < 500,
    });

    if (!createSuccess) {
      errorRate.add(1);
    } else {
      successfulRequests.add(1);
    }
    requestDuration.add(createRes.timings.duration);

    const riskId = createRes.json('id');

    // Get Risk
    if (riskId) {
      const getRes = http.get(`${BASE_URL}/risks/${riskId}`, { headers });
      
      const getSuccess = check(getRes, {
        'Risk get status 200': (r) => r.status === 200,
        'Risk get response time': (r) => r.timings.duration < 300,
      });

      if (!getSuccess) {
        errorRate.add(1);
      } else {
        successfulRequests.add(1);
      }
      requestDuration.add(getRes.timings.duration);

      // Update Risk
      const updatePayload = JSON.stringify({
        title: `Updated Risk ${Date.now()}`,
        status: 'active',
      });

      const updateRes = http.patch(`${BASE_URL}/risks/${riskId}`, updatePayload, {
        headers,
      });

      const updateSuccess = check(updateRes, {
        'Risk update status 200': (r) => r.status === 200,
        'Risk update response time': (r) => r.timings.duration < 500,
      });

      if (!updateSuccess) {
        errorRate.add(1);
      } else {
        successfulRequests.add(1);
      }
      requestDuration.add(updateRes.timings.duration);
    }
  });

  // Group 2: List and Search Operations
  group('List and Search', () => {
    // List Risks with pagination
    const listRes = http.get(
      `${BASE_URL}/risks?page=1&limit=20&sort_by=score`,
      { headers }
    );

    const listSuccess = check(listRes, {
      'Risk list status 200': (r) => r.status === 200,
      'Risk list response time': (r) => r.timings.duration < 800,
      'Risk list has items': (r) => r.json('items').length > 0,
    });

    if (!listSuccess) {
      errorRate.add(1);
    } else {
      successfulRequests.add(1);
    }
    requestDuration.add(listRes.timings.duration);

    // Search Risks
    const searchRes = http.get(`${BASE_URL}/risks?q=test&status=active`, {
      headers,
    });

    const searchSuccess = check(searchRes, {
      'Risk search status 200': (r) => r.status === 200,
      'Risk search response time': (r) => r.timings.duration < 1000,
    });

    if (!searchSuccess) {
      errorRate.add(1);
    } else {
      successfulRequests.add(1);
    }
    requestDuration.add(searchRes.timings.duration);
  });

  // Group 3: Analytics Operations
  group('Analytics', () => {
    const analyticsRes = http.get(`${BASE_URL}/analytics/dashboard`, { headers });

    const analyticsSuccess = check(analyticsRes, {
      'Analytics status 200': (r) => r.status === 200,
      'Analytics response time': (r) => r.timings.duration < 2000,
    });

    if (!analyticsSuccess) {
      errorRate.add(1);
    } else {
      successfulRequests.add(1);
    }
    requestDuration.add(analyticsRes.timings.duration);
  });

  // Group 4: Concurrent Risk Reads
  group('Concurrent Reads', () => {
    const batch = http.batch([
      ['GET', `${BASE_URL}/risks?limit=10`, null, { headers }],
      ['GET', `${BASE_URL}/risks?limit=10&offset=10`, null, { headers }],
      ['GET', `${BASE_URL}/risks?limit=10&offset=20`, null, { headers }],
      ['GET', `${BASE_URL}/risks?status=active`, null, { headers }],
      ['GET', `${BASE_URL}/analytics/risks/metrics`, null, { headers }],
    ]);

    batch.forEach((res) => {
      check(res, {
        'Batch request status 200': (r) => r.status === 200,
        'Batch request response time': (r) => r.timings.duration < 1000,
      });
      requestDuration.add(res.timings.duration);
    });
  });

  activeConnections.add(-1);
  sleep(1);
}

export function teardown(data) {
  console.log('Load test completed');
  console.log(`Total successful requests: ${successfulRequests.value}`);
  console.log(`Average request duration: ${requestDuration.value}ms`);
  console.log(`Error rate: ${errorRate.value * 100}%`);
}
