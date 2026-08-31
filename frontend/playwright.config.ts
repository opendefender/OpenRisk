// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration, serving two suites:
 *
 *   e2e/visual/overlays.spec.ts  renders overlays against a static harness. It
 *     tests that a themed surface resolves its tokens correctly in both themes,
 *     which needs no backend, no session and no seeded data — coupling it to a
 *     live API would make a theme regression indistinguishable from a flaky
 *     login.
 *
 *   e2e/empty-states.spec.ts     drives the real app against a real backend,
 *     because the thing under test is what a brand-new tenant sees. It creates
 *     its own tenant over the API and skips itself when the API is unreachable.
 */
export default defineConfig({
  testDir: './e2e',
  // Screenshot comparison is meaningless if two tests race on the same page.
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',

  use: {
    /*
     * Overridable, because a stale dev server is a real hazard for this suite:
     * Tailwind generates utilities from the config it loaded at startup, so a
     * server left running from before a config change serves a stylesheet
     * missing the new utilities — and the snapshots then record unstyled
     * markup while passing. Pointing the suite at a server you just started is
     * the cheap way to be sure.
     */
    baseURL: process.env.OPENRISK_BASE_URL ?? 'http://localhost:5173',
    // Deterministic rendering: animations mid-flight are the classic source of
    // one-pixel diffs that erode trust in a visual suite until it gets ignored.
    trace: 'retain-on-failure',
  },

  expect: {
    toHaveScreenshot: {
      // Font hinting differs marginally between machines; a small tolerance
      // keeps genuine colour regressions visible without daily false alarms.
      maxDiffPixelRatio: 0.01,
      animations: 'disabled',
      caret: 'hide',
    },
  },

  // Serves the app for the empty-states suite. Reuses an already-running `npm
  // run dev` so the usual local loop is untouched; the visual suite ignores it.
  webServer: {
    command: 'npm run dev',
    url: process.env.OPENRISK_BASE_URL ?? 'http://localhost:5173',
    reuseExistingServer: true,
    timeout: 120_000,
  },

  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1280, height: 800 },
        deviceScaleFactor: 1,
      },
    },
  ],
});
