import { useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { parseCurrencyDisplayType } from '@/lib/currency'
import { useSystemOptions, getOptionValue } from '../hooks/use-system-options'
import type { GeneralSettings } from '../types'
import {
  GENERAL_DEFAULT_SECTION,
  getGeneralSectionContent,
} from './section-registry.tsx'

const defaultGeneralSettings: GeneralSettings = {
  'theme.frontend': 'default',
  Notice: '',
  SystemName: 'AIKanHub',
  Logo: '',
  Footer: '',
  About: '',
  HomePageContent: '',
  ServerAddress: '',
  'legal.user_agreement': '',
  'legal.privacy_policy': '',
  QuotaForNewUser: 0,
  PreConsumedQuota: 0,
  QuotaForInviter: 0,
  QuotaForInvitee: 0,
  TopUpLink: '',
  'general_setting.docs_link': '',
  'quota_setting.enable_free_model_pre_consume': true,
  'credit_display_setting.enabled': true,
  'credit_display_setting.label': 'Credits',
  'credit_display_setting.quota_per_credit': 500000,
  'credit_display_setting.precision': 2,
  'profit_setting.default_markup_percent': 30,
  'profit_setting.upstream_cost_per_million_tokens': 0,
  'video_billing_setting.profiles': '{}',
  'onboarding_setting.new_user_quota': 0,
  'onboarding_setting.default_group': 'default',
  'onboarding_setting.default_token_enabled': -1,
  'onboarding_setting.default_token_group': '',
  'onboarding_setting.default_token_quota': 500000,
  'onboarding_setting.default_token_unlimited': true,
  QuotaPerUnit: 500000,
  USDExchangeRate: 7,
  'general_setting.quota_display_type': 'USD',
  'general_setting.custom_currency_symbol': '¤',
  'general_setting.custom_currency_exchange_rate': 1,
  RetryTimes: 0,
  DisplayInCurrencyEnabled: true,
  DisplayTokenStatEnabled: true,
  DefaultCollapseSidebar: false,
  DemoSiteEnabled: false,
  SelfUseModeEnabled: false,
  'checkin_setting.enabled': false,
  'checkin_setting.min_quota': 1000,
  'checkin_setting.max_quota': 10000,
  'channel_affinity_setting.enabled': false,
  'channel_affinity_setting.switch_on_success': true,
  'channel_affinity_setting.max_entries': 100000,
  'channel_affinity_setting.default_ttl_seconds': 3600,
  'channel_affinity_setting.rules': '[]',
}

export function GeneralSettings() {
  const { t } = useTranslation()
  const { data, isLoading } = useSystemOptions()
  const params = useParams({
    from: '/_authenticated/system-settings/general/$section',
  })

  if (isLoading) {
    return (
      <div className='flex items-center justify-center py-12'>
        <div className='text-muted-foreground'>{t('Loading settings...')}</div>
      </div>
    )
  }

  const settings = getOptionValue(data?.data, defaultGeneralSettings)
  const quotaDisplayType = parseCurrencyDisplayType(
    settings['general_setting.quota_display_type']
  )
  const activeSection = (params?.section ?? GENERAL_DEFAULT_SECTION) as
    | 'system-info'
    | 'quota'
    | 'credits-profit'
    | 'onboarding'
    | 'invite-campaigns'
    | 'config-bundles'
    | 'pricing'
    | 'checkin'
    | 'behavior'
    | 'channel-affinity'
  const sectionContent = getGeneralSectionContent(
    activeSection,
    settings,
    quotaDisplayType
  )

  return (
    <div className='flex h-full w-full flex-1 flex-col'>
      <div className='faded-bottom h-full w-full overflow-y-auto scroll-smooth pe-4 pb-12'>
        <div className='space-y-4'>{sectionContent}</div>
      </div>
    </div>
  )
}
