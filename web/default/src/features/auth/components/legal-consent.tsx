import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import type { SystemStatus } from '../types'

interface LegalConsentProps {
  status: SystemStatus | null
  checked: boolean
  onCheckedChange: (nextValue: boolean) => void
  className?: string
}

export function LegalConsent({
  status,
  checked,
  onCheckedChange,
  className,
}: LegalConsentProps) {
  const { t } = useTranslation()
  const hasUserAgreement = Boolean(status?.user_agreement_enabled)
  const hasPrivacyPolicy = Boolean(status?.privacy_policy_enabled)
  const hasRefundPolicy = Boolean(status?.refund_policy_enabled)

  const links = [
    hasUserAgreement
      ? { href: '/user-agreement', label: t('User Agreement') }
      : null,
    hasPrivacyPolicy
      ? { href: '/privacy-policy', label: t('Privacy Policy') }
      : null,
    hasRefundPolicy
      ? { href: '/refund-policy', label: t('Refund Policy') }
      : null,
  ].filter(Boolean) as Array<{ href: string; label: string }>

  if (links.length === 0) {
    return null
  }

  const handleChange = (value: boolean | 'indeterminate') => {
    onCheckedChange(value === true)
  }

  return (
    <div
      className={cn(
        'border-border/60 bg-muted/40 flex items-start gap-3 rounded-md border p-3',
        className
      )}
    >
      <Checkbox
        id='legal-consent'
        checked={checked}
        onCheckedChange={handleChange}
        className='mt-0.5'
      />
      <Label
        htmlFor='legal-consent'
        className='text-muted-foreground items-start gap-1 text-left text-xs leading-5 font-normal'
      >
        <span>
          {t('I have read and agree to the')}{' '}
          {links.map((link, index) => (
            <span key={link.href}>
              {index > 0 &&
                (index === links.length - 1 ? ` ${t('and')} ` : ', ')}
              <a
                href={link.href}
                target='_blank'
                rel='noopener noreferrer'
                className='text-primary hover:underline'
              >
                {link.label}
              </a>
            </span>
          ))}
          .
        </span>
      </Label>
    </div>
  )
}
