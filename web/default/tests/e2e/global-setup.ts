import { request, type FullConfig } from '@playwright/test'
import { writeFileSync, mkdirSync } from 'node:fs'
import { dirname } from 'node:path'

/**
 * Login once via the API, capture the resulting session cookie + the
 * `New-Api-User` header value (the user id), and persist a
 * Playwright `storageState` so every spec starts already-authenticated.
 *
 * The kittyvibe frontend reads the user id from localStorage under `user`
 * (JSON.stringified profile) and looks up `New-Api-User` from there for
 * every authenticated request — so we also bake that into storageState.
 */
export default async function globalSetup(config: FullConfig) {
  const baseURL = process.env.KITTYVIBE_BASE_URL ?? 'http://localhost:3000'
  const username = process.env.KITTYVIBE_ADMIN_USER ?? 'admin'
  const password = process.env.KITTYVIBE_ADMIN_PASS ?? 'admin123456'
  const storageStatePath =
    process.env.KITTYVIBE_STORAGE_STATE ?? './.auth/admin.json'

  const ctx = await request.newContext({ baseURL, ignoreHTTPSErrors: true })
  const res = await ctx.post('/api/user/login', {
    data: { username, password },
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok()) {
    throw new Error(`login failed: HTTP ${res.status()} ${await res.text()}`)
  }
  const body = await res.json()
  const user = body?.data
  if (!user?.id) throw new Error(`login response missing data.id: ${JSON.stringify(body)}`)

  const state = await ctx.storageState()
  state.origins.push({
    origin: baseURL,
    localStorage: [
      { name: 'user', value: JSON.stringify(user) },
      { name: 'uid', value: String(user.id) },
      { name: 'role', value: String(user.role) },
      { name: 'token', value: '' },
    ],
  })
  mkdirSync(dirname(storageStatePath), { recursive: true })
  writeFileSync(storageStatePath, JSON.stringify(state, null, 2))
  // Side-channel for tests that need to issue API requests outside the
  // browser context (page.request). Read in fixtures.ts.
  process.env.KITTYVIBE_USER_ID = String(user.id)
  writeFileSync(`${dirname(storageStatePath)}/userid.txt`, String(user.id))
  console.log(
    `[global-setup] saved auth for ${user.username} (id=${user.id}, role=${user.role}) → ${storageStatePath}`,
  )
  await ctx.dispose()
}
