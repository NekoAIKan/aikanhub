import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import type { SystemStatus } from '../types'

interface TermsFooterProps {
  variant?: 'sign-in' | 'sign-up'
  className?: string
  status?: SystemStatus | null
}

export function TermsFooter({
  variant = 'sign-in',
  className,
  status,
}: TermsFooterProps) {
  const { t } = useTranslation()
  const text =
    variant === 'sign-in'
      ? t('By clicking sign in, you agree to our')
      : t('By creating an account, you agree to our')

  const hasUserAgreement = Boolean(status?.user_agreement_enabled)
  const hasPrivacyPolicy = Boolean(status?.privacy_policy_enabled)
  const hasRefundPolicy = Boolean(status?.refund_policy_enabled)

  const activeLinks = [
    hasUserAgreement
      ? { label: t('User Agreement'), href: '/user-agreement' }
      : null,
    hasPrivacyPolicy
      ? { label: t('Privacy Policy'), href: '/privacy-policy' }
      : null,
    hasRefundPolicy
      ? { label: t('Refund Policy'), href: '/refund-policy' }
      : null,
  ].filter(Boolean) as Array<{ label: string; href: string }>

  if (activeLinks.length === 0) {
    return null
  }

  return (
    <p className={cn('text-muted-foreground text-center text-xs', className)}>
      {text}{' '}
      {activeLinks.map((link, index) => (
        <span key={link.href}>
          {index > 0 &&
            (index === activeLinks.length - 1 ? ` ${t('and')} ` : ', ')}
          <a
            href={link.href}
            className='hover:text-primary underline underline-offset-4'
          >
            {link.label}
          </a>
        </span>
      ))}
      .
    </p>
  )
}
