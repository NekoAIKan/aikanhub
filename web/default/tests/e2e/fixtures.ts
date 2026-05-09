/**
 * Custom test fixture that overrides Playwright's default `request` so it
 * carries the `New-Api-User: <id>` header on every API call. The id is
 * persisted by global-setup.ts to .auth/userid.txt.
 */
import { test as base, expect, type APIRequestContext } from '@playwright/test'
import { readFileSync } from 'node:fs'

let cachedUserId: string | null = null
function userId(): string {
  if (cachedUserId !== null) return cachedUserId
  const path = process.env.KITTYVIBE_USERID_FILE ?? './.auth/userid.txt'
  cachedUserId = readFileSync(path, 'utf8').trim()
  return cachedUserId
}

export const test = base.extend<{ apiRequest: APIRequestContext }>({
  apiRequest: async ({ playwright, baseURL, storageState }, use) => {
    const ctx = await playwright.request.newContext({
      baseURL,
      storageState: storageState as string,
      extraHTTPHeaders: { 'New-Api-User': userId() },
    })
    await use(ctx)
    await ctx.dispose()
  },
})

export { expect }
