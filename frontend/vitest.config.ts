import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: true,
    // Vitest's default include is **/*.spec.ts, which swept up the Playwright
    // specs under e2e/. Those import @playwright/test, whose test() refuses to
    // run outside the Playwright runner, so three suites failed to collect on
    // every run — noise that trains people to ignore a red result. Playwright
    // owns e2e/ (see playwright.config.ts); Vitest owns src/.
    exclude: ['**/node_modules/**', '**/dist/**', 'e2e/**'],
    // Coverage ratchet (#341, D-022).
    //
    // CI advertised a "70%" threshold for months while the job could not even
    // start: @vitest/coverage-v8 was never a dependency, so `vitest --coverage`
    // died with MISSING DEPENDENCY and the threshold step never ran. The step
    // that "checked" it did nothing but `echo`.
    //
    // The real number, measured 2026-09-02 across all of src/, is 15%. The
    // thresholds below are set AT that floor, not at the 70% nobody was
    // enforcing, because a gate has to be true to be a gate. 70% remains the
    // destination; these numbers are how far the ratchet has been pulled.
    //
    // `all` is on deliberately. Without it, coverage only counts files a test
    // already imports, so adding a wholly untested feature would RAISE the
    // percentage — the opposite of what the gate is for.
    //
    // THESE NUMBERS MAY ONLY EVER GO UP.
    coverage: {
      provider: 'v8',
      all: true,
      include: ['src/**/*.{ts,tsx}'],
      exclude: [
        'src/**/__tests__/**',
        'src/**/*.test.{ts,tsx}',
        'src/**/*.spec.{ts,tsx}',
        'src/test/**',
        'src/types/**',
        'src/**/*.d.ts',
        'src/main.tsx',
        'src/vite-env.d.ts',
      ],
      // json-summary is what the CI threshold step reads; the others are for
      // humans and for Codecov.
      reporter: ['text-summary', 'json-summary', 'json', 'lcov'],
      thresholds: {
        statements: 15,
        branches: 15,
        functions: 11,
        lines: 15,
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
});
