import { useState } from 'react'
import type React from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import {
  createChannelModelCost,
  createFxRate,
  createRetailPricingPolicy,
  deleteChannelModelCost,
  deleteFxRate,
  deleteRetailPricingPolicy,
  getChannelModelCosts,
  getFxRates,
  getRetailPricingPolicies,
  previewMoneyPricingQuote,
  updateChannelModelCost,
  updateFxRate,
  updateRetailPricingPolicy,
} from '../api'
import type {
  ChannelModelCost,
  FxRate,
  MoneyPricingListResponse,
  RetailPricingPolicy,
} from '../types'

const defaultMoneyProfile = JSON.stringify(
  {
    schema_version: 1,
    profile_type: 'money_usage_pricing',
    currency: 'USD',
    rates: {
      input_token: {
        amount_micros: 1_000_000,
        basis: 'per_million_units',
        currency: 'USD',
      },
    },
  },
  null,
  2
)

const defaultFxRate: FxRate = {
  id: '',
  base_currency: 'CNY',
  quote_currency: 'USD',
  rate_micros: 137000,
  buffer_bps: 500,
  source: 'manual',
  effective_at: Math.floor(Date.now() / 1000),
}

const defaultChannelCost: ChannelModelCost = {
  channel_id: 0,
  upstream_model: '',
  endpoint_type: 'openai',
  billing_rule_json: defaultMoneyProfile,
  source: 'manual',
  source_ref: '',
  enabled: true,
}

const defaultRetailPolicy: RetailPricingPolicy = {
  public_model: '',
  group: 'default',
  endpoint_type: 'openai',
  pricing_mode: 'cost_plus',
  billing_rule_json: '',
  markup_bps: 3000,
  fx_policy: 'latest',
  fx_buffer_bps: 0,
  currency: 'USD',
  enabled: true,
}

function listItems<T>(response: MoneyPricingListResponse<T> | undefined): T[] {
  if (!response?.data) return []
  if (Array.isArray(response.data)) return response.data
  return response.data.items ?? []
}

function formatMicros(value: number | undefined, currency = '') {
  if (value === undefined || !Number.isFinite(value)) return '-'
  return `${currency ? `${currency} ` : ''}${(value / 1_000_000).toFixed(6)}`
}

function parseJsonObject(value: string) {
  const parsed = JSON.parse(value || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Expected JSON object')
  }
  return parsed as Record<string, unknown>
}

export function MoneyPricingAdminSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [fxForm, setFxForm] = useState<FxRate>(defaultFxRate)
  const [costForm, setCostForm] = useState<ChannelModelCost>(defaultChannelCost)
  const [policyForm, setPolicyForm] =
    useState<RetailPricingPolicy>(defaultRetailPolicy)
  const [previewFeatures, setPreviewFeatures] = useState(
    JSON.stringify({ InputTokens: 1_000_000 }, null, 2)
  )
  const [previewResult, setPreviewResult] = useState<
    Awaited<ReturnType<typeof previewMoneyPricingQuote>>['data'] | undefined
  >()

  const fxRatesQuery = useQuery({
    queryKey: ['money-pricing', 'fx-rates'],
    queryFn: getFxRates,
  })
  const channelCostsQuery = useQuery({
    queryKey: ['money-pricing', 'channel-costs'],
    queryFn: getChannelModelCosts,
  })
  const retailPoliciesQuery = useQuery({
    queryKey: ['money-pricing', 'retail-policies'],
    queryFn: getRetailPricingPolicies,
  })

  const fxRates = listItems(fxRatesQuery.data)
  const channelCosts = listItems(channelCostsQuery.data)
  const retailPolicies = listItems(retailPoliciesQuery.data)

  const invalidateMoneyPricing = () =>
    queryClient.invalidateQueries({ queryKey: ['money-pricing'] })

  const saveFxRate = useMutation({
    mutationFn: (rate: FxRate) =>
      fxRates.some((item) => item.id === rate.id)
        ? updateFxRate(rate)
        : createFxRate(rate),
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to save FX rate'))
        return
      }
      toast.success(t('FX rate saved'))
      invalidateMoneyPricing()
    },
  })

  const removeFxRate = useMutation({
    mutationFn: deleteFxRate,
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to delete FX rate'))
        return
      }
      toast.success(t('FX rate deleted'))
      invalidateMoneyPricing()
    },
  })

  const saveChannelCost = useMutation({
    mutationFn: (cost: ChannelModelCost) =>
      cost.id ? updateChannelModelCost(cost) : createChannelModelCost(cost),
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to save channel cost'))
        return
      }
      toast.success(t('Channel cost saved'))
      setCostForm(defaultChannelCost)
      invalidateMoneyPricing()
    },
  })

  const removeChannelCost = useMutation({
    mutationFn: deleteChannelModelCost,
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to delete channel cost'))
        return
      }
      toast.success(t('Channel cost deleted'))
      invalidateMoneyPricing()
    },
  })

  const saveRetailPolicy = useMutation({
    mutationFn: (policy: RetailPricingPolicy) =>
      policy.id
        ? updateRetailPricingPolicy(policy)
        : createRetailPricingPolicy(policy),
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to save retail policy'))
        return
      }
      toast.success(t('Retail policy saved'))
      setPolicyForm(defaultRetailPolicy)
      invalidateMoneyPricing()
    },
  })

  const removeRetailPolicy = useMutation({
    mutationFn: deleteRetailPricingPolicy,
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to delete retail policy'))
        return
      }
      toast.success(t('Retail policy deleted'))
      invalidateMoneyPricing()
    },
  })

  const previewQuote = useMutation({
    mutationFn: () =>
      previewMoneyPricingQuote({
        policy: policyForm,
        features: parseJsonObject(previewFeatures),
        settlement_currency: policyForm.currency,
      }),
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to preview quote'))
        return
      }
      setPreviewResult(response.data)
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to preview quote'))
    },
  })

  return (
    <div className='space-y-4'>
      <div>
        <h3 className='text-sm font-medium'>{t('Money pricing controls')}</h3>
        <p className='text-muted-foreground text-xs'>
          {t('Manage settlement FX, upstream channel costs, and retail rules.')}
        </p>
      </div>

      <Tabs defaultValue='fx' className='w-full'>
        <TabsList>
          <TabsTrigger value='fx'>{t('FX Rates')}</TabsTrigger>
          <TabsTrigger value='costs'>{t('Channel Costs')}</TabsTrigger>
          <TabsTrigger value='retail'>{t('Retail Policies')}</TabsTrigger>
        </TabsList>

        <TabsContent value='fx' className='space-y-4'>
          <div className='grid gap-3 md:grid-cols-6'>
            <Input
              placeholder='fx-cny-usd'
              value={fxForm.id}
              onChange={(event) =>
                setFxForm({ ...fxForm, id: event.target.value })
              }
            />
            <Input
              value={fxForm.base_currency}
              onChange={(event) =>
                setFxForm({
                  ...fxForm,
                  base_currency: event.target.value.toUpperCase(),
                })
              }
            />
            <Input
              value={fxForm.quote_currency}
              onChange={(event) =>
                setFxForm({
                  ...fxForm,
                  quote_currency: event.target.value.toUpperCase(),
                })
              }
            />
            <Input
              type='number'
              value={fxForm.rate_micros}
              onChange={(event) =>
                setFxForm({
                  ...fxForm,
                  rate_micros: event.target.valueAsNumber,
                })
              }
            />
            <Input
              type='number'
              value={fxForm.buffer_bps}
              onChange={(event) =>
                setFxForm({ ...fxForm, buffer_bps: event.target.valueAsNumber })
              }
            />
            <Button
              type='button'
              onClick={() => saveFxRate.mutate(fxForm)}
              disabled={saveFxRate.isPending}
            >
              {t('Save')}
            </Button>
          </div>

          <MoneyPricingTable
            isLoading={fxRatesQuery.isLoading}
            headers={['ID', 'Pair', 'Rate', 'Buffer', 'Source', '']}
            rows={fxRates.map((rate) => ({
              key: rate.id,
              cells: [
                rate.id,
                `${rate.base_currency}/${rate.quote_currency}`,
                rate.rate_micros.toString(),
                `${rate.buffer_bps} bps`,
                rate.source || '-',
                <div className='flex justify-end gap-2' key='actions'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => setFxForm(rate)}
                  >
                    {t('Edit')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='icon'
                    onClick={() => removeFxRate.mutate(rate)}
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                </div>,
              ],
            }))}
          />
        </TabsContent>

        <TabsContent value='costs' className='space-y-4'>
          <div className='grid gap-3 md:grid-cols-5'>
            <Input
              type='number'
              placeholder={t('Channel ID')}
              value={costForm.channel_id}
              onChange={(event) =>
                setCostForm({
                  ...costForm,
                  channel_id: event.target.valueAsNumber,
                })
              }
            />
            <Input
              placeholder={t('Upstream model')}
              value={costForm.upstream_model}
              onChange={(event) =>
                setCostForm({ ...costForm, upstream_model: event.target.value })
              }
            />
            <Input
              value={costForm.endpoint_type}
              onChange={(event) =>
                setCostForm({ ...costForm, endpoint_type: event.target.value })
              }
            />
            <Input
              value={costForm.source}
              onChange={(event) =>
                setCostForm({ ...costForm, source: event.target.value })
              }
            />
            <div className='flex items-center gap-2'>
              <Switch
                checked={costForm.enabled}
                onCheckedChange={(enabled) =>
                  setCostForm({ ...costForm, enabled })
                }
              />
              <Button
                type='button'
                onClick={() => saveChannelCost.mutate(costForm)}
                disabled={saveChannelCost.isPending}
              >
                {t('Save')}
              </Button>
            </div>
          </div>
          <Textarea
            className='min-h-40 font-mono text-xs'
            spellCheck={false}
            value={costForm.billing_rule_json}
            onChange={(event) =>
              setCostForm({
                ...costForm,
                billing_rule_json: event.target.value,
              })
            }
          />

          <MoneyPricingTable
            isLoading={channelCostsQuery.isLoading}
            headers={['ID', 'Channel', 'Model', 'Endpoint', 'Enabled', '']}
            rows={channelCosts.map((cost) => ({
              key: String(cost.id),
              cells: [
                cost.id?.toString() || '-',
                cost.channel_id.toString(),
                cost.upstream_model,
                cost.endpoint_type,
                <Badge
                  key='enabled'
                  variant={cost.enabled ? 'default' : 'outline'}
                >
                  {cost.enabled ? t('Enabled') : t('Disabled')}
                </Badge>,
                <div className='flex justify-end gap-2' key='actions'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => setCostForm(cost)}
                  >
                    {t('Edit')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='icon'
                    onClick={() => removeChannelCost.mutate(cost)}
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                </div>,
              ],
            }))}
          />
        </TabsContent>

        <TabsContent value='retail' className='space-y-4'>
          <div className='grid gap-3 md:grid-cols-6'>
            <Input
              placeholder={t('Public model')}
              value={policyForm.public_model}
              onChange={(event) =>
                setPolicyForm({
                  ...policyForm,
                  public_model: event.target.value,
                })
              }
            />
            <Input
              value={policyForm.group}
              onChange={(event) =>
                setPolicyForm({ ...policyForm, group: event.target.value })
              }
            />
            <Input
              value={policyForm.endpoint_type}
              onChange={(event) =>
                setPolicyForm({
                  ...policyForm,
                  endpoint_type: event.target.value,
                })
              }
            />
            <Select
              value={policyForm.pricing_mode}
              onValueChange={(value) =>
                setPolicyForm({
                  ...policyForm,
                  pricing_mode: value as RetailPricingPolicy['pricing_mode'],
                })
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='cost_plus'>{t('Cost plus')}</SelectItem>
                <SelectItem value='fixed_rule'>{t('Fixed rule')}</SelectItem>
              </SelectContent>
            </Select>
            <Input
              type='number'
              value={policyForm.markup_bps}
              onChange={(event) =>
                setPolicyForm({
                  ...policyForm,
                  markup_bps: event.target.valueAsNumber,
                })
              }
            />
            <div className='flex items-center gap-2'>
              <Switch
                checked={policyForm.enabled}
                onCheckedChange={(enabled) =>
                  setPolicyForm({ ...policyForm, enabled })
                }
              />
              <Button
                type='button'
                onClick={() => saveRetailPolicy.mutate(policyForm)}
                disabled={saveRetailPolicy.isPending}
              >
                {t('Save')}
              </Button>
            </div>
          </div>
          <div className='grid gap-3 md:grid-cols-3'>
            <Input
              value={policyForm.currency}
              onChange={(event) =>
                setPolicyForm({
                  ...policyForm,
                  currency: event.target.value.toUpperCase(),
                })
              }
            />
            <Input
              value={policyForm.fx_policy}
              onChange={(event) =>
                setPolicyForm({ ...policyForm, fx_policy: event.target.value })
              }
            />
            <Input
              type='number'
              value={policyForm.fx_buffer_bps}
              onChange={(event) =>
                setPolicyForm({
                  ...policyForm,
                  fx_buffer_bps: event.target.valueAsNumber,
                })
              }
            />
          </div>
          <Textarea
            className='min-h-40 font-mono text-xs'
            spellCheck={false}
            placeholder={t('Leave empty for channel cost-plus policies')}
            value={policyForm.billing_rule_json || ''}
            onChange={(event) =>
              setPolicyForm({
                ...policyForm,
                billing_rule_json: event.target.value,
              })
            }
          />
          <div className='grid gap-3 md:grid-cols-[1fr_auto]'>
            <Textarea
              className='min-h-24 font-mono text-xs'
              spellCheck={false}
              value={previewFeatures}
              onChange={(event) => setPreviewFeatures(event.target.value)}
            />
            <div className='flex flex-col gap-2'>
              <Button
                type='button'
                variant='outline'
                onClick={() => previewQuote.mutate()}
                disabled={previewQuote.isPending}
              >
                {t('Preview Quote')}
              </Button>
              <div className='text-muted-foreground text-xs'>
                {formatMicros(
                  previewResult?.RetailAmountMicros,
                  previewResult?.SettlementCurrency
                )}
              </div>
            </div>
          </div>

          <MoneyPricingTable
            isLoading={retailPoliciesQuery.isLoading}
            headers={['ID', 'Model', 'Group', 'Mode', 'Markup', '']}
            rows={retailPolicies.map((policy) => ({
              key: String(policy.id),
              cells: [
                policy.id?.toString() || '-',
                policy.public_model,
                policy.group,
                policy.pricing_mode,
                `${policy.markup_bps} bps`,
                <div className='flex justify-end gap-2' key='actions'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => setPolicyForm(policy)}
                  >
                    {t('Edit')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='icon'
                    onClick={() => removeRetailPolicy.mutate(policy)}
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                </div>,
              ],
            }))}
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}

type MoneyPricingTableProps = {
  isLoading: boolean
  headers: string[]
  rows: Array<{ key: string; cells: React.ReactNode[] }>
}

function MoneyPricingTable({
  isLoading,
  headers,
  rows,
}: MoneyPricingTableProps) {
  const { t } = useTranslation()
  return (
    <div className='overflow-x-auto rounded-md border'>
      <Table>
        <TableHeader>
          <TableRow>
            {headers.map((header) => (
              <TableHead key={header}>{header ? t(header) : ''}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? (
            <TableRow>
              <TableCell colSpan={headers.length}>
                <div className='text-muted-foreground flex items-center gap-2 py-4 text-sm'>
                  <RefreshCw className='h-4 w-4 animate-spin' />
                  {t('Loading...')}
                </div>
              </TableCell>
            </TableRow>
          ) : rows.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={headers.length}
                className='text-muted-foreground py-4 text-sm'
              >
                {t('No records')}
              </TableCell>
            </TableRow>
          ) : (
            rows.map((row) => (
              <TableRow key={row.key}>
                {row.cells.map((cell, index) => (
                  <TableCell key={`${row.key}-${index}`}>{cell}</TableCell>
                ))}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  )
}
