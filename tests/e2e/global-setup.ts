// Playwright global setup:
//   1. wait for backend health (/api/v1/health) + frontend
//   2. run the deterministic seed (scripts/seed-e2e.mjs) as a subprocess
//   3. mint one storageState per USABLE persona into tests/e2e/.auth/*.json
// Tests then attach a storageState instead of logging in through the UI
// (auth.login.spec.ts is the only UI-login test).

import { request as pwRequest, type FullConfig } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { API_URL, FRONTEND_URL, ADMIN, AUTH_DIR, SEED_IDS_FILE, authFileFor } from './support/env';
import { apiLogin, buildStorageState } from './support/auth';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SEED_SCRIPT = path.resolve(HERE, '../../scripts/seed-e2e.mjs');

async function waitFor(url: string, label: string, tries = 60) {
  for (let i = 0; i < tries; i++) {
    try {
      const res = await fetch(url);
      if (res.ok || res.status < 500) {
        console.log(`[global-setup] ${label} ready (${res.status})`);
        return;
      }
    } catch {
      /* not up yet */
    }
    await new Promise((r) => setTimeout(r, 1000));
  }
  throw new Error(`[global-setup] ${label} not ready at ${url} after ${tries}s`);
}

export default async function globalSetup(_config: FullConfig) {
  await waitFor(`${API_URL}/health`, 'backend');
  await waitFor(FRONTEND_URL, 'frontend');

  console.log('[global-setup] seeding…');
  execFileSync('node', [SEED_SCRIPT], { stdio: 'inherit', env: process.env });

  const seed = JSON.parse(fs.readFileSync(SEED_IDS_FILE, 'utf8'));
  fs.mkdirSync(AUTH_DIR, { recursive: true });

  const dataset = JSON.parse(
    fs.readFileSync(path.resolve(HERE, '../../dev/fixtures/e2e-dataset.json'), 'utf8'),
  );

  const ctx = await pwRequest.newContext();
  const loginWithRetry = async (email: string, password: string) => {
    for (let attempt = 1; attempt <= 8; attempt++) {
      try {
        return await apiLogin(ctx, email, password);
      } catch (e) {
        if (/429/.test((e as Error).message) && attempt < 8) {
          console.warn(`[global-setup] login 429, backing off ${attempt * 6}s`);
          await new Promise((r) => setTimeout(r, attempt * 6000));
          continue;
        }
        throw e;
      }
    }
    throw new Error('login retries exhausted');
  };
  try {
    // Admin is always usable. Reuse the login the seed already performed (avoids a
    // 2nd hit on the rate-limited auth endpoint); fall back to a fresh login.
    const adminLogin = seed.adminLogin ?? (await loginWithRetry(ADMIN.email, ADMIN.password));
    fs.writeFileSync(authFileFor('admin'), JSON.stringify(buildStorageState(adminLogin), null, 2));
    console.log('[global-setup] minted admin storageState');

    // Personas only if the seed confirmed they can authenticate.
    for (const key of ['analyst', 'auditor']) {
      const p = seed.personas?.[key];
      const creds = dataset.personas?.[key];
      if (p?.usable && creds) {
        try {
          const login = await apiLogin(ctx, creds.email, creds.password);
          fs.writeFileSync(authFileFor(key), JSON.stringify(buildStorageState(login), null, 2));
          console.log(`[global-setup] minted ${key} storageState`);
        } catch (e) {
          console.warn(`[global-setup] ${key} login failed, skipping: ${(e as Error).message}`);
        }
      } else {
        console.warn(`[global-setup] ${key} not usable (${p?.reason ?? 'no seed record'}) — its tests will fixme`);
      }
    }
  } finally {
    await ctx.dispose();
  }
}
