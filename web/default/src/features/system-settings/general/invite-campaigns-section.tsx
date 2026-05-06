import * as z from 'zod'
import axios from 'axios'
import type { Resolver } from 'react-hook-form'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Gift, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  createInviteCampaign,
  deleteInviteCampaign,
  getInviteCampaigns,
  updateInviteCampaign,
} from '../api'
import { SettingsSection } from '../components/settings-section'
import type { InviteCampaign } from '../types'

const campaignSchema = z.object({
  code: z.string().min(1),
  name: z.string().min(1),
  group: z.string().min(1),
  quota: z.coerce.number().min(0),
  max_uses: z.coerce.number().int().min(0),
  enabled: z.boolean(),
  expires_at: z.string().optional(),
})

type CampaignFormValues = z.infer<typeof campaignSchema>

const CAMPAIGN_STATUS_ENABLED = 1
const CAMPAIGN_STATUS_DISABLED = 2

function isRouteUnavailable(error: unknown) {
  if (!axios.isAxiosError(error)) return false
  return error.response?.status === 404 || error.response?.status === 405
}

function normalizeCampaigns(
  data: Awaited<ReturnType<typeof getInviteCampaigns>> | undefined
) {
  if (!data?.data) return []
  const campaigns = Array.isArray(data.data) ? data.data : (data.data.items ?? [])
  return campaigns.map((campaign) => ({
    ...campaign,
    enabled:
      campaign.enabled ??
      (campaign.status === CAMPAIGN_STATUS_ENABLED ||
        campaign.status === undefined),
    max_uses: campaign.max_uses ?? campaign.usage_limit ?? 0,
    expires_at:
      campaign.expires_at ??
      timestampToDateTimeLocal(campaign.end_time),
  }))
}

function timestampToDateTimeLocal(value: number | undefined) {
  if (!value || value <= 0) return ''
  const date = new Date(value * 1000)
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString().slice(0, 16)
}

function dateTimeLocalToTimestamp(value: string | undefined) {
  if (!value) return 0
  const timestamp = new Date(value).getTime()
  if (Number.isNaN(timestamp)) return 0
  return Math.floor(timestamp / 1000)
}

function formatDate(value: string | undefined, timestamp?: number) {
  if (timestamp && timestamp > 0) {
    return new Date(timestamp * 1000).toLocaleString()
  }
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function campaignPayload(values: CampaignFormValues): InviteCampaign {
  return {
    code: values.code,
    name: values.name,
    group: values.group,
    quota: values.quota,
    status: values.enabled
      ? CAMPAIGN_STATUS_ENABLED
      : CAMPAIGN_STATUS_DISABLED,
    usage_limit: values.max_uses,
    end_time: dateTimeLocalToTimestamp(values.expires_at),
  }
}

export function InviteCampaignsSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const campaignsQuery = useQuery({
    queryKey: ['invite-campaigns'],
    queryFn: getInviteCampaigns,
    retry: false,
  })

  const form = useForm<CampaignFormValues>({
    resolver: zodResolver(campaignSchema) as Resolver<
      CampaignFormValues,
      unknown,
      CampaignFormValues
    >,
    defaultValues: {
      code: '',
      name: '',
      group: 'default',
      quota: 0,
      max_uses: 0,
      enabled: true,
      expires_at: '',
    },
  })

  const createMutation = useMutation({
    mutationFn: createInviteCampaign,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to create campaign'))
        return
      }
      toast.success(t('Campaign created'))
      form.reset({
        code: '',
        name: '',
        group: 'default',
        quota: 0,
        max_uses: 0,
        enabled: true,
        expires_at: '',
      })
      queryClient.invalidateQueries({ queryKey: ['invite-campaigns'] })
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to create campaign'))
    },
  })

  const updateMutation = useMutation({
    mutationFn: updateInviteCampaign,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to update campaign'))
        return
      }
      toast.success(t('Campaign updated'))
      queryClient.invalidateQueries({ queryKey: ['invite-campaigns'] })
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to update campaign'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteInviteCampaign,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to delete campaign'))
        return
      }
      toast.success(t('Campaign deleted'))
      queryClient.invalidateQueries({ queryKey: ['invite-campaigns'] })
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to delete campaign'))
    },
  })

  const unavailable =
    isRouteUnavailable(campaignsQuery.error) ||
    campaignsQuery.data?.success === false
  const campaigns = normalizeCampaigns(campaignsQuery.data)
  const actionsDisabled =
    unavailable ||
    campaignsQuery.isLoading ||
    createMutation.isPending ||
    updateMutation.isPending ||
    deleteMutation.isPending

  const handleCreate = (values: CampaignFormValues) => {
    createMutation.mutate(campaignPayload(values))
  }

  const toggleCampaign = (campaign: InviteCampaign) => {
    if (!campaign.id && !campaign.code) return
    updateMutation.mutate({
      ...campaign,
      status: campaign.enabled
        ? CAMPAIGN_STATUS_DISABLED
        : CAMPAIGN_STATUS_ENABLED,
    })
  }

  return (
    <SettingsSection
      title={t('Campaign Invites')}
      description={t(
        'Create and manage campaign invite codes when backend support is available.'
      )}
    >
      {unavailable ? (
        <Empty className='border'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <Gift />
            </EmptyMedia>
            <EmptyTitle>{t('Campaign backend unavailable')}</EmptyTitle>
            <EmptyDescription>
              {t(
                'The frontend is ready, but /api/invite-campaign is not available in this backend build yet.'
              )}
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button
              type='button'
              variant='outline'
              onClick={() => campaignsQuery.refetch()}
            >
              <RefreshCw className='h-4 w-4' />
              {t('Retry')}
            </Button>
          </EmptyContent>
        </Empty>
      ) : (
        <div className='space-y-6'>
          <Form {...form}>
            <form
              onSubmit={form.handleSubmit(handleCreate)}
              className='grid gap-4 rounded-lg border p-4 md:grid-cols-6'
            >
              <FormField
                control={form.control}
                name='code'
                render={({ field }) => (
                  <FormItem className='md:col-span-2'>
                    <FormLabel>{t('Campaign code')}</FormLabel>
                    <FormControl>
                      <Input placeholder='BETA2026' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem className='md:col-span-2'>
                    <FormLabel>{t('Campaign name')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('Beta launch')} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='group'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Group')}</FormLabel>
                    <FormControl>
                      <Input placeholder='default' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='quota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='0'
                        value={field.value as number}
                        onChange={(e) => field.onChange(e.target.valueAsNumber)}
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='max_uses'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Max uses')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='0'
                        value={field.value as number}
                        onChange={(e) => field.onChange(e.target.valueAsNumber)}
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Use 0 for unlimited.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='expires_at'
                render={({ field }) => (
                  <FormItem className='md:col-span-2'>
                    <FormLabel>{t('Expires at')}</FormLabel>
                    <FormControl>
                      <Input type='datetime-local' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border px-3 py-2 md:col-span-2'>
                    <div>
                      <FormLabel>{t('Enabled')}</FormLabel>
                      <FormDescription>
                        {t('Accept this code during registration.')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />

              <div className='flex items-end gap-2 md:col-span-2'>
                <Button type='submit' disabled={actionsDisabled}>
                  <Plus className='h-4 w-4' />
                  {createMutation.isPending ? t('Creating...') : t('Create')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  onClick={() => campaignsQuery.refetch()}
                  disabled={campaignsQuery.isFetching}
                >
                  <RefreshCw className='h-4 w-4' />
                  {t('Refresh')}
                </Button>
              </div>
            </form>
          </Form>

          <div className='overflow-hidden rounded-lg border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Code')}</TableHead>
                  <TableHead>{t('Name')}</TableHead>
                  <TableHead>{t('Group')}</TableHead>
                  <TableHead>{t('Quota')}</TableHead>
                  <TableHead>{t('Usage')}</TableHead>
                  <TableHead>{t('Expires at')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {campaigns.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={8} className='h-24 text-center'>
                      {campaignsQuery.isLoading
                        ? t('Loading campaigns...')
                        : t('No campaign invites yet')}
                    </TableCell>
                  </TableRow>
                ) : (
                  campaigns.map((campaign) => (
                    <TableRow key={String(campaign.id ?? campaign.code)}>
                      <TableCell className='font-medium'>
                        {campaign.code ?? '-'}
                      </TableCell>
                      <TableCell>{campaign.name ?? '-'}</TableCell>
                      <TableCell>{campaign.group ?? 'default'}</TableCell>
                      <TableCell>{campaign.quota ?? 0}</TableCell>
                      <TableCell>
                        {(campaign.used_count ?? 0).toLocaleString()} /{' '}
                        {campaign.max_uses && campaign.max_uses > 0
                          ? campaign.max_uses.toLocaleString()
                          : t('Unlimited')}
                      </TableCell>
                      <TableCell>
                        {formatDate(campaign.expires_at, campaign.end_time)}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={campaign.enabled ? 'default' : 'secondary'}
                        >
                          {campaign.enabled ? t('Enabled') : t('Disabled')}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className='flex justify-end gap-2'>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            disabled={actionsDisabled}
                            onClick={() => toggleCampaign(campaign)}
                          >
                            {campaign.enabled ? t('Disable') : t('Enable')}
                          </Button>
                          <AlertDialog>
                            <AlertDialogTrigger asChild>
                              <Button
                                type='button'
                                variant='outline'
                                size='sm'
                                disabled={actionsDisabled}
                              >
                                <Trash2 className='h-4 w-4' />
                                {t('Delete')}
                              </Button>
                            </AlertDialogTrigger>
                            <AlertDialogContent>
                              <AlertDialogHeader>
                                <AlertDialogTitle>
                                  {t('Delete campaign invite?')}
                                </AlertDialogTitle>
                                <AlertDialogDescription>
                                  {t(
                                    'This campaign code will stop being accepted during registration.'
                                  )}
                                </AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel>
                                  {t('Cancel')}
                                </AlertDialogCancel>
                                <AlertDialogAction
                                  onClick={() =>
                                    deleteMutation.mutate(campaign)
                                  }
                                >
                                  {t('Delete')}
                                </AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </div>
      )}
    </SettingsSection>
  )
}
