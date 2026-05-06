import type { GeneralSettings } from '../types'
import { parseBillingVisibilitySettings } from '@/lib/billing-visibility'
import { createSectionRegistry } from '../utils/section-registry'
import { ChannelAffinitySection } from './channel-affinity'
import { CheckinSettingsSection } from './checkin-settings-section'
import { ConfigBundlesSection } from './config-bundles-section'
import { CreditsSettingsSection } from './credits-settings-section'
import { InviteCampaignsSection } from './invite-campaigns-section'
import { OnboardingSettingsSection } from './onboarding-settings-section'
import { PricingSection } from './pricing-section'
import { QuotaSettingsSection } from './quota-settings-section'
import { SystemBehaviorSection } from './system-behavior-section'
import { SystemInfoSection } from './system-info-section'

const GENERAL_SECTIONS = [
  {
    id: 'system-info',
    titleKey: 'System Information',
    descriptionKey: 'Configure basic system information and branding',
    build: (settings: GeneralSettings) => (
      <SystemInfoSection
        defaultValues={{
          theme: {
            frontend: settings['theme.frontend'] as 'default' | 'classic',
          },
          Notice: settings.Notice,
          SystemName: settings.SystemName,
          Logo: settings.Logo,
          Footer: settings.Footer,
          About: settings.About,
          HomePageContent: settings.HomePageContent,
          ServerAddress: settings.ServerAddress,
          legal: {
            user_agreement: settings['legal.user_agreement'],
            privacy_policy: settings['legal.privacy_policy'],
          },
        }}
      />
    ),
  },
  {
    id: 'quota',
    titleKey: 'Quota Settings',
    descriptionKey: 'Configure user quota allocation and rewards',
    build: (settings: GeneralSettings) => (
      <QuotaSettingsSection
        defaultValues={{
          QuotaForNewUser: settings.QuotaForNewUser,
          PreConsumedQuota: settings.PreConsumedQuota,
          QuotaForInviter: settings.QuotaForInviter,
          QuotaForInvitee: settings.QuotaForInvitee,
          TopUpLink: settings.TopUpLink,
          general_setting: {
            docs_link: settings['general_setting.docs_link'],
          },
          quota_setting: {
            enable_free_model_pre_consume:
              settings['quota_setting.enable_free_model_pre_consume'],
          },
        }}
      />
    ),
  },
  {
    id: 'credits-profit',
    titleKey: 'Credits & Profit',
    descriptionKey: 'Configure credit display and profit preview inputs',
    build: (settings: GeneralSettings) => (
      <CreditsSettingsSection
        quotaPerUnit={settings.QuotaPerUnit}
        defaultValues={{
          credit_display_setting: {
            enabled: settings['credit_display_setting.enabled'],
            label: settings['credit_display_setting.label'],
            quota_per_credit:
              settings['credit_display_setting.quota_per_credit'],
            precision: settings['credit_display_setting.precision'],
          },
          profit_setting: {
            default_markup_percent:
              settings['profit_setting.default_markup_percent'],
            upstream_cost_per_million_tokens:
              settings['profit_setting.upstream_cost_per_million_tokens'],
            apply_to_default_video_profiles:
              settings['profit_setting.apply_to_default_video_profiles'],
          },
          billing_visibility_setting: {
            default_mode: settings['billing_visibility_setting.default_mode'] as
              | 'credits'
              | 'summary'
              | 'detailed'
              | 'internal',
            group_modes: settings['billing_visibility_setting.group_modes'],
          },
          video_billing_setting: {
            profiles: settings['video_billing_setting.profiles'],
          },
        }}
      />
    ),
  },
  {
    id: 'onboarding',
    titleKey: 'Onboarding',
    descriptionKey: 'Configure new user quotas, groups, and starter tokens',
    build: (settings: GeneralSettings) => (
      <OnboardingSettingsSection
        defaultValues={{
          QuotaForNewUser: settings.QuotaForNewUser,
          QuotaForInviter: settings.QuotaForInviter,
          QuotaForInvitee: settings.QuotaForInvitee,
          onboarding_setting: {
            new_user_quota: settings['onboarding_setting.new_user_quota'],
            default_group: settings['onboarding_setting.default_group'],
            default_token_enabled:
              settings['onboarding_setting.default_token_enabled'] > 0,
            default_token_group:
              settings['onboarding_setting.default_token_group'],
            default_token_quota:
              settings['onboarding_setting.default_token_quota'],
            default_token_unlimited:
              settings['onboarding_setting.default_token_unlimited'],
          },
        }}
      />
    ),
  },
  {
    id: 'invite-campaigns',
    titleKey: 'Campaign Invites',
    descriptionKey: 'Create and manage campaign invite codes',
    build: (settings: GeneralSettings) => (
      <InviteCampaignsSection
        billingVisibility={parseBillingVisibilitySettings({
          defaultMode: settings['billing_visibility_setting.default_mode'],
          groupModes: settings['billing_visibility_setting.group_modes'],
        })}
      />
    ),
  },
  {
    id: 'config-bundles',
    titleKey: 'Config Bundles',
    descriptionKey: 'Export, preview, and import configuration bundles',
    build: () => <ConfigBundlesSection />,
  },
  {
    id: 'pricing',
    titleKey: 'Pricing & Display',
    descriptionKey: 'Configure pricing model and display options',
    build: (
      settings: GeneralSettings,
      quotaDisplayType: 'USD' | 'CNY' | 'TOKENS' | 'CUSTOM'
    ) => (
      <PricingSection
        defaultValues={{
          QuotaPerUnit: settings.QuotaPerUnit,
          USDExchangeRate: settings.USDExchangeRate,
          DisplayInCurrencyEnabled: settings.DisplayInCurrencyEnabled,
          DisplayTokenStatEnabled: settings.DisplayTokenStatEnabled,
          general_setting: {
            quota_display_type: quotaDisplayType,
            custom_currency_symbol:
              settings['general_setting.custom_currency_symbol'] ?? '¤',
            custom_currency_exchange_rate:
              settings['general_setting.custom_currency_exchange_rate'] ?? 1,
          },
        }}
      />
    ),
  },
  {
    id: 'checkin',
    titleKey: 'Check-in Settings',
    descriptionKey: 'Configure daily check-in rewards for users',
    build: (settings: GeneralSettings) => (
      <CheckinSettingsSection
        defaultValues={{
          enabled: settings['checkin_setting.enabled'],
          minQuota: settings['checkin_setting.min_quota'],
          maxQuota: settings['checkin_setting.max_quota'],
        }}
      />
    ),
  },
  {
    id: 'behavior',
    titleKey: 'System Behavior',
    descriptionKey: 'Configure system-wide behavior and defaults',
    build: (settings: GeneralSettings) => (
      <SystemBehaviorSection
        defaultValues={{
          RetryTimes: settings.RetryTimes,
          DefaultCollapseSidebar: settings.DefaultCollapseSidebar,
          DemoSiteEnabled: settings.DemoSiteEnabled,
          SelfUseModeEnabled: settings.SelfUseModeEnabled,
        }}
      />
    ),
  },
  {
    id: 'channel-affinity',
    titleKey: 'Channel Affinity',
    descriptionKey: 'Configure channel affinity (sticky routing) rules',
    build: (settings: GeneralSettings) => (
      <ChannelAffinitySection
        defaultValues={{
          'channel_affinity_setting.enabled':
            settings['channel_affinity_setting.enabled'],
          'channel_affinity_setting.switch_on_success':
            settings['channel_affinity_setting.switch_on_success'],
          'channel_affinity_setting.max_entries':
            settings['channel_affinity_setting.max_entries'],
          'channel_affinity_setting.default_ttl_seconds':
            settings['channel_affinity_setting.default_ttl_seconds'],
          'channel_affinity_setting.rules':
            settings['channel_affinity_setting.rules'],
        }}
      />
    ),
  },
] as const

export type GeneralSectionId = (typeof GENERAL_SECTIONS)[number]['id']

const generalRegistry = createSectionRegistry<
  GeneralSectionId,
  GeneralSettings,
  ['USD' | 'CNY' | 'TOKENS' | 'CUSTOM']
>({
  sections: GENERAL_SECTIONS,
  defaultSection: 'system-info',
  basePath: '/system-settings/general',
  urlStyle: 'path',
})

export const GENERAL_SECTION_IDS = generalRegistry.sectionIds
export const GENERAL_DEFAULT_SECTION = generalRegistry.defaultSection
export const getGeneralSectionNavItems = generalRegistry.getSectionNavItems
export const getGeneralSectionContent = generalRegistry.getSectionContent
