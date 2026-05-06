/**
 * Admin UI E2E for PR #27 — "feat: add video billing and growth admin controls".
 *
 * Each section navigates to its dedicated route under
 *   /system-settings/general/<section-id>
 * exercises a representative interaction, and asserts the change either
 * persisted server-side (Onboarding, Credits) or surfaced through the API
 * (Invite Campaigns, Config Bundles).
 */
import type { Route } from '@playwright/test'
import { test, expect } from '../fixtures'

const SECTION = {
  credits: '/system-settings/general/credits-profit',
  onboarding: '/system-settings/general/onboarding',
  invites: '/system-settings/general/invite-campaigns',
  bundles: '/system-settings/general/config-bundles',
} as const

const SAVE_RX = /save\s*changes|save|保存/i
const CREATE_RX = /^(create|创建)$/i
const SUMMARY_BILLING_OPTIONS = [
  { key: 'billing_visibility_setting.default_mode', value: 'credits' },
  {
    key: 'billing_visibility_setting.group_modes',
    value: JSON.stringify({
      default: 'credits',
      invited: 'detailed',
      b2b: 'summary',
      enterprise: 'detailed',
    }),
  },
]

async function fulfillSummaryBillingOptions(route: Route) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      success: true,
      data: SUMMARY_BILLING_OPTIONS,
    }),
  })
}

test.describe('PR #27 — admin-side video billing & growth', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
  })

  test('Onboarding: change new-user quota and persist via /api/option', async ({ page, apiRequest }) => {
    await page.goto(SECTION.onboarding)
    await expect(page.getByRole('heading', { name: /Onboarding Quota Policy/i })).toBeVisible()

    // The label-based locator is resilient to react-hook-form rename churn.
    const input = page.getByLabel(/Policy new user quota/i)
    await expect(input).toBeVisible()
    const stamp = String(500_000 + Math.floor(Math.random() * 1_000))
    await input.fill(stamp)
    await page.getByRole('button', { name: SAVE_RX }).first().click()

    // Wait for the unsaved-changes alert to disappear (form clean again).
    await expect(page.getByText(/unsaved changes/i)).toHaveCount(0, { timeout: 10_000 })

    const res = await apiRequest.get('/api/option/')
    const body = await res.json()
    const opts = body?.data ?? body?.items ?? []
    const row = (opts as Array<{ key: string; value: string }>).find(
      (o) => o.key === 'onboarding_setting.new_user_quota',
    )
    expect(row).toBeDefined()
    expect(String(row?.value)).toBe(stamp)
  })

  test('Credits/Profit: configure default markup percent', async ({ page, apiRequest }) => {
    await page.goto(SECTION.credits)
    // The page heading might be the section's "Credits & Profit" or a sub-section title.
    await expect(
      page.getByRole('heading', { name: /Credits|Profit/i }).first(),
    ).toBeVisible()
    await expect(page.getByText(/Preview only/i)).toHaveCount(0)
    await expect(page.getByRole('heading', { name: /User credits display/i })).toBeVisible()
    await expect(page.getByRole('heading', { name: /Retail pricing & profit/i })).toBeVisible()
    await expect(page.getByRole('heading', { name: /Transparency group policy/i })).toBeVisible()
    await expect(page.getByRole('heading', { name: /Video billing profiles/i })).toBeVisible()

    const preview = page.getByTestId('fixture-pricing-preview')
    await expect(preview).toBeVisible()
    await expect(preview.getByText(/720p/i)).toBeVisible()
    await expect(preview.getByText(/1080p/i)).toBeVisible()
    await expect(preview.getByText('Edit / multimodal', { exact: true })).toBeVisible()
    await expect(preview.getByText(/Extend/i)).toBeVisible()
    await expect(preview.getByText(/10\.89 Credits/i)).toBeVisible()
    await expect(preview.getByText(/54,450/)).toBeVisible()
    await expect(preview.getByText(/Gross margin/i)).toBeVisible()

    await expect(page.getByTestId('billing-group-mode-default').getByRole('combobox')).toContainText('Credits')
    await expect(page.getByTestId('billing-group-mode-invited').getByRole('combobox')).toContainText('Detailed')
    await expect(page.getByTestId('billing-group-mode-b2b').getByRole('combobox')).toContainText('Detailed')

    // The Default markup percent field — labeled by react-hook-form FormLabel.
    const markup = page.getByLabel(/Default markup percent/i).first()
    await expect(markup).toBeVisible()
    const stamp = String(15 + Math.floor(Math.random() * 10))
    await markup.fill(stamp)
    await page.getByRole('button', { name: SAVE_RX }).first().click()
    await expect(page.getByText(/unsaved changes/i)).toHaveCount(0, { timeout: 10_000 })

    const res = await apiRequest.get('/api/option/')
    const body = await res.json()
    const opts = body?.data ?? body?.items ?? []
    const row = (opts as Array<{ key: string; value: string }>).find(
      (o) => o.key === 'profit_setting.default_markup_percent',
    )
    expect(row).toBeDefined()
    expect(String(row?.value)).toBe(stamp)
  })

  test('Invite Campaigns: create + list + delete via UI', async ({ page, apiRequest }) => {
    await page.route('**/api/option**', fulfillSummaryBillingOptions)
    await page.goto(SECTION.invites)
    await expect(page.getByRole('heading', { name: /Campaign Invites/i })).toBeVisible()
    await expect(page.getByText(/invited, b2b, or enterprise/i)).toBeVisible()

    const code = `e2e-${Date.now().toString(36)}`
    await page.getByLabel(/Campaign code/i).fill(code)
    await page.getByLabel(/Campaign name/i).fill('E2E Beta Invite')
    await page.getByLabel('Group', { exact: true }).fill('b2b')
    await page.getByLabel('Quota', { exact: true }).fill('100000')
    await page.getByLabel(/Max uses/i).fill('5')

    await page.getByRole('button', { name: CREATE_RX }).click()
    // Backend up-cases the code on persist, so match case-insensitively.
    await expect(
      page.getByRole('row', { name: new RegExp(code, 'i') }).first(),
    ).toBeVisible({ timeout: 6_000 })
    await expect(
      page.getByRole('row', { name: new RegExp(`${code}.*Summary`, 'i') }).first(),
    ).toBeVisible()

    const res = await apiRequest.get('/api/invite_campaign/')
    const body = await res.json()
    const items: Array<{ id: number; code: string }> =
      body?.data?.items ?? body?.data ?? []
    const created = items.find((it) => it.code.toLowerCase() === code.toLowerCase())
    expect(created).toBeDefined()

    if (created) {
      await apiRequest.delete(`/api/invite_campaign/${created.id}`)
    }
  })

  test('Config Bundles: export → preview → confirm round-trip', async ({ page, apiRequest }) => {
    await page.goto(SECTION.bundles)
    await expect(page.getByRole('heading', { name: /Config Bundles/i })).toBeVisible()

    await expect(
      page.getByRole('button', { name: /(export|导出)/i }).first(),
    ).toBeVisible()

    const exp = await apiRequest.get('/api/option/bundle/export')
    expect(exp.status()).toBe(200)
    const exported = await exp.json()
    const bundle = exported?.data ?? exported

    const preview = await apiRequest.post('/api/option/bundle/import/preview', {
      data: { bundle },
      headers: { 'Content-Type': 'application/json' },
    })
    expect(preview.status()).toBe(200)
    const previewBody = await preview.json()
    expect(previewBody?.success).toBeTruthy()
  })

  test('Users: group shows resolved billing visibility mode', async ({ page }) => {
    await page.route('**/api/option**', fulfillSummaryBillingOptions)
    const b2bUser = {
      id: 42,
      username: 'e2e-b2b-user',
      display_name: 'E2E B2B User',
      password: '',
      quota: 54450,
      used_quota: 122512,
      request_count: 3,
      group: 'b2b',
      status: 1,
      role: 1,
      remark: '',
    }

    await page.route('**/api/user/?**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: { items: [b2bUser], total: 1, page: 1, page_size: 10 },
        }),
      })
    })
    await page.route('**/api/group/', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: ['default', 'invited', 'b2b', 'enterprise'],
        }),
      })
    })
    await page.route('**/api/user/42', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ success: true, data: b2bUser }),
      })
    })

    await page.goto('/users')
    const row = page.getByRole('row', { name: /e2e-b2b-user/i })
    await expect(row).toBeVisible()
    await expect(row.getByText(/Summary/i)).toBeVisible()

    await row.getByRole('button', { name: /Open menu/i }).click()
    await page.getByRole('menuitem', { name: /Edit/i }).click()
    await expect(page.getByText(/Billing visibility/i)).toBeVisible()
    await expect(page.getByText(/Summary billing/i)).toBeVisible()
  })
})
