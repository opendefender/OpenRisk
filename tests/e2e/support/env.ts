// Shared E2E environment resolution. Single source for URLs, credentials and the
// on-disk locations of minted auth states + seed output.

import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));

/** Frontend base URL — Playwright `baseURL`; relative `page.goto('/x')` resolves here. */
export const FRONTEND_URL = process.env.E2E_BASE_URL || 'http://localhost:5173';

/** Frontend origin, used for the storageState localStorage entry. */
export const FRONTEND_ORIGIN = new URL(FRONTEND_URL).origin;

/** Backend API base — the frontend hardcodes this; keep them aligned. */
export const API_URL =
  process.env.E2E_API_URL || process.env.API_URL || 'http://localhost:8080/api/v1';

/** Deterministic admin: seeded on every backend boot (handlers.SeedAdminUser). */
export const ADMIN = {
  email: process.env.E2E_ADMIN_EMAIL || 'admin@opendefender.io',
  password: process.env.E2E_ADMIN_PASSWORD || 'admin123',
};

/** tests/e2e directory. */
export const E2E_DIR = path.resolve(here, '..');

/** Where global-setup writes one storageState JSON per usable persona. */
export const AUTH_DIR = path.join(E2E_DIR, '.auth');

/** Where the seed script records IDs (framework/risk/incident) + persona usability. */
export const SEED_IDS_FILE = path.join(E2E_DIR, '.seed-ids.json');

/** storageState path for a persona (admin | analyst | auditor). */
export const authFileFor = (persona: string) => path.join(AUTH_DIR, `${persona}.json`);
