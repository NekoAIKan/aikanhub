import { useMemo } from 'react'
import * as z from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Calculator } from 'lucide-react'
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

const FIXTURE_CASES = [
  {
    labelKey: '720p text',
    tokens: 108900,
    quota: 54450,
    prechargeQuota: 67500,
  },
  {
    labelKey: '1080p text',
    tokens: 245024,
    quota: 122512,
    prechargeQuota: 151875,
  },
  {
    labelKey: 'Edit / multimodal',
    tokens: 324900,
    quota: 162450,
    prechargeQuota: 216000,
  },
  {
    labelKey: 'Extend',
    tokens: 389700,
    quota: 194850,
    prechargeQuota: 345600,
  },
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

  const preview = useMemo(() => {
    const quotaPerCredit = watched.credit_display_setting?.quota_per_credit || 1
    const markup = watched.profit_setting?.default_markup_percent || 0
    const upstreamCost =
      watched.profit_setting?.upstream_cost_per_million_tokens || 0
    const retailCost = upstreamCost * (1 + markup / 100)
    const effectiveRetailCost = retailCost > 0 ? retailCost : 1

    return {
      retailCost: effectiveRetailCost,
      cases: FIXTURE_CASES.map((fixture) => {
        const upstreamQuota =
          upstreamCost > 0
            ? Math.round((fixture.quota * upstreamCost) / effectiveRetailCost)
            : 0
        const grossMarginQuota = fixture.quota - upstreamQuota
        const grossMarginPercent =
          fixture.quota > 0 ? (grossMarginQuota / fixture.quota) * 100 : 0

        return {
          ...fixture,
          credits: fixture.quota / quotaPerCredit,
          retailUsd: fixture.quota / quotaPerUnit,
          upstreamUsd: upstreamQuota / quotaPerUnit,
          upstreamQuota,
          grossMarginQuota,
          grossMarginPercent,
        }
      }),
    }
  }, [
    quotaPerUnit,
    watched.credit_display_setting?.quota_per_credit,
    watched.profit_setting?.default_markup_percent,
    watched.profit_setting?.upstream_cost_per_million_tokens,
  ])

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

          <div
            data-testid='fixture-pricing-preview'
            className='rounded-lg border p-4'
          >
            <div className='mb-3 flex items-center gap-2'>
              <Calculator className='text-muted-foreground h-4 w-4' />
              <div className='text-sm font-medium'>
                {t('Fixture pricing preview')}
              </div>
              <Badge variant='outline'>{t('Admin preview')}</Badge>
            </div>
            <div className='overflow-x-auto'>
              <table className='w-full min-w-[720px] text-sm'>
                <thead className='text-muted-foreground border-b text-xs'>
                  <tr>
                    <th className='py-2 text-left font-medium'>{t('Case')}</th>
                    <th className='py-2 text-left font-medium'>
                      {t('Credits')}
                    </th>
                    <th className='py-2 text-left font-medium'>{t('Quota')}</th>
                    <th className='py-2 text-left font-medium'>
                      {t('Retail Charge')}
                    </th>
                    <th className='py-2 text-left font-medium'>
                      {t('Upstream Cost')}
                    </th>
                    <th className='py-2 text-left font-medium'>
                      {t('Gross margin')}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {preview.cases.map((row) => (
                    <tr key={row.labelKey} className='border-b last:border-0'>
                      <td className='py-2 font-medium'>{t(row.labelKey)}</td>
                      <td className='py-2 font-mono'>
                        {formatNumber(row.credits, 2)}{' '}
                        {watched.credit_display_setting?.label || t('Credits')}
                      </td>
                      <td className='py-2 font-mono'>
                        {formatNumber(row.quota, 0)}
                      </td>
                      <td className='py-2 font-mono'>
                        ${formatNumber(row.retailUsd, 4)}
                      </td>
                      <td className='py-2 font-mono'>
                        ${formatNumber(row.upstreamUsd, 4)}
                      </td>
                      <td className='py-2 font-mono'>
                        {formatNumber(row.grossMarginPercent, 2)}%
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

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
