import { createFileRoute } from '@tanstack/react-router'
import { CheckoutConfirm } from '@/features/service-checkout'

export const Route = createFileRoute('/checkout/confirm')({
  component: CheckoutConfirm,
})
