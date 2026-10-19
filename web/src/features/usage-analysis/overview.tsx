import { useTranslation } from 'react-i18next'
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { Button } from '@/components/ui/button'

import type { Report, Bucket } from './api'
import { formatTokenMillions } from './format'
import { UsageTable } from './usage-table'

export function UsageOverview(props: {
  data: Report
  trend: Bucket[]
  period: string
  onPeriodChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const data = props.data
  const trend = props.trend
  const period = props.period
  const setPeriod = props.onPeriodChange
  return (
    <>
      <h2 className='font-medium'>{t('Selected range totals')}</h2>
      <p>
        {t('Total tokens')}:{' '}
        <span
          title={`${(data.totals.input + data.totals.output).toLocaleString()} tokens`}
        >
          {formatTokenMillions(data.totals.input + data.totals.output)}
        </span>{' '}
        · {t('Active users')}: {data.users} · {t('Models used')}: {data.models}
      </p>
      <UsageTable rows={[{ ...data.totals, name: t('Selected range') }]} />
      <h2 className='font-medium'>{t('Current periods and averages')}</h2>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Periods include today so far. Averages include zero-use dates: monthly uses complete months in the previous year; daily and weekly use up to 30 complete days. Filters apply; selected dates only affect range totals and details.'
        )}
      </p>
      <p className='text-muted-foreground text-sm'>
        {t('Complete months covered')}: {data.month_count} ·{' '}
        {t('Complete days covered')}: {data.day_count}
      </p>
      <UsageTable
        rows={[
          'today',
          'week',
          'month',
          'monthly_average',
          'weekly_average',
          'daily_average',
        ]
          .filter((key) => data.periods[key])
          .map((key) => ({
            ...data.periods[key],
            name: t(
              {
                today: 'Today so far',
                week: 'This week so far',
                month: 'This month so far',
                monthly_average: 'Monthly average over previous year',
                weekly_average: 'Weekly average over recent month',
                daily_average: 'Daily average over recent month',
              }[key] ?? key
            ),
          }))}
      />
      <div className='flex flex-wrap items-center gap-2'>
        <h2 className='mr-auto font-medium'>{t('Usage trend')}</h2>
        {['day', 'week', 'month'].map((p) => (
          <Button
            key={p}
            variant={period === p ? 'default' : 'outline'}
            onClick={() => setPeriod(p)}
          >
            {t({ day: 'Daily', week: 'Weekly', month: 'Monthly' }[p] ?? p)}
          </Button>
        ))}
      </div>
      <div
        className='h-72 w-full'
        role='img'
        aria-label={t('Input and output token usage trend')}
      >
        <ResponsiveContainer width='100%' height='100%'>
          <BarChart data={trend}>
            <CartesianGrid strokeDasharray='3 3' />
            <XAxis dataKey='name' />
            <YAxis tickFormatter={formatTokenMillions} width={90} />
            <Tooltip
              formatter={(value) => formatTokenMillions(Number(value))}
            />
            <Bar
              dataKey='input'
              name={t('Input tokens')}
              stackId='tokens'
              fill='#3b82f6'
            />
            <Bar
              dataKey='output'
              name={t('Output tokens')}
              stackId='tokens'
              fill='#10b981'
            />
          </BarChart>
        </ResponsiveContainer>
      </div>
      <UsageTable rows={trend} />
    </>
  )
}
