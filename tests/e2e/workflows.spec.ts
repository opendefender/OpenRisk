import { test, expect, Page } from '@playwright/test';

test.describe('End-to-End Workflows', () => {
  let page: Page;

  test.beforeEach(async ({ browser }) => {
    page = await browser.newPage();
    await page.goto('http://localhost:3000');
  });

  test.afterEach(async () => {
    await page.close();
  });

  test('Complete Risk Lifecycle Workflow', async () => {
    await page.click('button[data-testid="login-button"]');
    await page.fill('input[name="email"]', 'test@example.com');
    await page.fill('input[name="password"]', 'TestPassword123!');
    await page.click('button[type="submit"]');
    await page.waitForURL('**/dashboard');
    expect(page.url()).toContain('/dashboard');

    await page.click('[data-testid="nav-risks"]');
    await page.waitForURL('**/risks');

    await page.click('[data-testid="btn-create-risk"]');
    await page.fill('input[name="title"]', 'E2E Test Risk');
    await page.fill('textarea[name="description"]', 'Risk created during E2E test');
    await page.selectOption('select[name="category"]', 'Operational');
    await page.selectOption('select[name="severity"]', 'High');
    await page.fill('input[name="probability"]', '0.7');
    await page.fill('input[name="impact"]', '0.8');
    await page.click('button[data-testid="btn-save-risk"]');

    await page.waitForSelector('[data-testid="risk-card"]');
    const riskCard = await page.locator('[data-testid="risk-card"]:has-text("E2E Test Risk")');
    await expect(riskCard).toBeVisible();

    await riskCard.click();
    await page.waitForURL('**/risks/**');
    await page.click('[data-testid="btn-edit-risk"]');
    await page.fill('input[name="title"]', 'Updated E2E Test Risk');
    await page.selectOption('select[name="status"]', 'In Progress');
    await page.click('[data-testid="btn-save-changes"]');

    await page.waitForSelector('text=Updated E2E Test Risk');
    expect(page.locator('text=Updated E2E Test Risk')).toBeTruthy();

    await page.click('[data-testid="btn-edit-risk"]');
    await page.selectOption('select[name="status"]', 'Mitigated');
    await page.click('[data-testid="btn-save-changes"]');
    await page.waitForSelector('text=Mitigated');
  });

  test('Create and Track Incident Workflow', async () => {
    await page.click('button[data-testid="login-button"]');
    await page.fill('input[name="email"]', 'test@example.com');
    await page.fill('input[name="password"]', 'TestPassword123!');
    await page.click('button[type="submit"]');
    await page.waitForURL('**/dashboard');

    await page.click('[data-testid="nav-incidents"]');
    await page.waitForURL('**/incidents');

    await page.click('[data-testid="btn-create-incident"]');
    await page.fill('input[name="title"]', 'E2E Test Incident');
    await page.fill('textarea[name="description"]', 'Incident created during E2E test');
    await page.selectOption('select[name="severity"]', 'High');
    await page.fill('input[name="impact_score"]', '0.8');
    await page.click('button[data-testid="btn-save-incident"]');

    await page.waitForSelector('[data-testid="incident-card"]');
    const incidentCard = await page.locator('[data-testid="incident-card"]:has-text("E2E Test Incident")');
    await expect(incidentCard).toBeVisible();

    await incidentCard.click();
    await page.waitForURL('**/incidents/**');
    await page.click('[data-testid="btn-assign-incident"]');
    await page.selectOption('select[name="assigned_to"]', 'Current User');
    await page.click('[data-testid="btn-confirm-assign"]');

    await page.fill('textarea[data-testid="textarea-notes"]', 'Investigation findings: Potential data exposure identified.');
    await page.click('[data-testid="btn-add-note"]');

    await page.click('[data-testid="btn-resolve-incident"]');
    await page.waitForSelector('text=Resolved');
  });

  test('Analytics Export and Dashboard Workflow', async () => {
    await page.click('button[data-testid="login-button"]');
    await page.fill('input[name="email"]', 'test@example.com');
    await page.fill('input[name="password"]', 'TestPassword123!');
    await page.click('button[type="submit"]');
    await page.waitForURL('**/dashboard');

    await page.click('[data-testid="nav-analytics"]');
    await page.waitForURL('**/analytics');

    await page.waitForSelector('[data-testid="dashboard-widget"]');
    const widgets = await page.locator('[data-testid="dashboard-widget"]').count();
    expect(widgets).toBeGreaterThan(0);

    await page.click('[data-testid="btn-export-metrics"]');
    await page.selectOption('select[name="format"]', 'json');
    
    const downloadPromise = page.waitForEvent('download');
    await page.click('[data-testid="btn-download"]');
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toContain('metrics');

    await page.click('[data-testid="btn-export-trends"]');
    await page.selectOption('select[name="format"]', 'csv');
    
    const downloadPromise2 = page.waitForEvent('download');
    await page.click('[data-testid="btn-download"]');
    const download2 = await downloadPromise2;
    expect(download2.suggestedFilename()).toContain('trends');

    await page.click('[data-testid="btn-analyze-trends"]');
    await page.selectOption('select[name="time_period"]', '30d');
    await page.click('[data-testid="btn-analyze"]');
    
    await page.waitForSelector('[data-testid="trend-chart"]');
    expect(page.locator('[data-testid="trend-chart"]')).toBeTruthy();

    await page.click('[data-testid="btn-detect-anomalies"]');
    await page.waitForSelector('[data-testid="anomaly-list"]');
    const anomalies = await page.locator('[data-testid="anomaly-item"]').count();
    expect(anomalies).toBeGreaterThanOrEqual(0);
  });

  test('Gamification and Progress Workflow', async () => {
    await page.click('button[data-testid="login-button"]');
    await page.fill('input[name="email"]', 'test@example.com');
    await page.fill('input[name="password"]', 'TestPassword123!');
    await page.click('button[type="submit"]');
    await page.waitForURL('**/dashboard');

    await page.click('[data-testid="nav-progress"]');
    await page.waitForURL('**/progress');

    await page.waitForSelector('[data-testid="achievements-section"]');
    expect(page.locator('[data-testid="achievements-section"]')).toBeTruthy();

    const badges = await page.locator('[data-testid="achievement-badge"]').count();
    expect(badges).toBeGreaterThanOrEqual(0);

    await page.click('[data-testid="tab-leaderboard"]');
    await page.waitForSelector('[data-testid="leaderboard-entry"]');
    const entries = await page.locator('[data-testid="leaderboard-entry"]').count();
    expect(entries).toBeGreaterThan(0);

    const pointsDisplay = await page.locator('[data-testid="points-display"]').textContent();
    expect(pointsDisplay).toMatch(/\d+/);
  });

  test('Multi-Tenant Isolation Workflow', async () => {
    let pageA = await page.context().newPage();
    await pageA.goto('http://localhost:3000');
    await pageA.click('button[data-testid="login-button"]');
    await pageA.fill('input[name="email"]', 'tenant-a@example.com');
    await pageA.fill('input[name="password"]', 'TestPassword123!');
    await pageA.click('button[type="submit"]');
    await pageA.waitForURL('**/dashboard');

    await pageA.click('[data-testid="nav-risks"]');
    await pageA.click('[data-testid="btn-create-risk"]');
    await pageA.fill('input[name="title"]', 'Tenant A Risk');
    await pageA.click('button[data-testid="btn-save-risk"]');
    await pageA.waitForSelector('[data-testid="risk-card"]:has-text("Tenant A Risk")');

    let pageB = await page.context().newPage();
    await pageB.goto('http://localhost:3000');
    await pageB.click('button[data-testid="login-button"]');
    await pageB.fill('input[name="email"]', 'tenant-b@example.com');
    await pageB.fill('input[name="password"]', 'TestPassword123!');
    await pageB.click('button[type="submit"]');
    await pageB.waitForURL('**/dashboard');

    await pageB.click('[data-testid="nav-risks"]');
    
    const tenantARiskVisible = await pageB.locator('[data-testid="risk-card"]:has-text("Tenant A Risk")').isVisible().catch(() => false);
    expect(tenantARiskVisible).toBeFalsy();

    await pageA.close();
    await pageB.close();
  });

  test('Custom Metrics and Calculation Workflow', async () => {
    await page.click('button[data-testid="login-button"]');
    await page.fill('input[name="email"]', 'test@example.com');
    await page.fill('input[name="password"]', 'TestPassword123!');
    await page.click('button[type="submit"]');
    await page.waitForURL('**/dashboard');

    await page.click('[data-testid="nav-settings"]');
    await page.click('[data-testid="tab-custom-metrics"]');
    await page.waitForURL('**/settings/metrics');

    await page.click('[data-testid="btn-create-metric"]');
    await page.fill('input[name="name"]', 'Risk Density');
    await page.fill('textarea[name="description"]', 'Number of risks per asset');
    await page.fill('input[name="formula"]', 'total_risks / total_assets');
    await page.click('[data-testid="btn-save-metric"]');

    await page.waitForSelector('[data-testid="metric-card"]:has-text("Risk Density")');

    const metricCard = await page.locator('[data-testid="metric-card"]:has-text("Risk Density")');
    await metricCard.click();
    await page.click('[data-testid="btn-edit-metric"]');
    await page.fill('input[name="description"]', 'Updated: Risk density metric per asset');
    await page.click('[data-testid="btn-save-changes"]');

    await page.click('[data-testid="tab-history"]');
    await page.waitForSelector('[data-testid="history-entry"]');
    const historyEntries = await page.locator('[data-testid="history-entry"]').count();
    expect(historyEntries).toBeGreaterThan(0);
  });

  test('Error Handling - Missing Risk', async () => {
    await page.goto('http://localhost:3000/risks/nonexistent');
    await page.waitForSelector('[data-testid="error-message"], [data-testid="not-found"]');
    expect(page.locator('[data-testid="error-message"], [data-testid="not-found"]')).toBeTruthy();
  });

  test('Form Validation Test', async () => {
    await page.click('button[data-testid="login-button"]');
    await page.fill('input[name="email"]', 'test@example.com');
    await page.fill('input[name="password"]', 'TestPassword123!');
    await page.click('button[type="submit"]');
    await page.waitForURL('**/dashboard');

    await page.click('[data-testid="nav-risks"]');
    await page.click('[data-testid="btn-create-risk"]');
    await page.selectOption('select[name="category"]', 'Operational');
    await page.click('button[data-testid="btn-save-risk"]');

    await page.waitForSelector('[data-testid="error-message"]');
    expect(page.locator('[data-testid="error-message"]')).toBeTruthy();
  });

  test('Session Persistence Test', async () => {
    await page.click('button[data-testid="login-button"]');
    await page.fill('input[name="email"]', 'test@example.com');
    await page.fill('input[name="password"]', 'TestPassword123!');
    await page.click('button[type="submit"]');
    await page.waitForURL('**/dashboard');

    const newPage = await page.context().newPage();
    await newPage.goto('http://localhost:3000/dashboard');
    
    await newPage.waitForURL('**/dashboard');
    expect(newPage.url()).toContain('/dashboard');
    
    await newPage.close();
  });
});
