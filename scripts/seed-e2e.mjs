#!/usr/bin/env node
// Deterministic E2E seed. Idempotent: ensures the admin tenant holds AT LEAST the
// target dataset (12 risks, 2 frameworks, 5 vulnerabilities) + one incident, and
// records IDs for the parameterised routes. Runs standalone (`npm run seed:e2e`)
// and from Playwright global-setup.
//
// It seeds the always-present admin tenant (admin@opendefender.io, created by
// handlers.SeedAdminUser on every boot) rather than a throwaway registration,
// because that admin is a real tenant admin with permissions ["*"] and a resolved
// org — the registration path returns no token and second-member creation has no
// working API path (documented as OR-BUG in docs/UX_AUDIT_2026-07.md).
//
// The analyst/auditor personas are ATTEMPTED (POST /users + business-role assign);
// if the platform has no working "add a business-role member to a tenant" path,
// they are recorded as unusable with a reason and their journeys become test.fixme.

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SEED_IDS_FILE = path.resolve(HERE, '../tests/e2e/.seed-ids.json');
const DATASET_FILE = path.resolve(HERE, '../dev/fixtures/e2e-dataset.json');

const API_URL = process.env.E2E_API_URL || process.env.API_URL || 'http://localhost:8080/api/v1';
const ADMIN = {
  email: process.env.E2E_ADMIN_EMAIL || 'admin@opendefender.io',
  password: process.env.E2E_ADMIN_PASSWORD || 'admin123',
};

const dataset = JSON.parse(fs.readFileSync(DATASET_FILE, 'utf8'));

let TOKEN = '';
let ADMIN_LOGIN = null; // full login result, reused by global-setup to avoid a 2nd login
async function api(method, endpoint, body) {
  const res = await fetch(`${API_URL}${endpoint}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(TOKEN ? { Authorization: `Bearer ${TOKEN}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  let json;
  try {
    json = text ? JSON.parse(text) : null;
  } catch {
    json = text;
  }
  return { status: res.status, ok: res.ok, body: json };
}

/** Tolerant list unwrap — the API is inconsistent (array | {data} | {risks} | …). */
function asArray(body) {
  if (Array.isArray(body)) return body;
  if (!body || typeof body !== 'object') return [];
  for (const k of ['data', 'items', 'risks', 'frameworks', 'vulnerabilities', 'results', 'incidents']) {
    if (Array.isArray(body[k])) return body[k];
  }
  return [];
}
/** Tolerant id extraction from a create response. */
function idOf(body) {
  if (!body || typeof body !== 'object') return undefined;
  if (typeof body.id === 'string') return body.id;
  for (const k of ['data', 'risk', 'framework', 'incident', 'vulnerability']) {
    if (body[k] && typeof body[k].id === 'string') return body[k].id;
  }
  return undefined;
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function login() {
  // The auth endpoint is rate-limited; back off and retry on 429 so a burst of
  // test logins doesn't wedge the seed.
  let res;
  for (let attempt = 1; attempt <= 10; attempt++) {
    res = await api('POST', '/auth/login', { email: ADMIN.email, password: ADMIN.password });
    if (res.ok) break;
    if (res.status === 429) {
      console.warn(`[seed] login rate-limited (429), backing off ${attempt * 6}s (${attempt}/10)`);
      await sleep(attempt * 6000);
      continue;
    }
    break;
  }
  if (!res.ok) throw new Error(`admin login failed: ${res.status} ${JSON.stringify(res.body)}`);
  TOKEN = res.body?.token_pair?.access_token;
  ADMIN_LOGIN = res.body;
  if (!TOKEN) throw new Error('admin login returned no access_token');
  console.log(`[seed] logged in as ${ADMIN.email}`);
}

async function ensureRisks() {
  const list = asArray((await api('GET', '/risks?limit=200')).body);
  const existingTitles = new Set(list.map((r) => r?.title));
  let riskId = list[0]?.id;
  let created = 0;
  for (const r of dataset.risks) {
    if (existingTitles.has(r.title)) continue;
    const res = await api('POST', '/risks', {
      title: r.title,
      description: r.description,
      impact: r.impact,
      probability: r.probability,
      tags: r.tags ?? [],
      frameworks: r.frameworks ?? [],
    });
    if (res.ok) {
      created++;
      riskId = riskId || idOf(res.body);
    } else {
      console.warn(`[seed] risk "${r.title}" -> ${res.status} ${JSON.stringify(res.body)}`);
    }
  }
  // Re-read to capture a stable riskId and the true count.
  const after = asArray((await api('GET', '/risks?limit=200')).body);
  riskId = riskId || after[0]?.id;
  console.log(`[seed] risks: ${after.length} present (+${created} created)`);
  return { riskId, count: after.length };
}

async function ensureFrameworks() {
  let frameworks = asArray((await api('GET', '/compliance/frameworks')).body);
  const catalogs = asArray((await api('GET', '/compliance/catalogs')).body)
    .filter((c) => c && c.available !== false && (c.key || c.id))
    .map((c) => c.key || c.id);

  let created = 0;
  for (const spec of dataset.frameworks) {
    const exists = frameworks.some((f) => f?.name === spec.name);
    if (exists) continue;
    const res = await api('POST', '/compliance/frameworks', {
      name: spec.name,
      version: spec.version || '1.0',
      description: spec.description || '',
    });
    const fid = idOf(res.body);
    if (res.ok && fid) {
      created++;
      // Best-effort catalog import so the framework isn't empty.
      const key = spec.catalog_key && catalogs.includes(spec.catalog_key)
        ? spec.catalog_key
        : catalogs[created - 1] || catalogs[0];
      if (key) {
        const imp = await api('POST', `/compliance/frameworks/${fid}/import-catalog`, { catalog_key: key });
        console.log(`[seed] framework "${spec.name}" import ${key} -> ${imp.status}`);
      }
    } else {
      console.warn(`[seed] framework "${spec.name}" -> ${res.status} ${JSON.stringify(res.body)}`);
    }
  }
  frameworks = asArray((await api('GET', '/compliance/frameworks')).body);
  const frameworkId = frameworks[0]?.id;
  console.log(`[seed] frameworks: ${frameworks.length} present (+${created} created)`);
  return { frameworkId, count: frameworks.length };
}

async function ensureVulnerabilities() {
  const before = asArray((await api('GET', '/vulnerabilities?limit=200')).body);
  const existing = new Set(before.map((v) => v?.cve_id || v?.title));
  const toIngest = dataset.vulnerabilities.filter((v) => !existing.has(v.cve) && !existing.has(v.title));
  if (toIngest.length) {
    const res = await api('POST', '/vulnerabilities/ingest', {
      source: 'manual',
      findings: toIngest.map((v) => ({
        title: v.title,
        description: v.description,
        cve: v.cve,
        cvss: v.cvss,
        severity: v.severity,
        kev: v.kev ?? false,
        exploit_available: v.exploit_available ?? false,
        host: v.host,
      })),
    });
    console.log(`[seed] vuln ingest ${toIngest.length} findings -> ${res.status}`);
  }
  const after = asArray((await api('GET', '/vulnerabilities?limit=200')).body);
  console.log(`[seed] vulnerabilities: ${after.length} present`);
  return { count: after.length };
}

async function ensureIncident() {
  const list = asArray((await api('GET', '/incidents')).body);
  let incidentId = list[0]?.id;
  if (!incidentId && dataset.incident) {
    const res = await api('POST', '/incidents', dataset.incident);
    incidentId = idOf(res.body);
    console.log(`[seed] incident create -> ${res.status}`);
  }
  return { incidentId };
}

// Attempt to provision analyst + auditor as business-role members of the tenant.
// Recorded honestly whether it works or not — the platform may lack the path.
async function tryPersonas() {
  const personas = {
    admin: { usable: true, email: ADMIN.email },
    analyst: { usable: false, reason: 'not attempted' },
    auditor: { usable: false, reason: 'not attempted' },
  };
  for (const [key, spec] of Object.entries(dataset.personas || {})) {
    const created = await api('POST', '/users', {
      email: spec.email,
      username: spec.username,
      full_name: spec.full_name,
      password: spec.password,
      role: 'user',
      department: 'E2E',
    });
    if (!created.ok && created.status !== 409) {
      personas[key] = { usable: false, reason: `POST /users -> ${created.status}`, email: spec.email };
      continue;
    }
    // Can this user actually authenticate into a tenant?
    const loginRes = await api('POST', '/auth/login', { email: spec.email, password: spec.password });
    if (!loginRes.ok || !loginRes.body?.token_pair?.access_token) {
      personas[key] = {
        usable: false,
        reason: `created but cannot log in (${loginRes.status}) — no tenant membership resolved`,
        email: spec.email,
      };
      continue;
    }
    personas[key] = { usable: true, email: spec.email };
  }
  return personas;
}

async function main() {
  console.log(`[seed] API_URL=${API_URL}`);
  await login();
  const risks = await ensureRisks();
  const frameworks = await ensureFrameworks();
  const vulns = await ensureVulnerabilities();
  const incident = await ensureIncident();
  const personas = await tryPersonas();

  const out = {
    generatedAt: new Date().toISOString(),
    frameworkId: frameworks.frameworkId,
    riskId: risks.riskId,
    incidentId: incident.incidentId,
    personas,
    counts: { risks: risks.count, frameworks: frameworks.count, vulnerabilities: vulns.count },
    // Reused by global-setup to build the admin storageState WITHOUT a 2nd login
    // (keeps the rate-limited auth endpoint from wedging repeat runs).
    adminLogin: ADMIN_LOGIN,
  };
  fs.mkdirSync(path.dirname(SEED_IDS_FILE), { recursive: true });
  fs.writeFileSync(SEED_IDS_FILE, JSON.stringify(out, null, 2));
  console.log(`[seed] wrote ${SEED_IDS_FILE}`);
  console.log(`[seed] personas: ${Object.entries(personas).map(([k, v]) => `${k}=${v.usable}`).join(' ')}`);
}

main().catch((err) => {
  console.error('[seed] FAILED:', err.message);
  process.exit(1);
});
