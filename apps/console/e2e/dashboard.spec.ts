import { expect, test, type BrowserContext, type Page } from '@playwright/test';

const RUN_LIVE_E2E = Boolean(process.env.PLAYWRIGHT_LIVE_E2E);

test.describe('Prism Console Critical Flows', () => {
  test.skip(!RUN_LIVE_E2E, 'Requires live dashboard server (set PLAYWRIGHT_LIVE_E2E=1)');

  test('renders signin page with Turnstile handling', async ({ page }) => {
    await page.goto('/signin');
    await expect(page).toHaveURL(/\/signin/);
    await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible();
  });

  test('unauthenticated user is redirected from protected routes to /signin', async ({ page }) => {
    await page.goto('/gateway-keys');
    await expect(page).toHaveURL(/\/signin/);
    await expect(page.getByText('Sign in to Prism Console')).toBeVisible();
  });

  test.describe.serial('authenticated onboarding, workspace scoping, and security flows', () => {
    const uniqueId = `${Date.now()}-${Math.floor(Math.random() * 10000)}`;
    const email = `e2e-${uniqueId}@roozylabs.test`;
    const password = 'PrismE2ePassword!123';
    const orgName = `E2E Org ${uniqueId}`;
    const primaryWsName = 'Production Environment';
    const secondaryWsName = `Staging ${uniqueId}`;

    let context: BrowserContext | undefined;
    let page: Page;

    test.afterAll(async () => {
      await context?.close();
    });

    test('signup sets HttpOnly auth_token cookie and omits plaintext token from JSON', async ({ browser }) => {
      context = await browser.newContext();
      page = await context.newPage();

      await page.goto('/signup');
      await expect(page.getByText('Create your Prism Account')).toBeVisible();

      await page.getByPlaceholder('John Doe').fill('E2E Tester');
      await page.getByPlaceholder('name@company.com').fill(email);
      await page.getByPlaceholder('At least 8 chars').fill(password);
      await page.getByPlaceholder('Re-enter password').fill(password);

      const [signupResponse] = await Promise.all([
        page.waitForResponse(
          (res) => res.url().includes('/api/auth/signup') && res.request().method() === 'POST'
        ),
        page.getByRole('button', { name: /Create Free Account/i }).click(),
      ]);

      expect(signupResponse.ok()).toBeTruthy();
      const body = (await signupResponse.json()) as Record<string, unknown>;
      expect(body.token).toBeUndefined();
      const dataObj = body.data as Record<string, unknown> | undefined;
      expect(dataObj?.token).toBeUndefined();

      const cookies = await context.cookies();
      const authCookie = cookies.find((c) => c.name === 'auth_token');
      expect(authCookie).toBeDefined();
      expect(authCookie?.httpOnly).toBe(true);

      await expect(page).toHaveURL(/\/onboarding/, { timeout: 15000 });
    });

    test('CSRF protection rejects mutating request from untrusted Origin with 403 CSRF_ORIGIN_REJECTED', async () => {
      const csrfResp = await context!.request.post('/api/gateway-keys', {
        headers: { Origin: 'https://evil.example' },
        data: { name: 'csrf-probe' },
      });
      expect(csrfResp.status()).toBe(403);
      const bodyText = await csrfResp.text();
      expect(bodyText).toContain('CSRF_ORIGIN_REJECTED');
    });

    test('completes onboarding, switches dashboard date range to All Time, and connects SSE', async () => {
      await expect(page.getByText('Configure Organization')).toBeVisible();
      await page.locator('input[name="organizationName"]').fill(orgName);
      await page.locator('input[name="workspaceName"]').fill(primaryWsName);
      await page.getByRole('button', { name: /Continue to Key Setup/i }).click();

      await expect(page.getByText('Create Gateway Key')).toBeVisible();
      await page.locator('input[name="gatewayKeyName"]').fill('Primary E2E Key');
      await page.getByRole('button', { name: /Complete Setup/i }).click();

      await expect(page.getByText('Setup Complete!')).toBeVisible({ timeout: 15000 });

      const sseResponsePromise = page.waitForResponse(
        (res) => res.url().includes('/api/sse') && res.status() === 200,
        { timeout: 15000 }
      );
      await page.getByRole('button', { name: /Go to Control Plane Console/i }).click();

      await expect(page).toHaveURL(/\/$/);
      await expect(page.getByRole('heading', { name: 'API Gateway Overview' })).toBeVisible();

      const sseResp = await sseResponsePromise;
      expect(sseResp.headers()['content-type'] ?? '').toContain('text/event-stream');

      await page.getByRole('combobox').click();
      const statsRefreshPromise = page.waitForResponse(
        (res) => res.url().includes('/api/') && res.url().includes('period=all') && res.ok(),
        { timeout: 10000 }
      ).catch(() => null);
      await page.getByRole('option', { name: 'All Time' }).click();
      await expect(page.getByRole('combobox')).toContainText('All Time');
      await statsRefreshPromise;
    });

    test('creates and switches workspace, refreshes queries, and navigates core routes', async () => {
      await page.goto('/settings/organization');
      await expect(page.getByText('Client Workspaces')).toBeVisible();

      await page.getByPlaceholder('Workspace name').fill(secondaryWsName);
      await page.getByRole('button', { name: /^Add$/i }).click();
      await expect(page.getByText(secondaryWsName).first()).toBeVisible({ timeout: 10000 });

      const wsTrigger = page.getByRole('button', { name: 'Select Workspace' });
      await wsTrigger.click();

      const refreshPromise = page.waitForResponse(
        (res) => res.url().includes('/api/auth/permissions') && res.ok(),
        { timeout: 10000 }
      );
      await page.getByRole('menuitem', { name: new RegExp(secondaryWsName) }).click();
      await refreshPromise;
      await expect(wsTrigger).toContainText(secondaryWsName);

      await page.goto('/approvals');
      await expect(page.getByText('Human Approval Inbox')).toBeVisible();

      await page.goto('/playground');
      await expect(page.getByText('Routing Simulator & Dry-Run Playground')).toBeVisible();

      await page.goto('/sandbox');
      await expect(page.getByText('Developer Web Sandbox')).toBeVisible();

      await page.goto('/gateway-keys');
      await expect(page.getByRole('heading', { name: 'Gateway Keys' })).toBeVisible();
    });

    test('logout clears auth_token session cookie and blocks protected routes', async () => {
      await page.getByRole('button', { name: 'User account menu' }).click();
      await page.getByRole('menuitem', { name: /Sign out/i }).click();

      await expect(page).toHaveURL(/\/signin/, { timeout: 15000 });
      const cookies = await context!.cookies();
      const activeCookie = cookies.find((c) => c.name === 'auth_token' && c.value);
      expect(activeCookie).toBeUndefined();

      await page.goto('/gateway-keys');
      await expect(page).toHaveURL(/\/signin/);
    });
  });
});
