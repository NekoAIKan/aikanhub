import { test, expect } from '@playwright/test'

test('admin session reaches System Settings → General', async ({ page }) => {
  await page.goto('/system-settings/general')
  await expect(page).toHaveURL(/system-settings\/general/)
  // Sanity: page should not redirect back to /login.
  await expect(page).not.toHaveURL(/\/login$/)
})
