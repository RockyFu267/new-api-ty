import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'

import { loadCompanyData, type QuotaUsage } from './api'
import { CompanyBills } from './bills'
import { QuotaUsageCard } from './usage'

export function UserQuotaDetails(props: {
  userId: number
  username: string
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['company-quota-user', props.userId],
    queryFn: () =>
      loadCompanyData<{ enabled: boolean; usage: QuotaUsage }>(
        `company-quota/users/${props.userId}`
      ),
    enabled: props.open,
  })
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[85vh] overflow-auto sm:max-w-5xl'>
        <DialogHeader>
          <DialogTitle>
            {props.username} · {t('Quota status')}
          </DialogTitle>
          <DialogDescription>
            {t('Company quota and wallet consumption are recorded separately.')}
          </DialogDescription>
        </DialogHeader>
        {query.isPending && <p>{t('Loading...')}</p>}
        {query.isError && (
          <Button onClick={() => void query.refetch()}>{t('Retry')}</Button>
        )}
        {query.data && (
          <QuotaUsageCard
            usage={query.data.usage}
            enabled={query.data.enabled}
          />
        )}
        <CompanyBills admin userId={props.userId} />
      </DialogContent>
    </Dialog>
  )
}
