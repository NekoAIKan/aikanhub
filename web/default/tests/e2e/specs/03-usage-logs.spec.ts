/**
 * Round-trip: submit one video task per shape via the gateway (which routes
 * to the Volcano mock), poll until succeeded, then assert the resulting row
 * surfaces correctly on the Usage Logs page with the right action label.
 */
import { test, expect, type APIRequestContext } from '../fixtures'
import type { Route } from '@playwright/test'

const TOKEN = process.env.KITTYVIBE_GATEWAY_TOKEN
const FIXTURE_TASK_ID = 'task-pr27-720p-54450'

async function submitAndPoll(req: APIRequestContext, body: Record<string, unknown>) {
  const sub = await req.post('/api/v3/contents/generations/tasks', {
    headers: {
      Authorization: `Bearer ${TOKEN}`,
      'Content-Type': 'application/json',
    },
    data: body,
  })
  expect(sub.status()).toBe(200)
  const { id } = await sub.json()
  // Poll until succeeded.
  for (let i = 0; i < 30; i++) {
    const r = await req.get(`/api/v3/contents/generations/tasks/${id}`, {
      headers: { Authorization: `Bearer ${TOKEN}` },
    })
    const j = await r.json()
    if (j.status === 'succeeded' || j.status === 'failed') return { id, ...j }
    await new Promise((r) => setTimeout(r, 1500))
  }
  throw new Error(`task ${id} did not finish`)
}

test.describe('PR #27 — usage logs surface video tasks correctly', () => {
  test('ordinary user sees credits-only billing breakdown', async ({ page }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem(
        'user',
        JSON.stringify({
          id: 101,
          username: 'ordinary-user',
          role: 1,
          group: 'default',
        }),
      )
    })
    const fulfillStats = async (route: Route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: { quota: 54450, rpm: 0, tpm: 0 },
        }),
      })
    }
    await page.route('**/api/log/stat?**', fulfillStats)
    await page.route('**/api/log/self/stat?**', fulfillStats)
    const fulfillLogs = async (route: Route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            total: 1,
            page: 1,
            page_size: 100,
            items: [
              {
                id: 9001,
                user_id: 101,
                created_at: Math.floor(Date.now() / 1000),
                type: 2,
                content: '720p video fixture',
                username: 'ordinary-user',
                token_name: 'fixture-key',
                model_name: 'doubao-seedance-2-0-fast-260128',
                quota: 54450,
                prompt_tokens: 0,
                completion_tokens: 0,
                use_time: 0,
                is_stream: false,
                channel: 0,
                group: 'default',
                billing_breakdown: {
                  mode: 'credits',
                  charged_credits: 10.89,
                  charged_quota: 54450,
                  label: 'Credits',
                },
              },
            ],
          },
        }),
      })
    }
    await page.route('**/api/log?**', fulfillLogs)
    await page.route('**/api/log/self?**', fulfillLogs)

    await page.goto('/usage-logs/common')
    await expect(page.getByText(/10\.89 Credits/i).first()).toBeVisible()
    await expect(page.getByText(/54,450 quota/i)).toHaveCount(0)
    await expect(page.getByText(/Unit Price/i)).toHaveCount(0)
    await expect(page.getByText(/Gross Margin/i)).toHaveCount(0)
  })

  test('b2b user sees detailed video formula evidence', async ({ page }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem(
        'user',
        JSON.stringify({
          id: 202,
          username: 'b2b-user',
          role: 1,
          group: 'b2b',
          billing_visibility_mode: 'detailed',
        }),
      )
    })
    const fulfillDetailedTasks = async (route: Route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            total: 1,
            page: 1,
            page_size: 100,
            items: [
              {
                id: 9101,
                user_id: 202,
                username: 'b2b-user',
                platform: 'doubao',
                task_id: FIXTURE_TASK_ID,
                action: 'TEXT_GENERATE',
                channel_id: 0,
                submit_time: Math.floor(Date.now() / 1000),
                finish_time: Math.floor(Date.now() / 1000) + 60,
                status: 'SUCCESS',
                billing_breakdown: {
                  mode: 'detailed',
                  label: 'Credits',
                  charged_credits: 10.89,
                  charged_quota: 54450,
                  model: 'doubao-seedance-2-0-fast-260128',
                  tokens: 108900,
                  resolution: '720p',
                  duration_seconds: 5,
                  unit_price_per_million: 1,
                  group_ratio: 1,
                  precharged_quota: 67500,
                  actual_quota: 54450,
                  adjustment_quota: -13050,
                  pricing_version: 'video_formula:v1:fixturehash',
                },
              },
            ],
          },
        }),
      })
    }
    await page.route('**/api/task?**', fulfillDetailedTasks)
    await page.route('**/api/task/self?**', fulfillDetailedTasks)

    await page.goto('/usage-logs/task')
    await expect(page.getByText(FIXTURE_TASK_ID).first()).toBeVisible()
    await expect(page.getByText(/10\.89 Credits/i).first()).toBeVisible()
    await expect(page.getByText(/108,900/).first()).toBeVisible()
    await expect(page.getByText(/720p.*5s/i).first()).toBeVisible()
    await expect(page.getByText(/Unit Price/i).first()).toBeVisible()
    await expect(page.getByText(/Pricing Version/i).first()).toBeVisible()
  })

  test('admin sees internal margin and pricing audit fields', async ({ page }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem(
        'user',
        JSON.stringify({
          id: 1,
          username: 'root',
          role: 100,
          group: 'default',
          billing_visibility_mode: 'internal',
        }),
      )
    })
    await page.route('**/api/task?**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            total: 1,
            page: 1,
            page_size: 100,
            items: [
              {
                id: 9201,
                user_id: 202,
                username: 'b2b-user',
                platform: 'doubao',
                task_id: FIXTURE_TASK_ID,
                action: 'TEXT_GENERATE',
                channel_id: 7,
                submit_time: Math.floor(Date.now() / 1000),
                finish_time: Math.floor(Date.now() / 1000) + 60,
                status: 'SUCCESS',
                billing_breakdown: {
                  mode: 'internal',
                  label: 'Credits',
                  charged_credits: 10.89,
                  charged_quota: 54450,
                  retail_charge_quota: 54450,
                  upstream_cost_quota: 42000,
                  gross_margin_quota: 12450,
                  gross_margin_percent: 22.86,
                  pricing_version: 'video_formula:v1:fixturehash',
                  pricing_hash: 'fixturehash',
                },
              },
            ],
          },
        }),
      })
    })

    await page.goto('/usage-logs/task')
    await expect(page.getByText(FIXTURE_TASK_ID).first()).toBeVisible()
    await expect(page.getByText(/Retail Charge/i).first()).toBeVisible()
    await expect(page.getByText(/Upstream Cost/i).first()).toBeVisible()
    await expect(page.getByText(/Gross Margin/i).first()).toBeVisible()
    await expect(page.getByText(/22\.86%/).first()).toBeVisible()
    await expect(page.getByText(/fixturehash/i).first()).toBeVisible()
  })

  test('text → textGenerate row in Usage Logs', async ({ page, apiRequest }) => {
    test.skip(!TOKEN, 'set KITTYVIBE_GATEWAY_TOKEN to a sk- token to run this spec')
    const submitted = await submitAndPoll(apiRequest, {
      model: 'doubao-seedance-2-0-fast-260128',
      content: [{ type: 'text', text: 'a tiny ginger cat' }],
      duration: 5,
      ratio: '16:9',
      resolution: '720p',
    })
    expect(submitted.status).toBe('succeeded')

    await page.goto('/')
    await page.goto('/usage-logs/task')
    await page.waitForLoadState('networkidle')
    // The full task id renders in a cell; partial-match keeps the spec
    // resilient to truncation in the UI.
    await expect(page.getByText(submitted.id.slice(0, 12))).toBeVisible({ timeout: 10_000 })
  })

  test('first+last → firstTailGenerate row in Usage Logs', async ({ page, apiRequest }) => {
    test.skip(!TOKEN, 'set KITTYVIBE_GATEWAY_TOKEN to a sk- token to run this spec')
    const submitted = await submitAndPoll(apiRequest, {
      model: 'doubao-seedance-2-0-fast-260128',
      content: [
        { type: 'text', text: 'transition' },
        {
          type: 'image_url',
          image_url: { url: 'https://picsum.photos/seed/aikanhub-first/800/600' },
          role: 'first_frame',
        },
        {
          type: 'image_url',
          image_url: { url: 'https://picsum.photos/seed/aikanhub-last/800/600' },
          role: 'last_frame',
        },
      ],
      duration: 5,
      ratio: '16:9',
    })
    expect(submitted.status).toBe('succeeded')

    await page.goto('/')
    await page.goto('/usage-logs/task')
    await page.waitForLoadState('networkidle')
    await expect(page.getByText(submitted.id.slice(0, 12))).toBeVisible({ timeout: 10_000 })
    // Bonus: the action column should display the label registered in
    // features/usage-logs/constants.ts → 'First/Last Frame to Video' (en) or
    // '首尾生视频' (zh).
    await expect(
      page.getByText(/(First\/Last Frame to Video|首尾生视频)/i).first(),
    ).toBeVisible()
  })
})
