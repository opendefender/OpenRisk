// OpenRisk E2E — Playwright config (repo root).
//
// Topology (locked by the app, see docs/UX_AUDIT_2026-07.md §Testability):
//   • The frontend HARDCODES its API base to http://localhost:8080/api/v1
//     (frontend/src/lib/api.ts) and the backend CORS allowlist defaults to
//     http://localhost:5173 — so the ONLY topology the browser can use is
//     backend :8080 + frontend dev :5173. VITE_API_URL is dead code.
//   • Auth lives in localStorage (auth_token / auth_user), not cookies, so the
//     storageState fixture seeds localStorage (see global-setup.ts).
//
// webServer (local): brings up backend + frontend, reusing them if already
// running (`make dev`). In CI we set E2E_NO_WEBSERVER=1 and start the servers in
// explicit workflow steps for tighter control over env/DB/Redis.

import { defineConfig, devices } from '@playwright/test';

const FRONTEND_URL = process.env.E2E_BASE_URL || 'http://localhost:5173';
const API_URL = process.env.E2E_API_URL || process.env.API_URL || 'http://localhost:8080/api/v1';

// Cross-browser (firefox/webkit) is reserved for the nightly job — a 25-min PR CI
// is a CI nobody uses. PR CI runs chromium + Mobile Chrome only.
const CROSS_BROWSER = !!process.env.E2E_CROSS_BROWSER;

export default defineConfig({
  testDir: './tests/e2e',
  testMatch: '**/*.spec.ts',
  outputDir: './tests/e2e/.artifacts/test-results',
  globalSetup: './tests/e2e/global-setup.ts',

  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  // A slow cold route chunk + a populated dev DB — give real budgets.
  timeout: 45_000,
  expect: { timeout: 10_000 },

  reporter: [
    ['html', { outputFolder: './tests/e2e/.artifacts/playwright-report', open: 'never' }],
    ['json', { outputFile: './tests/e2e/.artifacts/results.json' }],
    ['junit', { outputFile: './tests/e2e/.artifacts/junit.xml' }],
    ['list'],
  ],

  use: {
    baseURL: FRONTEND_URL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },

  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    // UX-27: responsive 360px→4K without loss of function — Mobile Chrome runs on PR.
    { name: 'Mobile Chrome', use: { ...devices['Pixel 5'] } },
    ...(CROSS_BROWSER
      ? [
          { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
          { name: 'webkit', use: { ...devices['Desktop Safari'] } },
        ]
      : []),
  ],

  webServer: process.env.E2E_NO_WEBSERVER
    ? undefined
    : [
        {
          // Reused when `make dev` already runs it. Health lives under /api/v1.
          command: 'cd backend && go run ./cmd/server',
          url: `${API_URL}/health`,
          timeout: 120_000,
          reuseExistingServer: !process.env.CI,
          stdout: 'ignore',
          stderr: 'pipe',
        },
        {
          command: 'npm --prefix frontend run dev -- --port 5173 --strictPort',
          url: FRONTEND_URL,
          timeout: 120_000,
          reuseExistingServer: !process.env.CI,
          stdout: 'ignore',
          stderr: 'pipe',
        },
      ],
});
