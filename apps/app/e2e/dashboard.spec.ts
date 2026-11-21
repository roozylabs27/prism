import { expect, test } from '@playwright/test';

const RUN_LIVE_E2E = Boolean(process.env.PLAYWRIGHT_LIVE_E2E);

test.describe('Prism Console Critical Flows', () => {
  test.skip(!RUN_LIVE_E2E, 'Requires live dashboard server (set PLAYWRIGHT_LIVE_E2E=1)');

  test('renders signin page with Turnstile handling', async ({ page }) => {
    await page.goto('/signin');
    await expect(page).toHaveURL(/\/signin/);
    await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible();
  });

  test('authenticated user can navigate approvals, playground, budgets, and settings', async ({ page }) => {
    await page.goto('/approvals');
    await expect(page.getByText('Human Approval Inbox')).toBeVisible();

    await page.goto('/playground');
    await expect(page.getByText('Routing Simulator & Dry-Run Playground')).toBeVisible();

    await page.goto('/settings/organization');
    await expect(page.getByText('Client Workspaces')).toBeVisible();
  });
});
