import { api } from '@/lib/api'
import type {
  ConfigBundleExportResponse,
  ConfigBundleImportResponse,
  ConfigBundlePreviewResponse,
  DeleteLogsResponse,
  FetchUpstreamRatiosRequest,
  ChannelModelCost,
  FxRate,
  InviteCampaign,
  InviteCampaignMutationResponse,
  InviteCampaignsResponse,
  MoneyPricingListResponse,
  MoneyPricingMutationResponse,
  MoneyPricingQuotePreviewResponse,
  RetailPricingPolicy,
  SystemOptionsResponse,
  UpdateOptionRequest,
  UpdateOptionResponse,
  UpstreamChannelsResponse,
  UpstreamRatiosResponse,
} from './types'

const quietRequestConfig = {
  skipErrorHandler: true,
  skipBusinessError: true,
} as Record<string, unknown>

export async function getSystemOptions() {
  const res = await api.get<SystemOptionsResponse>('/api/option/')
  return res.data
}

export async function updateSystemOption(request: UpdateOptionRequest) {
  const res = await api.put<UpdateOptionResponse>('/api/option/', request)
  return res.data
}

export async function deleteLogsBefore(targetTimestamp: number) {
  const res = await api.delete<DeleteLogsResponse>('/api/log/', {
    params: { target_timestamp: targetTimestamp },
  })
  return res.data
}

export async function resetModelRatios() {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/rest_model_ratio'
  )
  return res.data
}

export async function getUpstreamChannels() {
  const res = await api.get<UpstreamChannelsResponse>(
    '/api/ratio_sync/channels'
  )
  return res.data
}

export async function fetchUpstreamRatios(request: FetchUpstreamRatiosRequest) {
  const res = await api.post<UpstreamRatiosResponse>(
    '/api/ratio_sync/fetch',
    request
  )
  return res.data
}

export async function exportConfigBundle() {
  const res = await api.get<ConfigBundleExportResponse>(
    '/api/option/bundle/export',
    quietRequestConfig
  )
  return res.data
}

export async function previewConfigBundleImport(bundle: unknown) {
  const res = await api.post<ConfigBundlePreviewResponse>(
    '/api/option/bundle/import/preview',
    { bundle },
    quietRequestConfig
  )
  return res.data
}

export async function importConfigBundle(bundle: unknown) {
  const res = await api.post<ConfigBundleImportResponse>(
    '/api/option/bundle/import/apply',
    { bundle },
    quietRequestConfig
  )
  return res.data
}

export async function getInviteCampaigns() {
  const res = await api.get<InviteCampaignsResponse>(
    '/api/invite_campaign/',
    quietRequestConfig
  )
  return res.data
}

export async function createInviteCampaign(request: InviteCampaign) {
  const res = await api.post<InviteCampaignMutationResponse>(
    '/api/invite_campaign/',
    request,
    quietRequestConfig
  )
  return res.data
}

export async function updateInviteCampaign(request: InviteCampaign) {
  const campaignId = request.id ?? request.code
  const res = await api.put<InviteCampaignMutationResponse>(
    `/api/invite_campaign/${campaignId}`,
    request,
    quietRequestConfig
  )
  return res.data
}

export async function deleteInviteCampaign(campaign: InviteCampaign) {
  const campaignId = campaign.id ?? campaign.code
  const res = await api.delete<InviteCampaignMutationResponse>(
    `/api/invite_campaign/${campaignId}`,
    quietRequestConfig
  )
  return res.data
}

export async function getFxRates() {
  const res = await api.get<MoneyPricingListResponse<FxRate>>(
    '/api/money_pricing/fx_rates',
    quietRequestConfig
  )
  return res.data
}

export async function createFxRate(request: FxRate) {
  const res = await api.post<MoneyPricingMutationResponse<FxRate>>(
    '/api/money_pricing/fx_rates',
    request,
    quietRequestConfig
  )
  return res.data
}

export async function updateFxRate(request: FxRate) {
  const res = await api.put<MoneyPricingMutationResponse<FxRate>>(
    `/api/money_pricing/fx_rates/${request.id}`,
    request,
    quietRequestConfig
  )
  return res.data
}

export async function deleteFxRate(request: FxRate) {
  const res = await api.delete<MoneyPricingMutationResponse<null>>(
    `/api/money_pricing/fx_rates/${request.id}`,
    quietRequestConfig
  )
  return res.data
}

export async function getChannelModelCosts() {
  const res = await api.get<MoneyPricingListResponse<ChannelModelCost>>(
    '/api/money_pricing/channel_model_costs',
    quietRequestConfig
  )
  return res.data
}

export async function createChannelModelCost(request: ChannelModelCost) {
  const res = await api.post<MoneyPricingMutationResponse<ChannelModelCost>>(
    '/api/money_pricing/channel_model_costs',
    request,
    quietRequestConfig
  )
  return res.data
}

export async function updateChannelModelCost(request: ChannelModelCost) {
  const res = await api.put<MoneyPricingMutationResponse<ChannelModelCost>>(
    `/api/money_pricing/channel_model_costs/${request.id}`,
    request,
    quietRequestConfig
  )
  return res.data
}

export async function deleteChannelModelCost(request: ChannelModelCost) {
  const res = await api.delete<MoneyPricingMutationResponse<null>>(
    `/api/money_pricing/channel_model_costs/${request.id}`,
    quietRequestConfig
  )
  return res.data
}

export async function getRetailPricingPolicies() {
  const res = await api.get<MoneyPricingListResponse<RetailPricingPolicy>>(
    '/api/money_pricing/retail_policies',
    quietRequestConfig
  )
  return res.data
}

export async function createRetailPricingPolicy(request: RetailPricingPolicy) {
  const res = await api.post<MoneyPricingMutationResponse<RetailPricingPolicy>>(
    '/api/money_pricing/retail_policies',
    request,
    quietRequestConfig
  )
  return res.data
}

export async function updateRetailPricingPolicy(request: RetailPricingPolicy) {
  const res = await api.put<MoneyPricingMutationResponse<RetailPricingPolicy>>(
    `/api/money_pricing/retail_policies/${request.id}`,
    request,
    quietRequestConfig
  )
  return res.data
}

export async function deleteRetailPricingPolicy(request: RetailPricingPolicy) {
  const res = await api.delete<MoneyPricingMutationResponse<null>>(
    `/api/money_pricing/retail_policies/${request.id}`,
    quietRequestConfig
  )
  return res.data
}

export async function previewMoneyPricingQuote(request: {
  policy: RetailPricingPolicy
  features: Record<string, unknown>
  settlement_currency: string
}) {
  const res = await api.post<MoneyPricingQuotePreviewResponse>(
    '/api/money_pricing/quote_preview',
    request,
    quietRequestConfig
  )
  return res.data
}
