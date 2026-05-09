// Localized model description helper.
//
// `Pricing.Description` is a single string field on the backend, but
// operators want to provide copy in multiple languages. To stay
// backward-compatible with plain-string descriptions already in the DB,
// we treat any string that parses as a JSON object of the shape
// `{ "<lang>": "<copy>" }` as a locale map; everything else is taken
// verbatim.
//
// Example operator-facing values:
//   "Plain marketing copy."                          → returned as-is
//   '{"en":"...","zh":"..."}'                        → resolved by locale
//   '{"zh":"...","en":"...","ja":"..."}'             → resolved by locale
//
// Resolution order: requested language → "en" → "zh" → first non-empty
// value → empty string. This means a partial map (e.g. zh only) still
// renders something for an English user instead of falling through to
// "No description available".

const PROBABLY_JSON = /^\s*\{[\s\S]*\}\s*$/

export function localizedDescription(
  raw: string | undefined | null,
  language: string | undefined
): string {
  const value = (raw ?? '').trim()
  if (!value) return ''
  if (!PROBABLY_JSON.test(value)) return value

  let parsed: unknown
  try {
    parsed = JSON.parse(value)
  } catch {
    return value
  }
  if (
    !parsed ||
    typeof parsed !== 'object' ||
    Array.isArray(parsed)
  ) {
    return value
  }

  const map = parsed as Record<string, unknown>
  const lang = (language || 'en').toLowerCase()
  // Try the exact language first, then the language prefix (zh-CN → zh),
  // then English, then Chinese, then anything non-empty. Each candidate
  // must be a non-empty string to count.
  const candidates = [
    lang,
    lang.split('-')[0],
    lang.split('_')[0],
    'en',
    'zh',
  ]
  for (const k of candidates) {
    const v = map[k]
    if (typeof v === 'string' && v.trim()) return v
  }
  for (const v of Object.values(map)) {
    if (typeof v === 'string' && v.trim()) return v
  }
  return ''
}
