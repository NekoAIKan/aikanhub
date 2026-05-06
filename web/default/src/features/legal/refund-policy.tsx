import { useTranslation } from 'react-i18next'
import { getRefundPolicy } from './api'
import { LegalDocument } from './legal-document'

export function RefundPolicy() {
  const { t } = useTranslation()
  return (
    <LegalDocument
      title={t('Refund Policy')}
      queryKey='refund-policy'
      fetchDocument={getRefundPolicy}
      emptyMessage={t(
        'The administrator has not configured a refund policy yet.'
      )}
    />
  )
}
