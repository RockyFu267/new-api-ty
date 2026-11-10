import { useTranslation } from 'react-i18next'

import { useStatus } from '@/hooks/use-status'

export function ExchangeRateNotice({
  deepseek = false,
}: {
  deepseek?: boolean
}) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const configured = Number(status?.usd_exchange_rate)
  const rate = Number.isFinite(configured) && configured > 0 ? configured : 7.3
  return (
    <div className='text-muted-foreground bg-muted/40 shrink-0 rounded-lg px-3 py-2 text-xs leading-relaxed'>
      <p>
        {t(
          'Exchange rate: 1 USD = {{rate}} CNY. Wallet balances, department quotas and billed quota amounts are accounted for in USD; use this rate for CNY conversion.',
          { rate }
        )}
      </p>
      {deepseek && (
        <p>
          {t(
            'Historical DeepSeek actual bills retain the exchange rate recorded for each request; changing the current rate does not recalculate previous charges.'
          )}
        </p>
      )}
    </div>
  )
}
