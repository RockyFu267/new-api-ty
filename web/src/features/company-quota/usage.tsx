import { useTranslation } from 'react-i18next'

import { formatQuota } from '@/lib/format'

import type { Period, QuotaUsage } from './api'

export function QuotaPeriodDisplay(props: {
  period: Period
  configured: boolean
}) {
  const { t } = useTranslation()
  const period = props.period
  let remaining = t('Not configured')
  if (props.configured) {
    remaining =
      period.limit > 0
        ? formatQuota(Math.max(0, period.limit - period.used - period.reserved))
        : t('No period limit')
  }
  return (
    <div className='space-y-1 text-sm'>
      <p>
        {t('Limit')}: {period.limit > 0 ? formatQuota(period.limit) : '-'}
      </p>
      <p>
        {t('Used')}: {formatQuota(period.used)}
      </p>
      <p>
        {t('Reserved')}: {formatQuota(period.reserved)}
      </p>
      <p>
        {t('Remaining')}: {remaining}
      </p>
      <p className='text-muted-foreground text-xs'>
        {t('Next reset')}:{' '}
        {new Date(period.reset_at * 1000).toLocaleString('zh-CN', {
          timeZone: 'Asia/Shanghai',
        })}
      </p>
    </div>
  )
}

export function QuotaUsageCard(props: { usage: QuotaUsage; enabled: boolean }) {
  const { t } = useTranslation()
  return (
    <div className='space-y-4 rounded-xl border p-4'>
      <p className='font-semibold'>
        {t('Department')}: {props.usage.department_name || t('Unassigned')}
      </p>
      <p>
        {t('Wallet balance')}: {formatQuota(props.usage.wallet)}
      </p>
      <p>
        {t('Wallet consumption this month')}:{' '}
        {formatQuota(props.usage.wallet_monthly_used)}
      </p>
      <p className='text-muted-foreground text-sm'>
        {props.enabled
          ? t(
              'Company quota is used first; wallet balance covers requests when quota is unavailable.'
            )
          : t(
              'Enterprise quota mode is disabled. Existing billing rules apply.'
            )}
      </p>
      <div className='grid gap-4 sm:grid-cols-3'>
        {(['daily', 'weekly', 'monthly'] as const).map((period) => (
          <div key={period} className='bg-muted/30 space-y-2 rounded-lg p-3'>
            <p className='font-medium'>{t(period)}</p>
            <QuotaPeriodDisplay
              period={props.usage[period]}
              configured={props.enabled && props.usage.configured}
            />
          </div>
        ))}
      </div>
    </div>
  )
}
