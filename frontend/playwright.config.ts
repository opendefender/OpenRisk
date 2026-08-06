// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration, currently serving the overlay visual-regression
 * suite (e2e/visual/overlays.spec.ts).
 *
 * The suite renders overlays against a static harness rather than the running
 * app: it is testing that a themed surface resolves its tokens correctly in
 * both themes, which needs no backend, no session and no seeded data. Coupling
 * it to a live API would make a theme regression indistinguishable from a
 * flaky login.
 */
export default defineConfig({
  testDir: './e2e',
  // Screenshot comparison is meaningless if two tests race on the same page.
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',

  use: {
    baseURL: 'http://localhost:5173',
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
