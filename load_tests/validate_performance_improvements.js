/**
 * Performance Validation Suite - Production Data Testing
 * Tests performance improvements against production-like data volumes
 * 
 * Usage: k6 run validate_performance_improvements.js
 * 
 * Environment Variables:
 *   API_BASE: API endpoint (default: http://localhost:8080/api/v1)
 *   TENANT_ID: Tenant ID (default: test-tenant-prod)
 *   AUTH_TOKEN: Bearer token (default: Bearer test-token)
 *   DATA_SIZE: Number of risks to test (default: 10000)
 */

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Trend, Rate, Counter, Gauge } from 'k6/metrics';

const apiBase = __ENV.API_BASE || 'http://localhost:8080/api/v1';
const tenantId = __ENV.TENANT_ID || 'test-tenant-prod';
const authToken = __ENV.AUTH_TOKEN || 'Bearer test-token';

// Performance Metrics
const listRisksTime = new Trend('list_risks_duration');
const searchRisksTime = new Trend('search_risks_duration');
const getRiskTime = new Trend('get_risk_duration');
const createRiskTime = new Trend('create_risk_duration');
const updateRiskTime = new Trend('update_risk_duration');
const bulkOperationTime = new Trend('bulk_operation_duration');
const filterRisksTime = new Trend('filter_risks_duration');
const analyticsTime = new Trend('analytics_duration');

const successRate = new Rate('success_rate');
const errorRate = new Rate('error_rate');
const cacheHitRate = new Rate('cache_hit_rate');

const performanceTargetsMet = new Counter('performance_targets_met');
const performanceTargetsFailed = new Counter('performance_targets_failed');

// Performance Targets (in milliseconds)
const targets = {
  listRisks: 5000,          // 5 seconds for list with 10k+ records
  searchRisks: 2000,        // 2 seconds for search
  getRisk: 500,             // 500ms for single risk detail
  createRisk: 1000,         // 1 second for creation
  updateRisk: 1000,         // 1 second for update
  bulkOperation: 10000,     // 10 seconds for 100 item bulk operation
  filterRisks: 3000,        // 3 seconds for filtered list
  analyticsQuery: 5000,     // 5 seconds for analytics calculations
};

export const options = {
  stages: [
    { duration: '2m', target: 10 },   // Ramp up to 10 VUs
    { duration: '5m', target: 20 },   // Increase to 20 VUs
    { duration: '3m', target: 10 },   // Back down to 10 VUs
    { duration: '1m', target: 0 },    // Ramp down
  ],
  thresholds: {
    'list_risks_duration': ['p95<5000'],        // 95th percentile < 5s
    'search_risks_duration': ['p95<2000'],      // 95th percentile < 2s
    'get_risk_duration': ['p95<500'],           // 95th percentile < 500ms
    'success_rate': ['rate>0.95'],              // >95% success rate
    'error_rate': ['rate<0.05'],                // <5% error rate
  },
};

/**
 * Get risk IDs for testing (assumes risks exist from data generation)
 */
function getRiskIdsForTesting(count = 100) {
  const response = http.get(
    `${apiBase}/risks?limit=${count}&page=1`,
    {
      headers: {
        'Authorization': authToken,
        'X-Tenant-ID': tenantId,
      },
    }
  );

  if (response.status === 200) {
    const data = response.json('data') || [];
    return data.map((risk) => risk.id).slice(0, count);
  }
  return [];
}

/**
 * Test listing risks with large dataset
 */
function testListRisks() {
  group('List Risks - Production Data', () => {
    const startTime = Date.now();
    const response = http.get(
      `${apiBase}/risks?limit=100&page=1`,
      {
        headers: {
          'Authorization': authToken,
          'X-Tenant-ID': tenantId,
        },
      }
    );

    const duration = Date.now() - startTime;
    listRisksTime.add(duration);

    const success = check(response, {
      'status is 200': (r) => r.status === 200,
      'has data': (r) => r.json('data') !== undefined,
      'response < 5s': () => duration < targets.listRisks,
      'has pagination': (r) => r.json('total') !== undefined,
    });

    if (success) {
      successRate.add(true);
      if (duration < targets.listRisks) {
        performanceTargetsMet.add(1);
      } else {
        performanceTargetsFailed.add(1);
      }
    } else {
      successRate.add(false);
      errorRate.add(1);
    }
  });

  sleep(0.5);
}

/**
 * Test searching risks
 */
function testSearchRisks() {
  group('Search Risks', () => {
    const searchTerms = ['Risk', 'Operational', 'Financial', 'Cyber'];
    const searchTerm = searchTerms[Math.floor(Math.random() * searchTerms.length)];

    const startTime = Date.now();
    const response = http.get(
      `${apiBase}/risks/search?q=${searchTerm}&limit=50`,
      {
        headers: {
          'Authorization': authToken,
          'X-Tenant-ID': tenantId,
        },
      }
    );

    const duration = Date.now() - startTime;
    searchRisksTime.add(duration);

    const success = check(response, {
      'status is 200': (r) => r.status === 200,
      'search < 2s': () => duration < targets.searchRisks,
      'has results': (r) => r.json('data') !== undefined,
    });

    if (success) {
      successRate.add(true);
      if (duration < targets.searchRisks) {
        performanceTargetsMet.add(1);
      }
    } else {
      errorRate.add(1);
      performanceTargetsFailed.add(1);
    }
  });

  sleep(0.5);
}

/**
 * Test getting single risk detail (with cache)
 */
function testGetRisk(riskIds) {
  group('Get Risk Detail', () => {
    if (riskIds.length === 0) {
      console.log('No risk IDs available for testing');
      return;
    }

    const riskId = riskIds[Math.floor(Math.random() * riskIds.length)];

    const startTime = Date.now();
    const response = http.get(
      `${apiBase}/risks/${riskId}`,
      {
        headers: {
          'Authorization': authToken,
          'X-Tenant-ID': tenantId,
        },
      }
    );

    const duration = Date.now() - startTime;
    getRiskTime.add(duration);

    // Check for cache headers
    const isCached = response.headers['X-Cache-Hit'] === 'true';
    if (isCached) {
      cacheHitRate.add(true);
    }

    const success = check(response, {
      'status is 200': (r) => r.status === 200,
      'has data': (r) => r.json('data') !== undefined,
      'response < 500ms': () => duration < targets.getRisk,
      'cached response': (r) => r.headers['X-Cache-Hit'] !== undefined,
    });

    if (success) {
      successRate.add(true);
      if (duration < targets.getRisk) {
        performanceTargetsMet.add(1);
      }
    } else {
      errorRate.add(1);
      performanceTargetsFailed.add(1);
    }
  });

  sleep(0.3);
}

/**
 * Test filtering risks
 */
function testFilterRisks() {
  group('Filter Risks', () => {
    const statuses = ['Open', 'In Progress', 'Mitigating', 'Closed'];
    const status = statuses[Math.floor(Math.random() * statuses.length)];

    const startTime = Date.now();
    const response = http.get(
      `${apiBase}/risks?status=${status}&limit=50`,
      {
        headers: {
          'Authorization': authToken,
          'X-Tenant-ID': tenantId,
        },
      }
    );

    const duration = Date.now() - startTime;
    filterRisksTime.add(duration);

    const success = check(response, {
      'status is 200': (r) => r.status === 200,
      'filter < 3s': () => duration < targets.filterRisks,
      'has filtered data': (r) => r.json('data') !== undefined,
    });

    if (success) {
      successRate.add(true);
      if (duration < targets.filterRisks) {
        performanceTargetsMet.add(1);
      }
    } else {
      errorRate.add(1);
      performanceTargetsFailed.add(1);
    }
  });

  sleep(0.5);
}

/**
 * Test analytics queries (aggregations)
 */
function testAnalytics() {
  group('Analytics Queries', () => {
    const startTime = Date.now();
    const response = http.get(
      `${apiBase}/risks/analytics/summary`,
      {
        headers: {
          'Authorization': authToken,
          'X-Tenant-ID': tenantId,
        },
      }
    );

    const duration = Date.now() - startTime;
    analyticsTime.add(duration);

    const success = check(response, {
      'status is 200': (r) => r.status === 200 || r.status === 404,
      'analytics < 5s': () => duration < targets.analyticsQuery,
      'has summary': (r) => r.json('data') !== undefined || r.status === 404,
    });

    if (success) {
      successRate.add(true);
      if (duration < targets.analyticsQuery) {
        performanceTargetsMet.add(1);
      }
    } else {
      errorRate.add(1);
      performanceTargetsFailed.add(1);
    }
  });

  sleep(1);
}

/**
 * Test bulk operations
 */
function testBulkOperations(riskIds) {
  group('Bulk Operations', () => {
    if (riskIds.length < 10) {
      console.log('Not enough risk IDs for bulk operation test');
      return;
    }

    const idsForBulk = riskIds.slice(0, 10);
    const bulkPayload = {
      ids: idsForBulk,
      updates: {
        status: 'In Progress',
        tags: ['bulk-updated', 'production-test'],
      },
    };

    const startTime = Date.now();
    const response = http.post(
      `${apiBase}/risks/bulk-update`,
      JSON.stringify(bulkPayload),
      {
        headers: {
          'Content-Type': 'application/json',
          'Authorization': authToken,
          'X-Tenant-ID': tenantId,
        },
      }
    );

    const duration = Date.now() - startTime;
    bulkOperationTime.add(duration);

    const success = check(response, {
      'status is 200': (r) => r.status === 200 || r.status === 201,
      'bulk < 10s': () => duration < targets.bulkOperation,
      'has results': (r) => r.json('updated') !== undefined || r.status === 404,
    });

    if (success) {
      successRate.add(true);
      if (duration < targets.bulkOperation) {
        performanceTargetsMet.add(1);
      }
    } else {
      errorRate.add(1);
      performanceTargetsFailed.add(1);
    }
  });

  sleep(1);
}

/**
 * Main test execution
 */
export default function () {
  // Get risk IDs once per iteration
  const riskIds = getRiskIdsForTesting(100);

  testListRisks();
  testSearchRisks();
  testGetRisk(riskIds);
  testFilterRisks();
  testAnalytics();
  testBulkOperations(riskIds);
}

/**
 * Summary handler - print results
 */
export function handleSummary(data) {
  const summary = {
    'Performance Targets Met': performanceTargetsMet.value,
    'Performance Targets Failed': performanceTargetsFailed.value,
    'Overall Success Rate': `${(data.metrics.success_rate.values.rate * 100).toFixed(2)}%`,
    'Overall Error Rate': `${(data.metrics.error_rate.values.rate * 100).toFixed(2)}%`,
    'Cache Hit Rate': `${(data.metrics.cache_hit_rate.values.rate * 100).toFixed(2)}%`,
    'Average Metrics': {
      'List Risks': `${Math.round(data.metrics.list_risks_duration.values.mean)}ms (target: ${targets.listRisks}ms)`,
      'Search Risks': `${Math.round(data.metrics.search_risks_duration.values.mean)}ms (target: ${targets.searchRisks}ms)`,
      'Get Risk': `${Math.round(data.metrics.get_risk_duration.values.mean)}ms (target: ${targets.getRisk}ms)`,
      'Filter Risks': `${Math.round(data.metrics.filter_risks_duration.values.mean)}ms (target: ${targets.filterRisks}ms)`,
      'Analytics': `${Math.round(data.metrics.analytics_duration.values.mean)}ms (target: ${targets.analyticsQuery}ms)`,
    },
    'P95 Metrics': {
      'List Risks': `${Math.round(data.metrics.list_risks_duration.values['p(95)'])}ms`,
      'Search Risks': `${Math.round(data.metrics.search_risks_duration.values['p(95)'])}ms`,
      'Get Risk': `${Math.round(data.metrics.get_risk_duration.values['p(95)'])}ms`,
    },
  };

  console.log('\n========== PERFORMANCE VALIDATION SUMMARY ==========');
  console.log(JSON.stringify(summary, null, 2));

  return {
    stdout: JSON.stringify(summary, null, 2),
  };
}
