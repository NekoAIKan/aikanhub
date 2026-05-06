import { ROLE } from './roles'

export type BillingVisibilityMode =
  | 'credits'
  | 'summary'
  | 'detailed'
  | 'internal'

export const BILLING_VISIBILITY_MODES: BillingVisibilityMode[] = [
  'credits',
  'summary',
  'detailed',
  'internal',
]

export const DEFAULT_BILLING_VISIBILITY_MODE: BillingVisibilityMode = 'credits'

export const DEFAULT_BILLING_GROUP_MODES: Record<
  string,
  BillingVisibilityMode
> = {
  default: 'credits',
  trial: 'credits',
  beta: 'summary',
  invited: 'detailed',
  b2b: 'detailed',
  enterprise: 'detailed',
}

export type BillingVisibilitySettings = {
  defaultMode: BillingVisibilityMode
  groupModes: Record<string, BillingVisibilityMode>
}

export function normalizeBillingVisibilityMode(
  mode: unknown,
  fallback: BillingVisibilityMode = DEFAULT_BILLING_VISIBILITY_MODE
): BillingVisibilityMode {
  return typeof mode === 'string' &&
    BILLING_VISIBILITY_MODES.includes(mode as BillingVisibilityMode)
    ? (mode as BillingVisibilityMode)
    : fallback
}

export function parseBillingVisibilityGroupModes(
  value: unknown,
  fallback: Record<string, BillingVisibilityMode> = DEFAULT_BILLING_GROUP_MODES
): Record<string, BillingVisibilityMode> {
  const raw =
    typeof value === 'string'
      ? (() => {
          try {
            return JSON.parse(value) as unknown
          } catch {
            return null
          }
        })()
      : value

  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
    return fallback
  }

  const parsed: Record<string, BillingVisibilityMode> = {}
  for (const [group, mode] of Object.entries(raw)) {
    const key = group.trim().toLowerCase()
    if (!key) continue
    parsed[key] = normalizeBillingVisibilityMode(mode)
  }

  return Object.keys(parsed).length > 0 ? parsed : fallback
}

export function parseBillingVisibilitySettings(params: {
  defaultMode?: unknown
  groupModes?: unknown
}): BillingVisibilitySettings {
  return {
    defaultMode: normalizeBillingVisibilityMode(params.defaultMode),
    groupModes: parseBillingVisibilityGroupModes(params.groupModes),
  }
}

export function billingVisibilitySettingsFromOptionRows(
  options: Array<{ key: string; value: string }> | undefined
): BillingVisibilitySettings {
  const byKey = new Map(options?.map((option) => [option.key, option.value]))
  return parseBillingVisibilitySettings({
    defaultMode: byKey.get('billing_visibility_setting.default_mode'),
    groupModes: byKey.get('billing_visibility_setting.group_modes'),
  })
}

export function resolveBillingVisibilityMode(params: {
  role?: number | null
  group?: string | null
  defaultMode?: BillingVisibilityMode
  groupModes?: Record<string, BillingVisibilityMode>
  explicitMode?: unknown
}): BillingVisibilityMode {
  if ((params.role ?? 0) >= ROLE.ADMIN) return 'internal'

  if (params.explicitMode) {
    return userBillingVisibilityMode(
      normalizeBillingVisibilityMode(params.explicitMode)
    )
  }

  const defaultMode = params.defaultMode ?? DEFAULT_BILLING_VISIBILITY_MODE
  const groupModes = params.groupModes ?? DEFAULT_BILLING_GROUP_MODES
  const group = params.group?.trim().toLowerCase() || 'default'

  return userBillingVisibilityMode(
    normalizeBillingVisibilityMode(groupModes[group], defaultMode)
  )
}

function userBillingVisibilityMode(
  mode: BillingVisibilityMode
): BillingVisibilityMode {
  return mode === 'internal' ? 'detailed' : mode
}

export function getBillingVisibilityLabelKey(
  mode: BillingVisibilityMode
): string {
  switch (mode) {
    case 'summary':
      return 'Summary billing'
    case 'detailed':
      return 'Detailed billing'
    case 'internal':
      return 'Internal billing'
    case 'credits':
    default:
      return 'Credits-only billing'
  }
}

export function getBillingVisibilityShortLabelKey(
  mode: BillingVisibilityMode
): string {
  switch (mode) {
    case 'summary':
      return 'Summary'
    case 'detailed':
      return 'Detailed'
    case 'internal':
      return 'Internal'
    case 'credits':
    default:
      return 'Credits'
  }
}

export function getBillingVisibilityDescriptionKey(
  mode: BillingVisibilityMode
): string {
  switch (mode) {
    case 'summary':
      return 'Shows credits plus high-level billing factors.'
    case 'detailed':
      return 'Shows auditable formula inputs without upstream margin.'
    case 'internal':
      return 'Admin-only view with upstream cost, retail charge, and margin.'
    case 'credits':
    default:
      return 'Shows packaged credits without formula internals.'
  }
}
