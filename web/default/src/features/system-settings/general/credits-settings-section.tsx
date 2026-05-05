import { useMemo } from 'react'
import * as z from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { AlertTriangle, Calculator } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { tryJsonParse } from '../utils/json-parser'

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

function formatNumber(value: number, digits = 2) {
  if (!Number.isFinite(value)) return '-'
  return value.toLocaleString(undefined, {
    maximumFractionDigits: digits,
    minimumFractionDigits: digits,
  })
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
  const preview = useMemo(() => {
    const quotaPerCredit = watched.credit_display_setting?.quota_per_credit || 1
    const markup = watched.profit_setting?.default_markup_percent || 0
    const upstreamCost =
      watched.profit_setting?.upstream_cost_per_million_tokens || 0
    const retailCost = upstreamCost * (1 + markup / 100)
    const quotaCost = retailCost * quotaPerUnit
    const creditCost = quotaCost / quotaPerCredit
    const grossProfit = retailCost - upstreamCost
    const margin = retailCost > 0 ? (grossProfit / retailCost) * 100 : 0

    return {
      retailCost,
      quotaCost,
      creditCost,
      grossProfit,
      margin,
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
        'Configure credit display, video pricing inputs, and margin preview.'
      )}
    >
      <FormNavigationGuard when={isDirty} />

      <Alert>
        <AlertTriangle className='h-4 w-4' />
        <AlertTitle>{t('Preview only')}</AlertTitle>
        <AlertDescription>
          {t(
            'Profit preview is calculated in the browser until a backend preview endpoint is available.'
          )}
        </AlertDescription>
      </Alert>

      <Form {...form}>
        <form onSubmit={handleSubmit} className='space-y-6'>
          <FormDirtyIndicator isDirty={isDirty} />

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
                      'Show quota balances as configurable credits in admin previews.'
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

          <div className='grid gap-4 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='profit_setting.upstream_cost_per_million_tokens'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Upstream cost per million tokens')}</FormLabel>
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
                    {t('Reference provider cost used by the preview.')}
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

          <div className='rounded-lg border p-4'>
            <div className='mb-3 flex items-center gap-2'>
              <Calculator className='text-muted-foreground h-4 w-4' />
              <div className='text-sm font-medium'>{t('Profit preview')}</div>
              <Badge variant='outline'>{t('Client-side')}</Badge>
            </div>
            <div className='grid gap-3 text-sm md:grid-cols-4'>
              <div>
                <div className='text-muted-foreground'>{t('Retail cost')}</div>
                <div className='font-medium'>
                  ${formatNumber(preview.retailCost, 4)}
                </div>
              </div>
              <div>
                <div className='text-muted-foreground'>{t('Quota cost')}</div>
                <div className='font-medium'>
                  {formatNumber(preview.quotaCost, 0)}
                </div>
              </div>
              <div>
                <div className='text-muted-foreground'>{t('Credit cost')}</div>
                <div className='font-medium'>
                  {formatNumber(preview.creditCost, 4)}
                </div>
              </div>
              <div>
                <div className='text-muted-foreground'>{t('Gross margin')}</div>
                <div className='font-medium'>
                  {formatNumber(preview.margin, 2)}%
                </div>
              </div>
            </div>
          </div>

          <FormField
            control={form.control}
            name='video_billing_setting.profiles'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Video billing profiles')}</FormLabel>
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
