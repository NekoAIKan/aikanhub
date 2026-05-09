import { useEffect, useMemo, useState } from 'react'
import * as z from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Calculator, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  BILLING_VISIBILITY_MODES,
  DEFAULT_BILLING_GROUP_MODES,
  getBillingVisibilityDescriptionKey,
  getBillingVisibilityLabelKey,
  getBillingVisibilityShortLabelKey,
  normalizeBillingVisibilityMode,
  type BillingVisibilityMode,
} from '@/lib/billing-visibility'
import { api } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { tryJsonParse } from '../utils/json-parser'

const billingVisibilityModeSchema = z.enum([
  'credits',
  'summary',
  'detailed',
  'internal',
])

const creditsSchema = z.object({
  credit_display_setting: z.object({
    enabled: z.boolean(),
    label: z.string().min(1),
    quota_per_credit: z.coerce.number().positive(),
    precision: z.coerce.number().int().min(0).max(6),
  }),
  profit_setting: z.object({
    default_markup_percent: z.coerce.number().min(0).max(10000),
    upstream_cost_per_million_tokens: z.coerce.number().min(0),
    apply_to_default_video_profiles: z.boolean(),
  }),
  billing_visibility_setting: z.object({
    default_mode: billingVisibilityModeSchema,
    group_modes: z.string().superRefine((value, ctx) => {
      const result = tryJsonParse(value)
      if (!result.success) {
        ctx.addIssue({ code: 'custom', message: result.error })
        return
      }
      if (
        !result.data ||
        typeof result.data !== 'object' ||
        Array.isArray(result.data)
      ) {
        ctx.addIssue({ code: 'custom', message: 'Expected JSON object' })
      }
    }),
  }),
  video_billing_setting: z.object({
    profiles: z.string().superRefine((value, ctx) => {
      const result = tryJsonParse(value)
      if (!result.success) {
        ctx.addIssue({ code: 'custom', message: result.error })
      }
    }),
  }),
})

type CreditsFormValues = z.infer<typeof creditsSchema>

type CreditsSettingsSectionProps = {
  defaultValues: CreditsFormValues
  quotaPerUnit: number
}

const TRANSPARENCY_GROUPS = [
  'default',
  'trial',
  'beta',
  'invited',
  'b2b',
  'enterprise',
] as const

function formatNumber(value: number, digits = 2) {
  if (!Number.isFinite(value)) return '-'
  return value.toLocaleString(undefined, {
    maximumFractionDigits: digits,
    minimumFractionDigits: digits,
  })
}

function parseGroupModes(value: string): Record<string, BillingVisibilityMode> {
  const result = tryJsonParse(value)
  if (!result.success || !result.data || typeof result.data !== 'object') {
    return DEFAULT_BILLING_GROUP_MODES
  }

  const parsed = result.data as Record<string, unknown>
  return Object.fromEntries(
    Object.entries({
      ...DEFAULT_BILLING_GROUP_MODES,
      ...parsed,
    }).map(([group, mode]) => [group, normalizeBillingVisibilityMode(mode)])
  )
}

function stringifyGroupModes(modes: Record<string, BillingVisibilityMode>) {
  return JSON.stringify(modes, null, 2)
}

export function CreditsSettingsSection({
  defaultValues,
  quotaPerUnit,
}: CreditsSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<CreditsFormValues>({
      resolver: zodResolver(creditsSchema) as Resolver<
        CreditsFormValues,
        unknown,
        CreditsFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key,
            value: value as string | number | boolean,
          })
        }
      },
    })

  const watched = form.watch()
  const groupModes = useMemo(
    () =>
      parseGroupModes(
        watched.billing_visibility_setting?.group_modes ||
          stringifyGroupModes(DEFAULT_BILLING_GROUP_MODES)
      ),
    [watched.billing_visibility_setting?.group_modes]
  )

  const setGroupMode = (group: string, mode: BillingVisibilityMode): void => {
    const next = {
      ...groupModes,
      [group]: mode,
    }
    form.setValue(
      'billing_visibility_setting.group_modes',
      stringifyGroupModes(next),
      {
        shouldDirty: true,
        shouldValidate: true,
      }
    )
  }

  return (
    <SettingsSection
      title={t('Credits, Pricing & Profit')}
      description={t(
        'Configure public credits, retail pricing, billing visibility, and video billing profiles.'
      )}
    >
      <FormNavigationGuard when={isDirty} />

      <Form {...form}>
        <form onSubmit={handleSubmit} className='space-y-6'>
          <FormDirtyIndicator isDirty={isDirty} />

          <div className='space-y-4'>
            <div>
              <h3 className='text-sm font-medium'>
                {t('User credits display')}
              </h3>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Ordinary user balances, wallet, API key quota, and usage costs use this packaged credit unit.'
                )}
              </p>
            </div>

            <FormField
              control={form.control}
              name='credit_display_setting.enabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Enable credit display')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'When enabled, regular quota formatters show credits before legacy currency modes.'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={updateOption.isPending}
                    />
                  </FormControl>
                </FormItem>
              )}
            />

            <div className='grid gap-4 md:grid-cols-3'>
              <FormField
                control={form.control}
                name='credit_display_setting.label'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Credit label')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('Credits')} {...field} />
                    </FormControl>
                    <FormDescription>
                      {t('User-facing unit name for balances.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='credit_display_setting.quota_per_credit'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Quota per credit')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='1'
                        value={field.value as number}
                        onChange={(e) => field.onChange(e.target.valueAsNumber)}
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Internal quota units represented by one credit.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='credit_display_setting.precision'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Credit precision')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='0'
                        max='6'
                        value={field.value as number}
                        onChange={(e) => field.onChange(e.target.valueAsNumber)}
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Decimal places shown for credit amounts.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <div className='space-y-4'>
            <div>
              <h3 className='text-sm font-medium'>
                {t('Retail pricing & profit')}
              </h3>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'These inputs drive derived retail video pricing when a profile does not set an explicit unit price.'
                )}
              </p>
            </div>

            <div className='grid gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='profit_setting.upstream_cost_per_million_tokens'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Upstream cost per million tokens')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='0'
                        step='0.000001'
                        value={field.value as number}
                        onChange={(e) => field.onChange(e.target.valueAsNumber)}
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Provider cost basis for derived retail pricing.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='profit_setting.default_markup_percent'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default markup percent')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='0'
                        step='0.01'
                        value={field.value as number}
                        onChange={(e) => field.onChange(e.target.valueAsNumber)}
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Markup applied on top of upstream cost.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name='profit_setting.apply_to_default_video_profiles'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Apply profit inputs to default video profiles')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Explicit unit prices in a video profile still win over derived pricing.'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={updateOption.isPending}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </div>

          <VideoBillingLivePreview
            quotaPerUnit={quotaPerUnit}
            creditsLabel={watched.credit_display_setting?.label}
            quotaPerCredit={watched.credit_display_setting?.quota_per_credit}
          />

          <div className='space-y-4'>
            <div>
              <h3 className='text-sm font-medium'>
                {t('Transparency group policy')}
              </h3>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Groups such as invited, b2b, and enterprise resolve to detailed transparent billing by default.'
                )}
              </p>
            </div>

            <FormField
              control={form.control}
              name='billing_visibility_setting.default_mode'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Default billing visibility')}</FormLabel>
                  <Select
                    value={field.value}
                    onValueChange={(value) =>
                      field.onChange(normalizeBillingVisibilityMode(value))
                    }
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue
                          placeholder={t('Select billing visibility')}
                        />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {BILLING_VISIBILITY_MODES.filter(
                        (mode) => mode !== 'internal'
                      ).map((mode) => (
                        <SelectItem key={mode} value={mode}>
                          {t(getBillingVisibilityLabelKey(mode))}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t(
                      'Admin and root users always resolve to internal billing.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='grid gap-3 md:grid-cols-2'>
              {TRANSPARENCY_GROUPS.map((group) => {
                const mode = groupModes[group] || 'credits'
                return (
                  <div
                    key={group}
                    data-testid={`billing-group-mode-${group}`}
                    className='flex items-center justify-between gap-3 rounded-md border p-3'
                  >
                    <div className='min-w-0'>
                      <div className='font-mono text-sm'>{group}</div>
                      <div className='text-muted-foreground text-xs'>
                        {t(getBillingVisibilityDescriptionKey(mode))}
                      </div>
                    </div>
                    <Select
                      value={mode}
                      onValueChange={(value) =>
                        setGroupMode(
                          group,
                          normalizeBillingVisibilityMode(value)
                        )
                      }
                    >
                      <SelectTrigger className='w-[132px]'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {BILLING_VISIBILITY_MODES.filter(
                          (item) => item !== 'internal'
                        ).map((item) => (
                          <SelectItem key={item} value={item}>
                            {t(getBillingVisibilityShortLabelKey(item))}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                )
              })}
            </div>

            <FormField
              control={form.control}
              name='billing_visibility_setting.group_modes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Group mode JSON')}</FormLabel>
                  <FormControl>
                    <Textarea
                      className='min-h-32 font-mono text-xs'
                      spellCheck={false}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Advanced map of group name to credits, summary, or detailed billing visibility.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='video_billing_setting.profiles'
            render={({ field }) => (
              <FormItem>
                <div>
                  <h3 className='text-sm font-medium'>
                    {t('Video billing profiles')}
                  </h3>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Per-model formula profiles can override derived retail pricing and precharge behavior.'
                    )}
                  </p>
                </div>
                <FormControl>
                  <Textarea
                    className='min-h-52 font-mono text-xs'
                    spellCheck={false}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'JSON map of model names to video billing profile inputs.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button
            type='submit'
            disabled={updateOption.isPending || isSubmitting}
          >
            {updateOption.isPending ? t('Saving...') : t('Save Changes')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}

type VideoBillingPreviewCase = {
  label: string
  tokens: number
  unit_price_per_million: number
  raw_cost_usd: number
  quota: number
  has_reference_media: boolean
  resolution?: string
  basis: string
}

type VideoBillingPreviewResponse = {
  model: string
  profile_found: boolean
  quota_per_unit: number
  cases: VideoBillingPreviewCase[]
  available_models: string[]
}

type VideoBillingLivePreviewProps = {
  quotaPerUnit: number
  creditsLabel?: string
  quotaPerCredit?: number
}

function VideoBillingLivePreview({
  quotaPerUnit,
  creditsLabel,
  quotaPerCredit,
}: VideoBillingLivePreviewProps) {
  const { t } = useTranslation()
  const [availableModels, setAvailableModels] = useState<string[]>([])
  const [selectedModel, setSelectedModel] = useState<string>('')
  const [cases, setCases] = useState<VideoBillingPreviewCase[]>([])
  const [profileFound, setProfileFound] = useState<boolean>(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Initial load: fetch the list of configured models with empty model arg.
  useEffect(() => {
    let cancelled = false
    void (async () => {
      setLoading(true)
      setError(null)
      try {
        const res = await api.post<{
          success: boolean
          message?: string
          data?: VideoBillingPreviewResponse
        }>('/api/option/video_billing/preview', { model: '' })
        if (cancelled) return
        const data = res.data?.data
        if (data) {
          const models = [...(data.available_models ?? [])].sort()
          setAvailableModels(models)
          if (models.length > 0 && !selectedModel) {
            setSelectedModel(models[0])
          }
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err))
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Refetch when selected model changes.
  useEffect(() => {
    if (!selectedModel) {
      setCases([])
      setProfileFound(false)
      return
    }
    let cancelled = false
    void (async () => {
      setLoading(true)
      setError(null)
      try {
        const res = await api.post<{
          success: boolean
          message?: string
          data?: VideoBillingPreviewResponse
        }>('/api/option/video_billing/preview', { model: selectedModel })
        if (cancelled) return
        const data = res.data?.data
        if (data) {
          setCases(data.cases ?? [])
          setProfileFound(data.profile_found ?? false)
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err))
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [selectedModel])

  return (
    <div
      data-testid='video-billing-live-preview'
      className='rounded-lg border p-4'
    >
      <div className='mb-3 flex flex-wrap items-center gap-2'>
        <Calculator className='text-muted-foreground h-4 w-4' />
        <div className='text-sm font-medium'>
          {t('Video billing preview (live)')}
        </div>
        <Badge variant='outline'>{t('Admin preview')}</Badge>
        {loading && (
          <Loader2 className='text-muted-foreground ml-2 h-3 w-3 animate-spin' />
        )}
      </div>
      <p className='text-muted-foreground mb-3 text-xs'>
        {t(
          'Resolves canonical request shapes against the saved video_billing_setting.profiles entry for the selected model. Reflects the same code path as runtime billing.'
        )}
      </p>

      {availableModels.length === 0 ? (
        <div className='text-muted-foreground text-xs'>
          {t(
            'No video billing profiles configured. Save a profile in the JSON above first.'
          )}
        </div>
      ) : (
        <div className='mb-3 max-w-sm'>
          <Select value={selectedModel} onValueChange={setSelectedModel}>
            <SelectTrigger>
              <SelectValue placeholder={t('Select a model')} />
            </SelectTrigger>
            <SelectContent>
              {availableModels.map((model) => (
                <SelectItem key={model} value={model}>
                  {model}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      {error && (
        <div className='text-destructive mb-3 text-xs'>{error}</div>
      )}

      {selectedModel && !profileFound && !loading && (
        <div className='text-muted-foreground text-xs'>
          {t('No profile found for this model.')}
        </div>
      )}

      {profileFound && cases.length > 0 && (
        <div className='overflow-x-auto'>
          <table className='w-full min-w-[840px] text-sm'>
            <thead className='text-muted-foreground border-b text-xs'>
              <tr>
                <th className='py-2 text-left font-medium'>{t('Case')}</th>
                <th className='py-2 text-left font-medium'>{t('Tokens')}</th>
                <th className='py-2 text-left font-medium'>
                  {t('Unit price ($/1M)')}
                </th>
                <th className='py-2 text-left font-medium'>
                  {t('Retail Charge')}
                </th>
                <th className='py-2 text-left font-medium'>{t('Quota')}</th>
                {quotaPerCredit && quotaPerCredit > 0 && (
                  <th className='py-2 text-left font-medium'>
                    {t('Credits')}
                  </th>
                )}
              </tr>
            </thead>
            <tbody>
              {cases.map((row) => {
                const credits =
                  quotaPerCredit && quotaPerCredit > 0
                    ? row.quota / quotaPerCredit
                    : null
                const retailUsd =
                  quotaPerUnit > 0 ? row.quota / quotaPerUnit : 0
                return (
                  <tr key={row.label} className='border-b last:border-0'>
                    <td className='py-2 font-medium'>{row.label}</td>
                    <td className='py-2 font-mono'>
                      {formatNumber(row.tokens, 0)}
                    </td>
                    <td className='py-2 font-mono'>
                      ${formatNumber(row.unit_price_per_million, 4)}
                    </td>
                    <td className='py-2 font-mono'>
                      ${formatNumber(retailUsd, 4)}
                    </td>
                    <td className='py-2 font-mono'>
                      {formatNumber(row.quota, 0)}
                    </td>
                    {credits !== null && (
                      <td className='py-2 font-mono'>
                        {formatNumber(credits, 2)}{' '}
                        {creditsLabel || t('Credits')}
                      </td>
                    )}
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
