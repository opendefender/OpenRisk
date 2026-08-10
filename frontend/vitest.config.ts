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
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
});
