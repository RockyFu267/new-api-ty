import { useMutation, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import {
  beijingDay,
  exportReport,
  getReport,
  groupTrend,
  type Filters,
} from './api'
import { UsageBreakdown } from './breakdown'
import { UsageFilters } from './filters'
import { UsageOverview } from './overview'
import { ComparisonPrices } from './prices'
import { SavingsAnalysis } from './savings'

export function UsageAnalysis() {
  const { t } = useTranslation()
  const today = beijingDay()
  const [filters, setFilters] = useState<Filters>({
    start: `${today.slice(0, 7)}-01`,
    end: today,
    department_id: '',
    department_basis: 'current',
    user_id: '',
    model: '',
    local: '',
    dimension: 'department',
    page: 1,
  })
  const [tab, setTab] = useState('usage')
  const [period, setPeriod] = useState('day')
  const query = useQuery({
    queryKey: ['usage-analysis', filters],
    queryFn: () => getReport(filters),
    refetchInterval: 30_000,
  })
  const download = useMutation({ mutationFn: () => exportReport(filters) })
  const data = query.data
  const trend = groupTrend(data?.trend ?? [], period)
  function filter(key: keyof Filters, value: string) {
    setFilters((previous) => ({ ...previous, [key]: value, page: 1 }))
  }
  return (
    <main className='space-y-5 p-4 md:p-6'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <h1 className='text-2xl font-semibold'>{t('Usage analysis')}</h1>
        <div className='flex gap-2'>
          <Button variant='outline' onClick={() => void query.refetch()}>
            {t('Refresh')}
          </Button>
          <Button
            variant='outline'
            disabled={download.isPending || !data}
            onClick={() => download.mutate()}
          >
            {t('Export CSV')}
          </Button>
        </div>
      </div>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Administrator-only token reporting. Beijing time; weeks start Sunday. Cached tokens are part of input, not added again. Prices and billing are unchanged.'
        )}
      </p>
      <div className='flex flex-wrap gap-2'>
        {['usage', 'savings', 'prices'].map((value) => (
          <Button
            key={value}
            variant={tab === value ? 'default' : 'outline'}
            onClick={() => {
              setTab(value)
              if (value === 'savings') filter('dimension', 'model')
            }}
          >
            {t(
              {
                usage: 'Token usage',
                savings: 'Avoided API expenditure',
                prices: 'Comparison prices',
              }[value] ?? value
            )}
          </Button>
        ))}
      </div>
      <UsageFilters
        key={JSON.stringify(filters)}
        filters={filters}
        onApply={setFilters}
        onReset={setFilters}
      />
      {query.isPending && <p role='status'>{t('Loading...')}</p>}
      {query.isError && (
        <p role='alert' className='text-destructive'>
          {t('Unable to load usage analysis')}: {query.error.message}
        </p>
      )}
      {download.isError && (
        <p role='alert'>
          {t('Export failed')}: {download.error.message}
        </p>
      )}
      {data && (
        <>
          <p className='text-muted-foreground text-sm'>
            {t('Coverage starts')}: {data.coverage_start} ·{' '}
            {t('Last aggregation')}:{' '}
            {new Date(data.last_sync * 1000).toLocaleString('zh-CN', {
              timeZone: 'Asia/Shanghai',
            })}{' '}
            · {t('Pending requests')}: {data.pending}
          </p>
          {!data.imported && (
            <p role='status'>
              {t(
                'Historical import in progress. Totals are incomplete until import finishes.'
              )}
            </p>
          )}
          <p className='text-muted-foreground text-sm'>
            {t('Department reporting basis')}:{' '}
            {t(
              filters.department_basis === 'current'
                ? 'Current department members (including historical usage)'
                : 'Department at request time'
            )}
          </p>
          {tab === 'prices' && <ComparisonPrices prices={data.prices} />}
          {tab === 'savings' && (
            <SavingsAnalysis
              data={data}
              trend={trend}
              period={period}
              onPeriodChange={setPeriod}
            />
          )}
          {tab === 'usage' && (
            <UsageOverview
              data={data}
              trend={trend}
              period={period}
              onPeriodChange={setPeriod}
            />
          )}
          {tab !== 'prices' && (
            <UsageBreakdown
              data={data}
              filters={filters}
              savings={tab === 'savings'}
              onChange={setFilters}
            />
          )}
          <p className='text-muted-foreground text-sm'>
            {t(
              'Cache reporting may be partial. Not provided does not mean zero. Measured, estimated and unknown columns count requests, not tokens. Historical department assignments cannot be reconstructed. Daily summaries survive request-log cleanup.'
            )}
          </p>
        </>
      )}
    </main>
  )
}
