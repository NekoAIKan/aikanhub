import { type ReactNode, useMemo, useState } from 'react'
import { Link } from '@tanstack/react-router'
import {
  ArrowRight,
  BadgeCheck,
  CheckCircle2,
  Clock3,
  CreditCard,
  Film,
  ImagePlus,
  LockKeyhole,
  ReceiptText,
  ShieldCheck,
  Sparkles,
  WalletCards,
  Webhook,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { Button } from '@/components/ui/button'
import { PublicLayout } from '@/components/layout'

const PLAN_PRICE = '¥99.00'
const ORDER_TOTAL = '¥99.00'

type BuyerInfo = {
  name: string
  phone: string
  email: string
  account: string
}

export function VideoGenerationService() {
  const { t } = useTranslation()

  const planHighlights = [
    t('20 successful video generation tasks'),
    t('Valid for 30 days after activation'),
    t('Text-to-video and image-to-video requests'),
    t('Online delivery through API and webhook'),
  ]

  const serviceCapabilities = [
    {
      icon: <Film className='size-4' />,
      label: t('Text to video'),
      desc: t('Create short videos from prompts and style notes.'),
    },
    {
      icon: <ImagePlus className='size-4' />,
      label: t('Image to video'),
      desc: t('Animate reference images into generated video clips.'),
    },
    {
      icon: <Webhook className='size-4' />,
      label: t('Webhook delivery'),
      desc: t('Receive task results through polling or callbacks.'),
    },
  ]

  return (
    <PublicLayout showMainContainer={false}>
      <main className='min-h-screen bg-[linear-gradient(180deg,var(--background),var(--muted)_58%,var(--background))]'>
        <section className='mx-auto grid w-full max-w-7xl gap-10 px-4 pt-28 pb-14 sm:px-6 lg:grid-cols-[1.04fr_0.96fr] lg:pt-32 lg:pb-20'>
          <div className='flex flex-col justify-center'>
            <div className='border-border bg-background/80 text-muted-foreground mb-5 inline-flex w-fit items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-medium shadow-sm'>
              <BadgeCheck className='size-3.5 text-emerald-600' />
              {t('Commercial service')}
            </div>
            <h1 className='max-w-3xl text-[clamp(2.25rem,6vw,4.35rem)] leading-[1.04] font-semibold tracking-tight'>
              {t('AI Video Generation API Service')}
            </h1>
            <p className='text-muted-foreground mt-5 max-w-2xl text-base leading-7 sm:text-lg'>
              {t(
                'A checkout-ready service package for text-to-video, image-to-video, and webhook-based delivery.'
              )}
            </p>
            <div className='mt-8 flex flex-wrap items-center gap-3'>
              <Button className='rounded-lg' asChild>
                <Link to='/checkout/confirm'>
                  {t('View checkout')}
                  <ArrowRight className='ml-1 size-4' />
                </Link>
              </Button>
              <Button variant='outline' className='rounded-lg' asChild>
                <Link to='/pricing'>{t('View model pricing')}</Link>
              </Button>
            </div>
          </div>

          <div className='border-border bg-background overflow-hidden rounded-lg border shadow-sm'>
            <div className='border-border flex items-center justify-between border-b px-5 py-4'>
              <div>
                <p className='text-sm font-semibold'>
                  {t('Starter Video Generation Package')}
                </p>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t('Online API service')}
                </p>
              </div>
              <div className='text-right'>
                <p className='text-2xl font-semibold tabular-nums'>
                  {PLAN_PRICE}
                </p>
                <p className='text-muted-foreground text-xs'>
                  {t('One-time purchase')}
                </p>
              </div>
            </div>

            <div className='bg-border grid gap-px sm:grid-cols-3'>
              {serviceCapabilities.map((item) => (
                <div key={item.label} className='bg-background p-5'>
                  <div className='text-primary bg-primary/10 mb-3 flex size-9 items-center justify-center rounded-lg'>
                    {item.icon}
                  </div>
                  <h2 className='text-sm font-semibold'>{item.label}</h2>
                  <p className='text-muted-foreground mt-2 text-xs leading-5'>
                    {item.desc}
                  </p>
                </div>
              ))}
            </div>

            <div className='p-5 sm:p-6'>
              <div className='relative overflow-hidden rounded-lg bg-zinc-950 p-5 text-white'>
                <div className='absolute inset-0 bg-[radial-gradient(circle_at_20%_20%,rgba(59,130,246,0.24),transparent_34%),radial-gradient(circle_at_76%_30%,rgba(16,185,129,0.22),transparent_32%),linear-gradient(135deg,rgba(255,255,255,0.07),transparent_42%)]' />
                <div className='relative grid gap-3 sm:grid-cols-3'>
                  {['Seedance 2.0', '1080p', 'Webhook'].map((label, index) => (
                    <div
                      key={label}
                      className='min-h-28 rounded-lg border border-white/12 bg-white/8 p-3'
                    >
                      <div className='mb-8 flex items-center justify-between'>
                        <span className='rounded bg-white/12 px-2 py-1 text-[10px] font-medium'>
                          {label}
                        </span>
                        <span className='size-2 rounded-full bg-emerald-300' />
                      </div>
                      <div className='space-y-2'>
                        <div
                          className='h-2 rounded bg-white/25'
                          style={{ width: `${86 - index * 12}%` }}
                        />
                        <div
                          className='h-2 rounded bg-white/15'
                          style={{ width: `${58 + index * 10}%` }}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className='mt-5 grid gap-3 sm:grid-cols-2'>
                {planHighlights.map((item) => (
                  <div key={item} className='flex items-start gap-2 text-sm'>
                    <CheckCircle2 className='mt-0.5 size-4 shrink-0 text-emerald-600' />
                    <span>{item}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        <section className='bg-background/70 border-y'>
          <div className='mx-auto grid max-w-7xl gap-6 px-4 py-10 sm:px-6 md:grid-cols-3'>
            <ServiceFact
              icon={<Clock3 className='size-4' />}
              title={t('Activation time')}
              value={t('Immediate after payment')}
            />
            <ServiceFact
              icon={<ShieldCheck className='size-4' />}
              title={t('Service type')}
              value={t('Digital API service')}
            />
            <ServiceFact
              icon={<ReceiptText className='size-4' />}
              title={t('Order content')}
              value={t('Video generation credit package')}
            />
          </div>
        </section>
      </main>
    </PublicLayout>
  )
}

export function CheckoutConfirm() {
  const { t } = useTranslation()
  const { auth } = useAuthStore()

  const initialBuyer = useMemo<BuyerInfo>(
    () => ({
      name: auth.user?.display_name || auth.user?.username || t('Buyer name'),
      phone: t('Contact phone'),
      email: auth.user?.email || 'billing@example.com',
      account: auth.user?.email || auth.user?.username || 'api-user',
    }),
    [auth.user?.display_name, auth.user?.email, auth.user?.username, t]
  )

  const [buyer, setBuyer] = useState<BuyerInfo>(initialBuyer)

  const updateBuyer = (field: keyof BuyerInfo, value: string) => {
    setBuyer((current) => ({ ...current, [field]: value }))
  }

  return (
    <PublicLayout showMainContainer={false}>
      <main className='bg-muted/40 min-h-screen px-4 pt-24 pb-12 sm:px-6'>
        <div className='mx-auto max-w-6xl'>
          <div className='mb-6 flex flex-col gap-4 md:flex-row md:items-end md:justify-between'>
            <div>
              <p className='text-muted-foreground mb-2 text-xs font-medium tracking-widest uppercase'>
                {t('Checkout')}
              </p>
              <h1 className='text-3xl font-semibold tracking-tight md:text-4xl'>
                {t('Confirm service order')}
              </h1>
            </div>
            <div className='border-border bg-background flex w-full items-center justify-between rounded-lg border px-4 py-3 text-sm md:w-auto md:min-w-80'>
              {[t('Service'), t('Order'), t('Payment')].map((step, index) => (
                <div key={step} className='flex items-center gap-2'>
                  <span className='bg-primary text-primary-foreground flex size-6 items-center justify-center rounded-full text-xs font-semibold'>
                    {index + 1}
                  </span>
                  <span className='font-medium'>{step}</span>
                </div>
              ))}
            </div>
          </div>

          <div className='grid gap-5 lg:grid-cols-[minmax(0,1fr)_360px]'>
            <div className='space-y-5'>
              <section className='border-border bg-background rounded-lg border'>
                <SectionTitle
                  icon={<Film className='size-4' />}
                  title={t('Product or service information')}
                />
                <div className='divide-border divide-y'>
                  <div className='grid gap-4 p-5 md:grid-cols-[96px_minmax(0,1fr)_140px] md:items-center'>
                    <div className='flex aspect-square items-center justify-center rounded-lg bg-zinc-950 text-white'>
                      <Sparkles className='size-8 text-emerald-300' />
                    </div>
                    <div>
                      <h2 className='font-semibold'>
                        {t('Starter Video Generation Package')}
                      </h2>
                      <p className='text-muted-foreground mt-1 text-sm leading-6'>
                        {t(
                          'Includes 20 successful video generation tasks for text-to-video or image-to-video API calls.'
                        )}
                      </p>
                      <div className='mt-3 flex flex-wrap gap-2'>
                        {[
                          t('Digital service'),
                          t('30-day validity'),
                          t('API delivery'),
                        ].map((label) => (
                          <span
                            key={label}
                            className='border-border bg-muted rounded-md border px-2 py-1 text-xs'
                          >
                            {label}
                          </span>
                        ))}
                      </div>
                    </div>
                    <div className='text-left md:text-right'>
                      <p className='text-muted-foreground text-xs'>
                        {t('Quantity')}
                      </p>
                      <p className='font-semibold'>{t('1 package')}</p>
                      <p className='text-primary mt-2 text-xl font-semibold tabular-nums'>
                        {PLAN_PRICE}
                      </p>
                    </div>
                  </div>
                  <div className='grid gap-4 p-5 text-sm md:grid-cols-3'>
                    <OrderMeta
                      label={t('Service delivery')}
                      value={t('Online activation')}
                    />
                    <OrderMeta
                      label={t('Service period')}
                      value={t('30 days')}
                    />
                    <OrderMeta
                      label={t('Order number')}
                      value='KV202605070001'
                    />
                  </div>
                </div>
              </section>

              <section className='border-border bg-background rounded-lg border'>
                <SectionTitle
                  icon={<ReceiptText className='size-4' />}
                  title={t('Buyer information')}
                />
                <div className='grid gap-4 p-5 sm:grid-cols-2'>
                  <BuyerField
                    label={t('Buyer name')}
                    value={buyer.name}
                    onChange={(value) => updateBuyer('name', value)}
                  />
                  <BuyerField
                    label={t('Contact phone')}
                    value={buyer.phone}
                    onChange={(value) => updateBuyer('phone', value)}
                  />
                  <BuyerField
                    label={t('Email address')}
                    value={buyer.email}
                    onChange={(value) => updateBuyer('email', value)}
                  />
                  <BuyerField
                    label={t('Account identifier')}
                    value={buyer.account}
                    onChange={(value) => updateBuyer('account', value)}
                  />
                </div>
              </section>

              <section className='border-border bg-background rounded-lg border'>
                <SectionTitle
                  icon={<WalletCards className='size-4' />}
                  title={t('Payment method')}
                />
                <div className='grid gap-3 p-5 sm:grid-cols-2'>
                  <div className='border-primary bg-primary/5 flex items-center gap-3 rounded-lg border p-4'>
                    <div className='flex size-10 items-center justify-center rounded-lg bg-[#1677ff] text-sm font-bold text-white'>
                      {t('Alipay short label')}
                    </div>
                    <div>
                      <p className='font-semibold'>{t('Alipay')}</p>
                      <p className='text-muted-foreground text-xs'>
                        {t('Recommended payment channel')}
                      </p>
                    </div>
                  </div>
                  <div className='border-border flex items-center gap-3 rounded-lg border p-4 opacity-55'>
                    <CreditCard className='bg-muted text-muted-foreground size-10 rounded-lg p-2' />
                    <div>
                      <p className='font-semibold'>{t('Balance payment')}</p>
                      <p className='text-muted-foreground text-xs'>
                        {t('Not selected')}
                      </p>
                    </div>
                  </div>
                </div>
              </section>
            </div>

            <aside className='lg:sticky lg:top-24 lg:self-start'>
              <section className='border-border bg-background rounded-lg border'>
                <SectionTitle
                  icon={<LockKeyhole className='size-4' />}
                  title={t('Order summary')}
                />
                <div className='space-y-3 p-5 text-sm'>
                  <SummaryRow label={t('Service package')} value={PLAN_PRICE} />
                  <SummaryRow label={t('Discount')} value='¥0.00' />
                  <SummaryRow label={t('Tax')} value='¥0.00' />
                  <div className='border-border flex items-end justify-between border-t pt-4'>
                    <span className='font-medium'>{t('Total payable')}</span>
                    <span className='text-3xl font-semibold tracking-tight text-red-600 tabular-nums'>
                      {ORDER_TOTAL}
                    </span>
                  </div>
                  <div className='bg-muted text-muted-foreground rounded-lg p-3 text-xs leading-5'>
                    {t(
                      'The service is activated online after payment confirmation and delivered to the buyer account shown on this page.'
                    )}
                  </div>
                  <Button className='h-11 w-full rounded-lg text-sm font-semibold'>
                    {t('Submit order')}
                  </Button>
                </div>
              </section>
            </aside>
          </div>
        </div>
      </main>
    </PublicLayout>
  )
}

function ServiceFact(props: { icon: ReactNode; title: string; value: string }) {
  return (
    <div className='flex items-start gap-3'>
      <div className='border-border bg-muted flex size-9 shrink-0 items-center justify-center rounded-lg border'>
        {props.icon}
      </div>
      <div>
        <p className='text-sm font-semibold'>{props.title}</p>
        <p className='text-muted-foreground mt-1 text-sm'>{props.value}</p>
      </div>
    </div>
  )
}

function SectionTitle(props: { icon: ReactNode; title: string }) {
  return (
    <div className='border-border flex items-center gap-2 border-b px-5 py-4'>
      <div className='text-primary'>{props.icon}</div>
      <h2 className='text-sm font-semibold'>{props.title}</h2>
    </div>
  )
}

function OrderMeta(props: { label: string; value: string }) {
  return (
    <div>
      <p className='text-muted-foreground text-xs'>{props.label}</p>
      <p className='mt-1 font-medium'>{props.value}</p>
    </div>
  )
}

function BuyerField(props: {
  label: string
  value: string
  onChange: (value: string) => void
}) {
  return (
    <label className='block'>
      <span className='text-muted-foreground text-xs font-medium'>
        {props.label}
      </span>
      <input
        value={props.value}
        onChange={(event) => props.onChange(event.target.value)}
        className='border-input bg-background focus:ring-ring/30 mt-2 h-10 w-full rounded-lg border px-3 text-sm transition-shadow outline-none focus:ring-2'
      />
    </label>
  )
}

function SummaryRow(props: { label: string; value: string }) {
  return (
    <div className='flex items-center justify-between'>
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='font-medium tabular-nums'>{props.value}</span>
    </div>
  )
}
