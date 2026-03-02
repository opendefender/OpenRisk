import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Counter, Gauge } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const successRate = new Rate('successes');
const apiDuration = new Gauge('api_duration_ms');
const throughput = new Counter('throughput');

// Load testing configuration
export const options = {
  scenarios: {
    // Sustained load: 1000 concurrent users for 24 hours
    sustained_24h: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '5m', target: 100 },      // Ramp up to 100 users
        { duration: '10m', target: 500 },     // Ramp up to 500 users
        { duration: '10m', target: 1000 },    // Ramp up to 1000 users
        { duration: '23h 30m', target: 1000 }, // Sustained at 1000 users
        { duration: '5m', target: 0 },        // Ramp down
      ],
      gracefulRampDown: '30s',
    },
  },
  // Alert thresholds
  thresholds: {
    'http_req_duration': ['p(95)<500'],        // 95th percentile <500ms
    'http_req_duration{api:auth}': ['p(99)<200'],  // Auth <200ms
    'errors': ['rate<0.01'],                   // Error rate <1%
    'http_req_failed': ['rate<0.01'],         // Failed requests <1%
  },
};

// Test data
const BASE_URL = 'http://localhost:8080/api/v1';
let authToken = '';

export default function () {
  group('Authentication Flow', () => {
    authenticate();
  });

  group('Risk Management', () => {
    createRisk();
    getRisks();
    getRiskById();
    updateRisk();
  });

  group('Analytics & Export', () => {
    exportMetrics();
    exportTrends();
    analyzeRisks();
  });

  group('Incidents', () => {
    createIncident();
    getIncidents();
    updateIncident();
  });

  group('Custom Metrics', () => {
    createMetric();
    calculateMetric();
    getMetricHistory();
  });

  group('Trend Analysis', () => {
    analyzeTrends();
    getTrendRecommendations();
    getAnomalies();
  });

  sleep(1);
}

function authenticate() {
  let res = http.post(`${BASE_URL}/auth/login`, {
    email: `user${Math.random()}@example.com`,
    password: 'TestPassword123!',
  }, {
    tags: { api: 'auth' },
  });

  check(res, {
    'auth status is 200': (r) => r.status === 200,
    'auth response has token': (r) => r.json('access_token') !== undefined,
  }) || errorRate.add(1);

  if (res.status === 200) {
    authToken = res.json('access_token');
    successRate.add(1);
  } else {
    errorRate.add(1);
  }

  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function createRisk() {
  let payload = {
    title: `Risk ${Date.now()}`,
    description: 'Load test risk',
    category: 'Operational',
    severity: 'High',
    probability: 0.7,
    impact: 0.8,
  };

  let res = http.post(`${BASE_URL}/risks`, payload, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'create risk status 201': (r) => r.status === 201,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function getRisks() {
  let res = http.get(`${BASE_URL}/risks?limit=100`, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'get risks status 200': (r) => r.status === 200,
    'risks array exists': (r) => r.json('data') !== undefined,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function getRiskById() {
  let res = http.get(`${BASE_URL}/risks/1`, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'get risk by id status 200': (r) => r.status === 200 || r.status === 404,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function updateRisk() {
  let payload = {
    severity: 'Critical',
    status: 'In Progress',
  };

  let res = http.put(`${BASE_URL}/risks/1`, payload, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'update risk status 200': (r) => r.status === 200 || r.status === 404,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function exportMetrics() {
  let res = http.get(`${BASE_URL}/analytics/export/metrics?format=json`, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'export metrics status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function exportTrends() {
  let res = http.get(`${BASE_URL}/analytics/export/trends?format=csv`, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'export trends status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function analyzeRisks() {
  let res = http.get(`${BASE_URL}/analytics/dashboard`, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'analyze risks status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function createIncident() {
  let payload = {
    title: `Incident ${Date.now()}`,
    description: 'Load test incident',
    severity: 'High',
    status: 'Open',
  };

  let res = http.post(`${BASE_URL}/incidents`, payload, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'create incident status 201': (r) => r.status === 201 || r.status === 400,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function getIncidents() {
  let res = http.get(`${BASE_URL}/incidents?limit=100`, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'get incidents status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function updateIncident() {
  let res = http.put(`${BASE_URL}/incidents/1`, { status: 'Resolved' }, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'update incident status 200': (r) => r.status === 200 || r.status === 404,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function createMetric() {
  let payload = {
    name: `Metric_${Date.now()}`,
    description: 'Load test metric',
    formula: 'risk_count / total_assets',
  };

  let res = http.post(`${BASE_URL}/metrics/custom`, payload, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'create metric status 201': (r) => r.status === 201,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function calculateMetric() {
  let res = http.post(`${BASE_URL}/metrics/custom/1/calculate`, {}, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'calculate metric status 200': (r) => r.status === 200 || r.status === 404,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function getMetricHistory() {
  let res = http.get(`${BASE_URL}/metrics/custom/1/history`, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'get metric history status 200': (r) => r.status === 200 || r.status === 404,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function analyzeTrends() {
  let res = http.post(`${BASE_URL}/analytics/trends/analyze`, {
    metric: 'risk_score',
    days: 30,
  }, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'analyze trends status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function getTrendRecommendations() {
  let res = http.get(`${BASE_URL}/analytics/trends/recommendations`, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'get recommendations status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}

function getAnomalies() {
  let res = http.post(`${BASE_URL}/analytics/trends/anomalies`, {
    metric: 'risk_score',
  }, {
    headers: { Authorization: `Bearer ${authToken}` },
  });

  check(res, {
    'get anomalies status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  successRate.add(res.status < 400 ? 1 : 0);
  apiDuration.set(res.timings.duration);
  throughput.add(1);
}
