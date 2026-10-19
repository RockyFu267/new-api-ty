import { useTranslation } from 'react-i18next'

import type { Report, Bucket } from './api'

export function SavingsAnalysis(props: {
  data: Report
  trend: Bucket[]
  period: string
  onPeriodChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const data = props.data
  const trend = props.trend
  return (
    <div className='space-y-3 rounded-lg border p-5'>
      <h2 className='text-lg font-medium'>
        {t('Estimated avoided commercial API expenditure')}
      </h2>
      <p className='text-3xl font-semibold'>
        USD{' '}
        {data.totals.avoided_usd.toLocaleString(undefined, {
          minimumFractionDigits: 2,
          maximumFractionDigits: 4,
        })}
      </p>
      <p>
        {t('Requests with effective comparison prices')}:{' '}
        {data.totals.priced_requests.toLocaleString()} /{' '}
        {data.totals.requests.toLocaleString()}
      </p>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Estimate uses normalized input and output at the effective comparison prices. No commercial cache discount or local infrastructure cost is assumed. This is not net savings; different models tokenize differently.'
        )}
      </p>
      <div className='overflow-auto'>
        <table className='w-full text-left text-sm'>
          <thead>
            <tr>
              <th className='p-2'>{t('Date')}</th>
              <th className='p-2'>
                {t('Estimated avoided commercial API expenditure')}
              </th>
              <th className='p-2'>
                {t('Requests with effective comparison prices')}
              </th>
            </tr>
          </thead>
          <tbody>
            {trend.map((row) => (
              <tr key={row.name} className='border-t'>
                <td className='p-2'>{row.name}</td>
                <td className='p-2'>USD {row.avoided_usd.toFixed(4)}</td>
                <td className='p-2'>{row.priced_requests.toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
