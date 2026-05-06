import { useTranslation } from 'react-i18next'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatLogQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { BillingBreakdown } from '../types'

type BillingBreakdownViewProps = {
  breakdown?: BillingBreakdown | null
  fallbackQuota?: number
  compact?: boolean
}

function formatNumber(value: number | undefined, digits = 2): string {
  if (value == null || !Number.isFinite(value)) return '-'
  return value.toLocaleString(undefined, {
    maximumFractionDigits: digits,
    minimumFractionDigits: 0,
  })
}

function formatCredits(value: number | undefined, label?: string): string {
  if (value == null || !Number.isFinite(value)) return '-'
  return `${formatNumber(value, 2)} ${label || 'Credits'}`
}

function formatQuotaEvidence(quota: number | undefined): string {
  if (quota == null || !Number.isFinite(quota)) return '-'
  return `${formatLogQuota(quota)} / ${formatNumber(quota, 0)} quota`
}

function formatUnitPrice(value: number | undefined): string {
  if (value == null || !Number.isFinite(value)) return '-'
  return `${formatBillingCurrencyFromUSD(value, {
    digitsLarge: 4,
    digitsSmall: 6,
    abbreviate: false,
    useCreditDisplay: false,
  })}/M`
}

function chargedDisplay(
  breakdown: BillingBreakdown | undefined | null,
  fallbackQuota: number | undefined
): string {
  if (breakdown?.charged_credits != null) {
    return formatCredits(breakdown.charged_credits, breakdown.label)
  }

  const quota = breakdown?.charged_quota ?? fallbackQuota
  return quota == null ? '-' : formatLogQuota(quota)
}

function billingRows(breakdown: BillingBreakdown) {
  const rows: Array<{ label: string; value: string; internal?: boolean }> = []
  const mode = breakdown.mode ?? 'credits'
  const isDetailed =
    mode === 'summary' || mode === 'detailed' || mode === 'internal'
  const isInternal = mode === 'internal'

  if (!isDetailed) return rows

  if (breakdown.tokens != null) {
    rows.push({
      label: 'Tokens',
      value: formatNumber(breakdown.tokens, 0),
    })
  }

  const mediaParts = [
    breakdown.resolution,
    breakdown.duration_seconds != null
      ? `${formatNumber(breakdown.duration_seconds, 0)}s`
      : null,
  ].filter(Boolean)
  if (mediaParts.length > 0) {
    rows.push({ label: 'Resolution / duration', value: mediaParts.join(' / ') })
  }

  if (mode === 'detailed' || mode === 'internal') {
    rows.push({
      label: 'Unit Price',
      value: formatUnitPrice(breakdown.unit_price_per_million),
    })
    if (breakdown.group_ratio != null) {
      rows.push({
        label: 'Group Ratio',
        value: `${formatNumber(breakdown.group_ratio, 4)}x`,
      })
    }
    rows.push({
      label: 'Precharge',
      value: formatQuotaEvidence(breakdown.precharged_quota),
    })
    rows.push({
      label: 'Actual',
      value: formatQuotaEvidence(breakdown.actual_quota),
    })
    rows.push({
      label: 'Adjustment',
      value: formatQuotaEvidence(breakdown.adjustment_quota),
    })
    if (breakdown.pricing_version) {
      rows.push({
        label: 'Pricing Version',
        value: breakdown.pricing_version,
      })
    }
  }

  if (isInternal) {
    rows.push({
      label: 'Retail Charge',
      value: formatQuotaEvidence(
        breakdown.retail_charge_quota ?? breakdown.charged_quota
      ),
      internal: true,
    })
    rows.push({
      label: 'Upstream Cost',
      value: formatQuotaEvidence(breakdown.upstream_cost_quota),
      internal: true,
    })
    rows.push({
      label: 'Gross Margin',
      value: `${formatQuotaEvidence(breakdown.gross_margin_quota)} (${formatNumber(
        breakdown.gross_margin_percent,
        2
      )}%)`,
      internal: true,
    })
    if (breakdown.pricing_hash) {
      rows.push({
        label: 'Pricing Hash',
        value: breakdown.pricing_hash,
        internal: true,
      })
    }
  }

  return rows
}

export function BillingBreakdownView({
  breakdown,
  fallbackQuota,
  compact = false,
}: BillingBreakdownViewProps) {
  const { t } = useTranslation()
  if (!breakdown && fallbackQuota == null) return null

  const rows = breakdown ? billingRows(breakdown) : []
  const mode = breakdown?.mode ?? 'credits'

  return (
    <div className='flex max-w-[300px] flex-col gap-1'>
      <span className='border-border/80 bg-muted/60 inline-flex w-fit items-center rounded-md border px-1.5 py-0.5 font-mono text-xs font-semibold tabular-nums'>
        {chargedDisplay(breakdown, fallbackQuota)}
      </span>
      {!compact && rows.length > 0 && (
        <div
          className={cn(
            'space-y-0.5 text-[11px]',
            mode === 'internal'
              ? 'text-orange-700 dark:text-orange-300'
              : 'text-muted-foreground'
          )}
        >
          {rows.map((row) => (
            <div key={`${row.label}-${row.value}`} className='min-w-0'>
              <span className='font-medium'>{t(row.label)}:</span>{' '}
              <span className='font-mono break-words'>{row.value}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
