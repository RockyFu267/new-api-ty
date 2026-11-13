import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ExchangeRateNotice } from '@/components/exchange-rate-notice'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { beijingDay } from '@/features/usage-analysis/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { getBill, millions, yuan, type Filters } from './api'
import { BillDetails, BillUsers } from './bill-tables'
import { BillingReportStatus } from './report-status'
import { DeepSeekRules } from './rules'

export function DeepSeekBillDemo() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = (user?.role ?? 0) >= ROLE.ADMIN
  const today = beijingDay()
  const from = new Date(`${today}T00:00:00+08:00`)
  from.setUTCDate(from.getUTCDate() - 29)
  const initial = {
    start: beijingDay(from),
    end: today,
    user_id: isAdmin ? '0' : String(user?.id ?? 0),
    page: 1,
  }
  const [draft, setDraft] = useState(initial)
  const [filters, setFilters] = useState<Filters>(initial)
  const [pageCursors, setPageCursors] = useState<Record<number, string>>({})
  const query = useQuery({
    queryKey: ['deepseek-bill-demo', user?.id, isAdmin, filters],
    queryFn: () => getBill(filters),
    retry: false,
    staleTime: 30_000,
    refetchOnWindowFocus: false,
    refetchInterval: (query) =>
      query.state.data?.status?.configured &&
      query.state.data.status.worker_running &&
      !query.state.data.status.history_complete
        ? 10_000
        : false,
  })
  const data = query.data
  const chosen = data?.users.find(
    (user) => String(user.user_id) === filters.user_id
  )
  const totals = chosen ?? data?.totals
  return (
    <main className='space-y-5 p-4 md:p-6'>
      <div>
        <h1 className='text-2xl font-semibold'>{t('DeepSeek bill demo')}</h1>
        <p className='text-muted-foreground mt-1'>
          deepseek-v4.1-flash ·{' '}
          {t('Original-provider billing and historical simulation')} · CNY
        </p>
      </div>
      {isAdmin && <DeepSeekRules />}
      <ExchangeRateNotice deepseek />
      <div className='rounded-xl border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950 dark:bg-amber-950/20 dark:text-amber-100'>
        <p>
          {t(
            'New requests with dedicated billing use their recorded rule and FX and consume existing quotas. Older requests are simulated using the 2026-11-09 reference prices only; they are never charged again. This is not a Ucloud invoice.'
          )}
        </p>
        <p className='mt-2'>
          {t(
            'Reference simulation: Beijing weekdays 09:00–12:00 and 14:00–18:00 are peak; weekends and 2026 holiday breaks are off-peak. Actual bills use the rule saved when the request started.'
          )}
        </p>
        <p className='mt-2'>
          {t(
            'Cache hits are included in input tokens. Missing cache reports use uncached prices and are marked estimated; invalid records are unpriced and excluded from cost totals.'
          )}
        </p>
      </div>
      <form
        className='flex flex-wrap items-end gap-3'
        onSubmit={(event) => {
          event.preventDefault()
          setFilters({ ...draft, user_id: initial.user_id, page: 1 })
          setPageCursors({})
        }}
      >
        <label className='space-y-1 text-sm'>
          {t('Start date')}
          <Input
            aria-label={t('Start date')}
            type='date'
            min='2026-08-17'
            max={today}
            value={draft.start}
            onChange={(event) =>
              setDraft({ ...draft, start: event.target.value })
            }
            required
          />
        </label>
        <label className='space-y-1 text-sm'>
          {t('End date')}
          <Input
            aria-label={t('End date')}
            type='date'
            min={draft.start}
            max={today}
            value={draft.end}
            onChange={(event) =>
              setDraft({ ...draft, end: event.target.value })
            }
            required
          />
        </label>
        <Button type='submit' disabled={query.isFetching}>
          {t('Query')}
        </Button>
        <Button
          type='button'
          variant='outline'
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
        >
          {t('Refresh')}
        </Button>
      </form>
      <div className='text-muted-foreground text-sm'>
        {t('Pricing snapshot')} 2026-11-09 ·{' '}
        <a
          className='underline'
          href='https://api-docs.deepseek.com/zh-cn/quick_start/pricing/'
          target='_blank'
          rel='noreferrer'
        >
          {t('Official pricing')}
        </a>{' '}
        ·{' '}
        {t(
          'Unit prices: uncached input / cache hit / output, CNY per million tokens. Peak: 2 / 0.04 / 8; off-peak: 1 / 0.02 / 4.'
        )}
      </div>
      {query.isPending && <p role='status'>{t('Loading')}</p>}
      {query.isError && (
        <p role='alert' className='text-destructive'>
          {t('Unable to load bill')}: {query.error.message}
        </p>
      )}
      <BillingReportStatus status={data?.status} />
      {data && (data.status?.history_complete ?? true) && (
        <>
          <h2 className='text-lg font-semibold'>
            {chosen?.username ?? (isAdmin ? t('All users') : user?.username)} ·{' '}
            {t('Billing summary')}
          </h2>
          <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
            {[
              [
                t('Total (actual + historical simulation)'),
                yuan((totals?.peak_units ?? 0) + (totals?.off_peak_units ?? 0)),
              ],
              [t('Requests'), String(totals?.requests ?? 0)],
              [
                t('Input / Output'),
                `${millions(totals?.input ?? 0)} / ${millions(totals?.output ?? 0)}`,
              ],
              [
                t('Estimated / Unpriced'),
                `${totals?.estimated ?? 0} / ${totals?.unpriced ?? 0}`,
              ],
            ].map(([label, value]) => (
              <div key={label} className='bg-card rounded-xl border p-4'>
                <p className='text-muted-foreground text-sm'>{label}</p>
                <p className='mt-2 text-xl font-semibold'>{value}</p>
              </div>
            ))}
          </div>
          <p className='text-muted-foreground text-sm'>
            {t('Actual dedicated billing requests')}: {totals?.actual ?? 0} ·{' '}
            {t('Actual quota charged (USD)')}: $
            {((totals?.actual_quota ?? 0) / 500000).toFixed(6)} ·{' '}
            {t('Historical simulation requests')}:{' '}
            {(totals?.requests ?? 0) - (totals?.actual ?? 0)}
          </p>
          {isAdmin && (
            <>
              <h2 className='text-lg font-semibold'>
                {t('Users with usage')} · {data.users.length}
              </h2>
              <BillUsers
                report={data}
                selected={filters.user_id}
                onSelect={(id) => {
                  setPageCursors({})
                  setFilters({
                    ...filters,
                    user_id: id,
                    page: 1,
                    cursor: undefined,
                  })
                }}
              />
              <div className='flex flex-wrap items-center gap-3'>
                <label className='text-sm' htmlFor='bill-user'>
                  {t('Detail user')}
                </label>
                <select
                  id='bill-user'
                  className='bg-background rounded-md border p-2 text-sm'
                  value={filters.user_id}
                  onChange={(event) => {
                    setPageCursors({})
                    setFilters({
                      ...filters,
                      user_id: event.target.value,
                      page: 1,
                      cursor: undefined,
                    })
                  }}
                >
                  <option value='0'>{t('All users')}</option>
                  {data.users.map((user) => (
                    <option key={user.user_id} value={String(user.user_id)}>
                      {user.username} · #{user.user_id}
                    </option>
                  ))}
                </select>
              </div>
            </>
          )}
          <BillDetails
            report={data}
            page={filters.page}
            loading={query.isFetching}
            onPage={(page) => {
              const cursor =
                page > filters.page ? data.next_cursor : pageCursors[page]
              if (page > 1 && !cursor) return
              setPageCursors({
                ...pageCursors,
                [filters.page]: filters.cursor ?? '',
                [page]: cursor ?? '',
              })
              setFilters({ ...filters, page, cursor: cursor || undefined })
            }}
          />
        </>
      )}
    </main>
  )
}
