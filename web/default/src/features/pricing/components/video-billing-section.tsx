import { useEffect, useMemo, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { PricingModel, VideoBillingPricing, VideoBillingPricingPoint } from '../types'

type Props = {
  model: PricingModel
}

// SectionTitle is private to model-details.tsx so we re-declare a small
// equivalent here rather than refactor for one extra section.
function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <h2 className='text-muted-foreground mb-3 text-xs font-semibold tracking-wider uppercase'>
      {children}
    </h2>
  )
}

function formatUSD(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '$0'
  if (value < 0.01) return `$${value.toFixed(4)}`
  if (value < 1) return `$${value.toFixed(3)}`
  return `$${value.toFixed(2)}`
}

function formatInteger(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return Math.round(value).toLocaleString()
}

// VideoBillingSection renders the per-model pricing matrix + a live
// calculator for video formula models. Both data sources are
// authoritative — the matrix comes from `/api/pricing.video_billing`
// (server-computed from the same profile the runtime reads), and the
// calculator calls `/api/pricing/calculate` which runs through
// `service.CalculateVideoBilling` directly. So whatever this section
// shows is exactly what billing will charge.
export function VideoBillingSection({ model }: Props) {
  const { t } = useTranslation()
  const data: VideoBillingPricing | undefined = model.video_billing
  if (!data) return null

  // Matrix is grouped two ways: by resolution → durations,
  // separately for text vs with-video. Renders cleaner than one giant
  // 18-row table.
  const grouped = useMemo(() => groupMatrix(data.matrix), [data.matrix])

  return (
    <section className='border-b py-5'>
      <SectionLabel>{t('Pricing matrix')}</SectionLabel>
      <p className='text-muted-foreground mb-4 text-xs'>
        {t(
          'Per-token billing. Token count is derived from request shape; the rate ($/1M tokens) shown is what the customer is charged after profile resolution.'
        )}
      </p>

      <div className='space-y-5'>
        {grouped.map(({ hasVideo, rows }) => (
          <MatrixTable
            key={String(hasVideo)}
            title={
              hasVideo
                ? t('With reference video input')
                : t('Text or image input only')
            }
            rows={rows}
          />
        ))}
      </div>

      {data.rules && data.rules.length > 0 && (
        <ul className='text-muted-foreground mt-4 list-disc space-y-1 pl-5 text-xs'>
          {data.rules.map((rule) => (
            <li key={rule}>{rule}</li>
          ))}
        </ul>
      )}

      <LiveCalculator model={model} fallback={data.headline} />
    </section>
  )
}

function groupMatrix(matrix: VideoBillingPricingPoint[]): Array<{
  hasVideo: boolean
  rows: VideoBillingPricingPoint[]
}> {
  const text: VideoBillingPricingPoint[] = []
  const withVideo: VideoBillingPricingPoint[] = []
  for (const row of matrix) {
    ;(row.has_video_input ? withVideo : text).push(row)
  }
  const out: Array<{ hasVideo: boolean; rows: VideoBillingPricingPoint[] }> = []
  if (text.length > 0) out.push({ hasVideo: false, rows: text })
  if (withVideo.length > 0) out.push({ hasVideo: true, rows: withVideo })
  return out
}

function MatrixTable(props: { title: string; rows: VideoBillingPricingPoint[] }) {
  const { t } = useTranslation()

  // Pivot rows into resolution × duration grid. Columns are sorted
  // ascending; uniqueness is enforced so we don't render two rows for
  // the same resolution.
  const resolutions = Array.from(
    new Set(props.rows.map((r) => r.resolution || `${r.width}x${r.height}`))
  )
  const durations = Array.from(
    new Set(props.rows.map((r) => r.duration_seconds))
  ).sort((a, b) => a - b)

  const cellMap = new Map<string, VideoBillingPricingPoint>()
  for (const row of props.rows) {
    const key = `${row.resolution || `${row.width}x${row.height}`}::${row.duration_seconds}`
    cellMap.set(key, row)
  }

  return (
    <div className='rounded-md border'>
      <div className='border-b px-3 py-2 text-xs font-medium'>{props.title}</div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='w-32'>{t('Resolution')}</TableHead>
            {durations.map((dur) => (
              <TableHead key={dur}>{`${dur} ${t('s')}`}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {resolutions.map((res) => (
            <TableRow key={res}>
              <TableCell className='font-mono text-xs'>{res}</TableCell>
              {durations.map((dur) => {
                const cell = cellMap.get(`${res}::${dur}`)
                if (!cell) return <TableCell key={dur}>—</TableCell>
                return (
                  <TableCell key={dur}>
                    <div className='font-mono text-sm tabular-nums'>
                      {formatUSD(cell.price_usd)}
                    </div>
                    <div className='text-muted-foreground text-[10px]'>
                      ${cell.unit_price_per_million.toFixed(2)}/M ·{' '}
                      {formatInteger(cell.tokens)} tok
                    </div>
                  </TableCell>
                )
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

type CalculateResponse = {
  profile_found: boolean
  width: number
  height: number
  fps: number
  duration_seconds: number
  has_video_input: boolean
  tokens: number
  unit_price_per_million: number
  price_usd: number
  quota: number
}

function LiveCalculator(props: {
  model: PricingModel
  fallback: VideoBillingPricingPoint
}) {
  const { t } = useTranslation()
  const headline = props.fallback

  // Resolutions surfaced in the calculator: pull from the matrix so we
  // never offer one the operator hasn't priced.
  const resolutions = useMemo(() => {
    const set = new Set<string>()
    for (const row of props.model.video_billing?.matrix ?? []) {
      if (row.resolution) set.add(row.resolution)
    }
    return Array.from(set)
  }, [props.model])

  const [resolution, setResolution] = useState(headline.resolution || '720p')
  const [duration, setDuration] = useState(headline.duration_seconds)
  const [hasVideoInput, setHasVideoInput] = useState(false)
  const [result, setResult] = useState<CalculateResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    void (async () => {
      setLoading(true)
      setError(null)
      try {
        const res = await api.post<{
          success: boolean
          message?: string
          data?: CalculateResponse
        }>('/api/pricing/calculate', {
          model: props.model.model_name,
          resolution,
          duration_seconds: duration,
          has_video_input: hasVideoInput,
        })
        if (cancelled) return
        if (res.data?.data) {
          setResult(res.data.data)
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err))
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [props.model.model_name, resolution, duration, hasVideoInput])

  return (
    <div className='bg-muted/40 mt-5 rounded-md border p-4'>
      <div className='mb-3 flex items-center gap-2'>
        <SectionLabel>{t('Estimate price for a request')}</SectionLabel>
        {loading && (
          <Loader2 className='text-muted-foreground h-3 w-3 animate-spin' />
        )}
      </div>

      <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
        <div>
          <label className='text-muted-foreground mb-1 block text-[11px]'>
            {t('Resolution')}
          </label>
          <Select value={resolution} onValueChange={setResolution}>
            <SelectTrigger className='h-9'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {resolutions.map((r) => (
                <SelectItem key={r} value={r}>
                  {r}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div>
          <label className='text-muted-foreground mb-1 block text-[11px]'>
            {t('Duration (seconds)')}
          </label>
          <Select
            value={String(duration)}
            onValueChange={(v) => setDuration(Number(v))}
          >
            <SelectTrigger className='h-9'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {[3, 5, 8, 10, 12, 15].map((d) => (
                <SelectItem key={d} value={String(d)}>
                  {d}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className='flex items-end gap-2'>
          <Switch
            id='has-video'
            checked={hasVideoInput}
            onCheckedChange={setHasVideoInput}
          />
          <label htmlFor='has-video' className='text-xs leading-none'>
            {t('Includes reference video')}
          </label>
        </div>
      </div>

      {error && <div className='text-destructive mt-3 text-xs'>{error}</div>}

      {result && result.profile_found && (
        <div className='mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4'>
          <CalcMetric label={t('Tokens')} value={formatInteger(result.tokens)} />
          <CalcMetric
            label={t('Unit price ($/1M)')}
            value={`$${result.unit_price_per_million.toFixed(2)}`}
          />
          <CalcMetric
            label={t('Estimated charge')}
            value={formatUSD(result.price_usd)}
            highlight
          />
          <CalcMetric label={t('Quota')} value={formatInteger(result.quota)} />
        </div>
      )}

      {result && !result.profile_found && (
        <div className='text-muted-foreground mt-3 text-xs'>
          {t('No profile found for this model.')}
        </div>
      )}
    </div>
  )
}

function CalcMetric(props: {
  label: string
  value: string
  highlight?: boolean
}) {
  return (
    <div className='rounded-md border bg-background px-3 py-2'>
      <div className='text-muted-foreground text-[10px] tracking-wider uppercase'>
        {props.label}
      </div>
      <div
        className={
          props.highlight
            ? 'text-primary mt-0.5 font-mono text-sm font-semibold tabular-nums'
            : 'mt-0.5 font-mono text-sm tabular-nums'
        }
      >
        {props.value}
      </div>
    </div>
  )
}
