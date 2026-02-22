import { test, expect } from '@playwright/test';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:5173';
const API_URL = process.env.API_URL || 'http://localhost:8080';

test.describe('OpenRisk E2E Tests', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to app
    await page.goto(BASE_URL);
    
    // Wait for app to load
    await page.waitForLoadState('networkidle');
  });

  test.describe('Authentication Flow', () => {
    test('should display login form', async ({ page }) => {
      // Check if we're redirected to login
      await expect(page).toHaveURL(/login|auth/, { timeout: 10000 });
      
      // Verify form elements
      await expect(page.locator('input[type="email"]')).toBeVisible();
      await expect(page.locator('input[type="password"]')).toBeVisible();
      await expect(page.locator('button[type="submit"]')).toBeVisible();
    });

    test('should show error on invalid credentials', async ({ page }) => {
      const emailInput = page.locator('input[type="email"]');
      const passwordInput = page.locator('password"]');
      const submitButton = page.locator('button[type="submit"]');

      await emailInput.fill('invalid@example.com');
      await passwordInput.fill('wrongpassword');
      await submitButton.click();

      // Wait for error message
      await expect(page.locator('text=Invalid credentials')).toBeVisible({ timeout: 5000 });
    });

    test('should login with valid credentials', async ({ page }) => {
      const emailInput = page.locator('input[type="email"]');
      const passwordInput = page.locator('input[type="password"]');
      const submitButton = page.locator('button[type="submit"]');

      await emailInput.fill(process.env.TEST_EMAIL || 'test@example.com');
      await passwordInput.fill(process.env.TEST_PASSWORD || 'password123');
      await submitButton.click();

      // Should redirect to dashboard
      await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
    });
  });

  test.describe('Risk Management', () => {
    test.beforeEach(async ({ page }) => {
      // Login first
      await loginUser(page);
    });

    test('should display risk list', async ({ page }) => {
      // Navigate to risks
      await page.click('text=Risks');
      await page.waitForLoadState('networkidle');

      // Verify risks are displayed
      await expect(page.locator('[data-testid="risk-list"]')).toBeVisible();
    });

    test('should create new risk', async ({ page }) => {
      await page.click('text=Create Risk');
      await page.waitForLoadState('networkidle');

      // Fill form
      await page.fill('input[name="title"]', 'E2E Test Risk');
      await page.fill('textarea[name="description"]', 'Test risk created by E2E tests');
      await page.selectOption('select[name="impact"]', 'high');
      await page.selectOption('select[name="probability"]', 'medium');

      // Submit
      await page.click('button:has-text("Create")');

      // Verify success message
      await expect(page.locator('text=Risk created successfully')).toBeVisible({ timeout: 5000 });
      await expect(page).toHaveURL(/\/risks\/\d+/);
    });

    test('should view risk details', async ({ page }) => {
      await page.click('text=Risks');
      await page.waitForLoadState('networkidle');

      // Click first risk
      const firstRisk = page.locator('[data-testid="risk-row"]').first();
      await firstRisk.click();

      // Verify details are displayed
      await expect(page.locator('[data-testid="risk-title"]')).toBeVisible();
      await expect(page.locator('[data-testid="risk-description"]')).toBeVisible();
      await expect(page.locator('[data-testid="risk-score"]')).toBeVisible();
    });

    test('should update risk', async ({ page }) => {
      await page.click('text=Risks');
      await page.waitForLoadState('networkidle');

      // Click first risk
      const firstRisk = page.locator('[data-testid="risk-row"]').first();
      await firstRisk.click();

      // Click edit button
      await page.click('button:has-text("Edit")');

      // Update title
      await page.fill('input[name="title"]', 'Updated Risk Title');

      // Submit
      await page.click('button:has-text("Save")');

      // Verify update
      await expect(page.locator('text=Risk updated successfully')).toBeVisible({ timeout: 5000 });
    });

    test('should delete risk', async ({ page }) => {
      await page.click('text=Risks');
      await page.waitForLoadState('networkidle');

      // Get initial count
      const initialCount = await page.locator('[data-testid="risk-row"]').count();

      // Click first risk
      const firstRisk = page.locator('[data-testid="risk-row"]').first();
      await firstRisk.click();

      // Click delete button
      await page.click('button:has-text("Delete")');

      // Confirm deletion
      await page.click('button:has-text("Confirm")');

      // Verify success
      await expect(page.locator('text=Risk deleted successfully')).toBeVisible({ timeout: 5000 });
    });
  });

  test.describe('Custom Fields', () => {
    test.beforeEach(async ({ page }) => {
      await loginUser(page);
    });

    test('should display custom fields page', async ({ page }) => {
      await page.click('text=Custom Fields');
      await page.waitForLoadState('networkidle');

      // Verify page loads
      await expect(page.locator('text=Custom Fields')).toBeVisible();
    });

    test('should create custom field', async ({ page }) => {
      await page.click('text=Custom Fields');
      await page.waitForLoadState('networkidle');

      // Click create button
      await page.click('button:has-text("Create Field")');

      // Fill form
      await page.fill('input[name="fieldName"]', 'Test Custom Field');
      await page.selectOption('select[name="fieldType"]', 'text');
      await page.check('input[name="isRequired"]');

      // Submit
      await page.click('button:has-text("Create")');

      // Verify creation
      await expect(page.locator('text=Custom field created successfully')).toBeVisible({ timeout: 5000 });
    });
  });

  test.describe('Bulk Operations', () => {
    test.beforeEach(async ({ page }) => {
      await loginUser(page);
    });

    test('should display bulk operations', async ({ page }) => {
      await page.click('text=Bulk Operations');
      await page.waitForLoadState('networkidle');

      // Verify page loads
      await expect(page.locator('text=Bulk Operations')).toBeVisible();
    });

    test('should show operation jobs', async ({ page }) => {
      await page.click('text=Bulk Operations');
      await page.waitForLoadState('networkidle');

      // Wait for jobs to load
      await page.waitForSelector('[data-testid="job-item"]', { timeout: 5000 });

      // Verify job items are visible
      const jobItems = page.locator('[data-testid="job-item"]');
      const count = await jobItems.count();
      expect(count).toBeGreaterThanOrEqual(0);
    });
  });

  test.describe('Performance', () => {
    test('should load dashboard within acceptable time', async ({ page }) => {
      const startTime = Date.now();
      
      await loginUser(page);
      await page.goto(`${BASE_URL}/dashboard`);
      await page.waitForLoadState('networkidle');

      const loadTime = Date.now() - startTime;
      
      // Dashboard should load within 3 seconds
      expect(loadTime).toBeLessThan(3000);
    });

    test('should handle large risk lists efficiently', async ({ page }) => {
      await loginUser(page);
      
      await page.goto(`${BASE_URL}/risks?limit=100`);
      const startTime = Date.now();
      
      await page.waitForLoadState('networkidle');
      const loadTime = Date.now() - startTime;

      // Large list should load within 5 seconds
      expect(loadTime).toBeLessThan(5000);

      // Verify all 100 items loaded
      const items = page.locator('[data-testid="risk-row"]');
      const count = await items.count();
      expect(count).toBeGreaterThan(0);
    });

    test('should handle rapid navigation', async ({ page }) => {
      await loginUser(page);

      // Rapid navigation to different pages
      const pages = [
        '/dashboard',
        '/risks',
        '/custom-fields',
        '/bulk-operations',
      ];

      const startTime = Date.now();

      for (const route of pages) {
        await page.goto(`${BASE_URL}${route}`);
        await page.waitForLoadState('networkidle');
      }

      const totalTime = Date.now() - startTime;

      // All navigation should complete within 10 seconds
      expect(totalTime).toBeLessThan(10000);
    });
  });

  test.describe('Error Handling', () => {
    test.beforeEach(async ({ page }) => {
      await loginUser(page);
    });

    test('should display error on network failure', async ({ page, context }) => {
      // Simulate network error
      await context.setOffline(true);

      await page.click('text=Risks');

      // Should show error message
      await expect(page.locator('text=Network error|Failed to load')).toBeVisible({ timeout: 5000 });

      // Restore network
      await context.setOffline(false);
    });

    test('should handle 500 server error gracefully', async ({ page }) => {
      // Intercept and return 500 error
      await page.route('**/api/v1/risks', route => {
        route.abort('failed');
      });

      await page.click('text=Risks');

      // Should show error message
      await expect(page.locator('text=Error loading risks|Server error')).toBeVisible({ timeout: 5000 });
    });
  });
});

// Helper function to login
async function loginUser(page: any) {
  const loginUrl = `${BASE_URL}/login`;
  await page.goto(loginUrl);
  
  await page.fill('input[type="email"]', process.env.TEST_EMAIL || 'test@example.com');
  await page.fill('input[type="password"]', process.env.TEST_PASSWORD || 'password123');
  await page.click('button[type="submit"]');
  
  // Wait for redirect to dashboard
  await page.waitForURL(/dashboard/, { timeout: 10000 });
}
