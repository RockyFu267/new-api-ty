import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { loadCompanyData, type QuotaUsage } from './api'
import { CompanyBills } from './bills'
import { QuotaUsageCard } from './usage'

export function CompanyWallet() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['company-quota-self'],
    queryFn: () =>
      loadCompanyData<{ enabled: boolean; usage: QuotaUsage }>('company-quota'),
    refetchInterval: 30000,
  })
  return (
    <div className='space-y-5 p-6'>
      <h1 className='text-2xl font-semibold'>{t('Quota and wallet')}</h1>
      {query.isPending && <p>{t('Loading...')}</p>}
      {query.isError && (
        <Button onClick={() => void query.refetch()}>{t('Retry')}</Button>
      )}
      {query.data && (
        <QuotaUsageCard usage={query.data.usage} enabled={query.data.enabled} />
      )}
      <p className='text-muted-foreground text-sm'>
        {t(
          'Ask your manager for additional wallet budget when company quota is exhausted.'
        )}
      </p>
      <CompanyBills />
    </div>
  )
}
