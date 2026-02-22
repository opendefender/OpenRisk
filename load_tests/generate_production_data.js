/**
 * Production-Like Data Generator for OpenRisk
 * Generates realistic data volumes for performance validation
 * 
 * Usage: k6 run generate_production_data.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend, Gauge } from 'k6/metrics';

const apiBase = __ENV.API_BASE || 'http://localhost:8080/api/v1';
const tenantId = __ENV.TENANT_ID || 'test-tenant-prod';
const authToken = __ENV.AUTH_TOKEN || 'Bearer test-token';

// Metrics
const dataGenerated = new Counter('data_generated');
const generationTime = new Trend('generation_time');
const generationErrors = new Counter('generation_errors');

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    generation_errors: ['count < 5'],
  },
};

// Configuration for production-like data
const config = {
  risks: 10000,           // 10k risks
  assetsPerRisk: 3,       // 3 assets per risk
  mitigationsPerRisk: 2,  // 2 mitigations per risk
  subActionsPerMitigation: 3, // 3 sub-actions per mitigation
  customFieldsPerRisk: 5, // 5 custom fields per risk
  batchSize: 100,         // Bulk insert batch size
};

// Risk categories for realistic data
const riskCategories = [
  'Operational',
  'Financial',
  'Strategic',
  'Compliance',
  'Cyber Security',
  'Reputational',
  'Market',
  'Regulatory',
];

// Risk statuses
const riskStatuses = ['Open', 'In Progress', 'Mitigating', 'Closed'];

// Asset types
const assetTypes = ['System', 'Application', 'Database', 'Server', 'Network', 'Process'];

// Mitigation statuses
const mitigationStatuses = ['Planned', 'In Progress', 'Completed'];

/**
 * Generate random integer between min and max
 */
function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

/**
 * Generate random risk score (1-100)
 */
function randomScore() {
  return randomInt(1, 100);
}

/**
 * Generate realistic risk data
 */
function generateRisk(index) {
  const probability = randomInt(1, 10); // 1-10
  const impact = randomInt(1, 10);      // 1-10
  const score = probability * impact;    // 1-100

  return {
    title: `Risk #${index}: ${riskCategories[index % riskCategories.length]} Risk`,
    description: `This is a production-like risk scenario #${index} with realistic data complexity`,
    category: riskCategories[index % riskCategories.length],
    status: riskStatuses[index % riskStatuses.length],
    probability: probability,
    impact: impact,
    score: score,
    owner: `owner-${index % 100}@company.com`,
    tags: [
      riskCategories[index % riskCategories.length],
      `priority-${probability}`,
      `department-${index % 5}`,
    ],
  };
}

/**
 * Generate realistic asset data
 */
function generateAsset(riskIndex, assetIndex) {
  return {
    name: `Asset-${riskIndex}-${assetIndex}`,
    type: assetTypes[assetIndex % assetTypes.length],
    description: `Asset supporting risk mitigation for risk #${riskIndex}`,
    location: `Location-${riskIndex % 10}`,
    owner: `asset-owner-${assetIndex}@company.com`,
    status: 'Active',
  };
}

/**
 * Generate realistic mitigation data
 */
function generateMitigation(riskIndex, mitigationIndex) {
  const daysUntilDue = randomInt(7, 180);
  const dueDate = new Date();
  dueDate.setDate(dueDate.getDate() + daysUntilDue);

  return {
    title: `Mitigation ${mitigationIndex + 1} for Risk #${riskIndex}`,
    description: `Action plan to mitigate risk #${riskIndex}`,
    owner: `mitigation-owner-${mitigationIndex}@company.com`,
    status: mitigationStatuses[mitigationIndex % mitigationStatuses.length],
    dueDate: dueDate.toISOString(),
    budget: randomInt(5000, 500000),
  };
}

/**
 * Generate sub-actions for mitigation
 */
function generateSubAction(riskIndex, mitigationIndex, actionIndex) {
  return {
    description: `Sub-action ${actionIndex + 1}: ${['Planning', 'Implementation', 'Verification', 'Review'][actionIndex % 4]}`,
    responsible: `team-member-${actionIndex}@company.com`,
    dueDate: new Date(Date.now() + randomInt(7, 60) * 24 * 60 * 60 * 1000).toISOString(),
    completed: actionIndex % 3 === 0, // Some completed
  };
}

/**
 * Generate custom field data
 */
function generateCustomField(riskIndex, fieldIndex) {
  const fieldTypes = ['text', 'number', 'date', 'select', 'checkbox'];
  const values = ['Value A', '42', new Date().toISOString(), 'Option 1', 'true'];

  return {
    name: `custom_field_${fieldIndex}`,
    value: values[fieldIndex % values.length],
    type: fieldTypes[fieldIndex % fieldTypes.length],
  };
}

/**
 * Bulk create risks
 */
function bulkCreateRisks(startIndex, count) {
  const risks = [];
  for (let i = 0; i < count; i++) {
    risks.push(generateRisk(startIndex + i));
  }

  const startTime = Date.now();
  const response = http.post(
    `${apiBase}/risks/bulk`,
    JSON.stringify({ risks }),
    {
      headers: {
        'Content-Type': 'application/json',
        'Authorization': authToken,
        'X-Tenant-ID': tenantId,
      },
    }
  );

  const duration = Date.now() - startTime;
  generationTime.add(duration);

  check(response, {
    'bulk risks created successfully': (r) => r.status === 200 || r.status === 201,
    'response has IDs': (r) => r.json('ids') !== undefined,
  });

  if (response.status !== 200 && response.status !== 201) {
    generationErrors.add(1);
  } else {
    dataGenerated.add(count);
  }

  return response.json('ids') || [];
}

/**
 * Create assets for a risk
 */
function createAssetsForRisk(riskId, riskIndex) {
  for (let i = 0; i < config.assetsPerRisk; i++) {
    const asset = generateAsset(riskIndex, i);
    const response = http.post(
      `${apiBase}/risks/${riskId}/assets`,
      JSON.stringify(asset),
      {
        headers: {
          'Content-Type': 'application/json',
          'Authorization': authToken,
          'X-Tenant-ID': tenantId,
        },
      }
    );

    if (response.status === 200 || response.status === 201) {
      dataGenerated.add(1);
    } else {
      generationErrors.add(1);
    }
  }
}

/**
 * Create mitigations for a risk
 */
function createMitigationsForRisk(riskId, riskIndex) {
  for (let m = 0; m < config.mitigationsPerRisk; m++) {
    const mitigation = generateMitigation(riskIndex, m);
    const response = http.post(
      `${apiBase}/risks/${riskId}/mitigations`,
      JSON.stringify(mitigation),
      {
        headers: {
          'Content-Type': 'application/json',
          'Authorization': authToken,
          'X-Tenant-ID': tenantId,
        },
      }
    );

    if (response.status === 200 || response.status === 201) {
      const mitigationId = response.json('id');
      dataGenerated.add(1);

      // Add sub-actions
      if (mitigationId) {
        for (let a = 0; a < config.subActionsPerMitigation; a++) {
          const action = generateSubAction(riskIndex, m, a);
          const actionResponse = http.post(
            `${apiBase}/mitigations/${mitigationId}/actions`,
            JSON.stringify(action),
            {
              headers: {
                'Content-Type': 'application/json',
                'Authorization': authToken,
                'X-Tenant-ID': tenantId,
              },
            }
          );

          if (actionResponse.status === 200 || actionResponse.status === 201) {
            dataGenerated.add(1);
          } else {
            generationErrors.add(1);
          }
        }
      }
    } else {
      generationErrors.add(1);
    }
  }
}

/**
 * Create custom fields for a risk
 */
function createCustomFieldsForRisk(riskId, riskIndex) {
  for (let i = 0; i < config.customFieldsPerRisk; i++) {
    const field = generateCustomField(riskIndex, i);
    const response = http.post(
      `${apiBase}/risks/${riskId}/custom-fields`,
      JSON.stringify(field),
      {
        headers: {
          'Content-Type': 'application/json',
          'Authorization': authToken,
          'X-Tenant-ID': tenantId,
        },
      }
    );

    if (response.status === 200 || response.status === 201) {
      dataGenerated.add(1);
    } else {
      generationErrors.add(1);
    }
  }
}

/**
 * Main execution
 */
export default function () {
  console.log(`Starting production-like data generation...`);
  console.log(`Configuration: ${JSON.stringify(config, null, 2)}`);

  const totalRisks = config.risks;
  const totalAssets = totalRisks * config.assetsPerRisk;
  const totalMitigations = totalRisks * config.mitigationsPerRisk;
  const totalActions = totalMitigations * config.subActionsPerMitigation;
  const totalCustomFields = totalRisks * config.customFieldsPerRisk;

  console.log(`\nExpected Data Volume:`);
  console.log(`  - Risks: ${totalRisks}`);
  console.log(`  - Assets: ${totalAssets}`);
  console.log(`  - Mitigations: ${totalMitigations}`);
  console.log(`  - Sub-Actions: ${totalActions}`);
  console.log(`  - Custom Fields: ${totalCustomFields}`);
  console.log(`  - Total Records: ${totalRisks + totalAssets + totalMitigations + totalActions + totalCustomFields}`);

  // Create risks in batches
  console.log(`\nCreating risks in batches of ${config.batchSize}...`);
  for (let batch = 0; batch < config.risks; batch += config.batchSize) {
    const batchCount = Math.min(config.batchSize, config.risks - batch);
    const riskIds = bulkCreateRisks(batch, batchCount);

    // Add related data for each risk
    for (let i = 0; i < riskIds.length; i++) {
      const riskId = riskIds[i];
      const riskIndex = batch + i;

      createAssetsForRisk(riskId, riskIndex);
      createMitigationsForRisk(riskId, riskIndex);
      createCustomFieldsForRisk(riskId, riskIndex);

      if ((i + 1) % 100 === 0) {
        console.log(`  Processed ${batch + i + 1}/${config.risks} risks...`);
      }
    }

    sleep(0.1); // Small delay between batches
  }

  console.log(`\n✅ Data Generation Complete!`);
  console.log(`\nSummary:`);
  console.log(`  - Records Generated: ${dataGenerated.value}`);
  console.log(`  - Generation Errors: ${generationErrors.value}`);
  console.log(`  - Average Batch Time: ${Math.round(generationTime.value.mean)}ms`);
}
