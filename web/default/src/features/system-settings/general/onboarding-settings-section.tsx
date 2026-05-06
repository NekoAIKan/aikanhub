import * as z from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
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
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const onboardingSchema = z.object({
  QuotaForNewUser: z.coerce.number().min(0),
  QuotaForInviter: z.coerce.number().min(0),
  QuotaForInvitee: z.coerce.number().min(0),
  onboarding_setting: z.object({
    new_user_quota: z.coerce.number().min(0),
    default_group: z.string().min(1),
    default_token_enabled: z.boolean(),
    default_token_group: z.string(),
    default_token_quota: z.coerce.number().min(0),
    default_token_unlimited: z.boolean(),
    require_invite_campaign_code: z.boolean(),
    default_token_expire_days: z.coerce.number().min(0),
    default_token_model_limits: z.string(),
  }),
})

type OnboardingFormValues = z.infer<typeof onboardingSchema>

type OnboardingSettingsSectionProps = {
  defaultValues: OnboardingFormValues
}

export function OnboardingSettingsSection({
  defaultValues,
}: OnboardingSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<OnboardingFormValues>({
      resolver: zodResolver(onboardingSchema) as Resolver<
        OnboardingFormValues,
        unknown,
        OnboardingFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          const optionValue =
            key === 'onboarding_setting.default_token_enabled'
              ? value
                ? 1
                : 0
              : value
          await updateOption.mutateAsync({
            key,
            value: optionValue as string | number | boolean,
          })
        }
      },
    })

  return (
    <SettingsSection
      title={t('Onboarding Quota Policy')}
      description={t(
        'Configure registration quota, default group, and starter token behavior.'
      )}
    >
      <FormNavigationGuard when={isDirty} />

      <Form {...form}>
        <form onSubmit={handleSubmit} className='space-y-6'>
          <FormDirtyIndicator isDirty={isDirty} />

          <div className='grid gap-4 md:grid-cols-3'>
            <FormField
              control={form.control}
              name='QuotaForNewUser'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Legacy new user quota')}</FormLabel>
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
                    {t(
                      'Existing quota option used by current registration code.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaForInviter'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Inviter reward')}</FormLabel>
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
                    {t('Quota given to users who invite others')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaForInvitee'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Invitee reward')}</FormLabel>
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
                    {t('Quota given to invited users')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-4 md:grid-cols-3'>
            <FormField
              control={form.control}
              name='onboarding_setting.new_user_quota'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Policy new user quota')}</FormLabel>
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
                    {t('Registration quota from the onboarding policy.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='onboarding_setting.default_group'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Default group')}</FormLabel>
                  <FormControl>
                    <Input placeholder='default' {...field} />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Group assigned to new users when backend support is enabled.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='onboarding_setting.default_token_quota'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Default token quota')}</FormLabel>
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
                    {t('Starter token quota when unlimited mode is off.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='onboarding_setting.default_token_group'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Starter token group')}</FormLabel>
                  <FormControl>
                    <Input placeholder='auto' {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('Leave empty to use the system default token group.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='onboarding_setting.default_token_expire_days'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Starter token expiry days')}</FormLabel>
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
                    {t('Use 0 to keep starter tokens from expiring.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='onboarding_setting.default_token_model_limits'
              render={({ field }) => (
                <FormItem className='md:col-span-2'>
                  <FormLabel>{t('Starter token model limits')}</FormLabel>
                  <FormControl>
                    <Input placeholder='gpt-4o,gpt-4o-mini' {...field} />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Comma-separated model IDs for starter tokens. Leave empty for no token-level model limit.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-4 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='onboarding_setting.require_invite_campaign_code'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Require campaign invite code')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'New password and OAuth registrations must provide an active campaign invite code.'
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

            <FormField
              control={form.control}
              name='onboarding_setting.default_token_enabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Generate starter token')}
                    </FormLabel>
                    <FormDescription>
                      {t('Create an API token automatically for new accounts.')}
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

            <FormField
              control={form.control}
              name='onboarding_setting.default_token_unlimited'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Starter token unlimited')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Allow the starter token to use the full account balance.'
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
