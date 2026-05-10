import { beforeEach, describe, expect, test } from 'bun:test'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'
import {
  formatLocalCurrencyAmount,
  formatPaymentGatewayAmount,
} from './currency'

function setCurrency(
  quotaDisplayType: CurrencyDisplayType,
  usdExchangeRate: number,
  extra: Partial<typeof DEFAULT_CURRENCY_CONFIG> = {}
) {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType,
      usdExchangeRate,
      ...extra,
    },
  })
}

const moneyOpts = { digitsLarge: 2, digitsSmall: 2, abbreviate: false }

describe('formatPaymentGatewayAmount', () => {
  beforeEach(() => {
    setCurrency('USD', 1)
  })

  test('USD display with non-1 USDExchangeRate uses ¥ (Epay gateway, the user-reported bug)', () => {
    setCurrency('USD', 7.3)
    expect(formatPaymentGatewayAmount(73, moneyOpts)).toBe('¥73')
  })

  test('USD display with rate=7 still uses ¥', () => {
    setCurrency('USD', 7)
    expect(formatPaymentGatewayAmount(70, moneyOpts)).toBe('¥70')
  })

  test('USD display with rate=1 falls back to $ (no implicit gateway)', () => {
    setCurrency('USD', 1)
    expect(formatPaymentGatewayAmount(10, moneyOpts)).toBe('$10')
  })

  test('CNY display path is unchanged', () => {
    setCurrency('CNY', 7)
    expect(formatPaymentGatewayAmount(73, moneyOpts)).toBe('¥73')
  })

  test('CUSTOM display uses the operator-configured symbol', () => {
    setCurrency('CUSTOM', 7, {
      customCurrencySymbol: '€',
      customCurrencyExchangeRate: 0.9,
    })
    expect(formatPaymentGatewayAmount(50, moneyOpts)).toBe('€50')
  })

  test('null / NaN inputs return placeholder', () => {
    expect(formatPaymentGatewayAmount(null)).toBe('-')
    expect(formatPaymentGatewayAmount(Number.NaN)).toBe('-')
  })
})

describe('formatLocalCurrencyAmount (legacy path)', () => {
  test('still uses $ under USD display — proves the fix lives in formatPaymentGatewayAmount, not in this function', () => {
    setCurrency('USD', 7.3)
    expect(formatLocalCurrencyAmount(73, moneyOpts)).toBe('$73')
  })
})
