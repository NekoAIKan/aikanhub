import { useTranslation } from 'react-i18next'
import {
  getBillingVisibilityShortLabelKey,
  type BillingVisibilityMode,
} from '@/lib/billing-visibility'
import { StatusBadge, type StatusBadgeProps } from './status-badge'

type BillingVisibilityBadgeProps = {
  mode: BillingVisibilityMode
  size?: 'sm' | 'md' | 'lg'
}

const MODE_VARIANTS: Record<
  BillingVisibilityMode,
  StatusBadgeProps['variant']
> = {
  credits: 'neutral',
  summary: 'blue',
  detailed: 'purple',
  internal: 'orange',
}

export function BillingVisibilityBadge({
  mode,
  size = 'sm',
}: BillingVisibilityBadgeProps) {
  const { t } = useTranslation()

  return (
    <StatusBadge
      label={t(getBillingVisibilityShortLabelKey(mode))}
      variant={MODE_VARIANTS[mode]}
      size={size}
      copyable={false}
    />
  )
}
