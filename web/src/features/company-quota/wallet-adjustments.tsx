import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { formatQuota } from '@/lib/format'

import {
  loadCompanyData,
  parseBillingOther,
  type CompanyLog,
  type Page,
} from './api'

export function WalletAdjustments() {
  const { t } = useTranslation()
  const [keyword, setKeyword] = useState('')
  const [action, setAction] = useState('')
  const [start, setStart] = useState('')
  const [end, setEnd] = useState('')
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: ['wallet-adjustments', keyword, action, start, end, page],
    queryFn: () =>
      loadCompanyData<Page<CompanyLog>>('wallet-adjustments', {
        keyword,
        action,
        p: page,
        page_size: 20,
        start: start
          ? Math.floor(new Date(`${start}T00:00:00+08:00`).getTime() / 1000)
          : '',
        end: end
          ? Math.floor(new Date(`${end}T23:59:59+08:00`).getTime() / 1000)
          : '',
      }),
  })
  return (
    <div className='space-y-4 p-6'>
      <h1 className='text-2xl font-semibold'>{t('Wallet adjustments')}</h1>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Administrative balance changes. Model consumption is listed separately in wallet bills.'
        )}
      </p>
      <div className='flex flex-wrap gap-2'>
        <Input
          className='w-60'
          aria-label={t('User department or note')}
          placeholder={t('User department or note')}
          value={keyword}
          onChange={(event) => {
            setKeyword(event.target.value)
            setPage(1)
          }}
        />
        <select
          aria-label={t('Operation filter')}
          className='bg-background rounded-md border p-2'
          value={action}
          onChange={(event) => {
            setAction(event.target.value)
            setPage(1)
          }}
        >
          <option value=''>{t('All operations')}</option>
          {['add', 'subtract', 'override'].map((value) => (
            <option key={value} value={value}>
              {t(value)}
            </option>
          ))}
        </select>
        <Input
          className='w-44'
          aria-label={t('Start date')}
          type='date'
          value={start}
          onChange={(event) => {
            setStart(event.target.value)
            setPage(1)
          }}
        />
        <Input
          className='w-44'
          aria-label={t('End date')}
          type='date'
          value={end}
          onChange={(event) => {
            setEnd(event.target.value)
            setPage(1)
          }}
        />
      </div>
      {query.isPending && <p>{t('Loading...')}</p>}
      {query.isError && (
        <Button onClick={() => void query.refetch()}>{t('Retry')}</Button>
      )}
      <div className='overflow-auto rounded-lg border'>
        <table className='w-full text-left text-sm'>
          <thead>
            <tr>
              {[
                t('Time'),
                t('Operator'),
                t('Target user'),
                t('Department'),
                t('Operation'),
                t('Amount'),
                t('Before'),
                t('After'),
                t('Note'),
              ].map((label) => (
                <th className='p-3' key={label}>
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {(query.data?.items ?? []).map((log) => {
              const other = parseBillingOther(log)
              const op = other.op as
                | { action?: string; params?: Record<string, unknown> }
                | undefined
              const params = op?.params ?? {}
              const operator = other.admin_info as
                | { admin_username?: string }
                | undefined
              return (
                <tr className='border-t' key={log.id}>
                  <td className='p-3 whitespace-nowrap'>
                    {new Date(log.created_at * 1000).toLocaleString()}
                  </td>
                  <td className='p-3'>
                    {operator?.admin_username ?? log.username}
                  </td>
                  <td className='p-3'>
                    {String(
                      params.target_username ?? params.target_user_id ?? '—'
                    )}
                  </td>
                  <td className='p-3'>
                    {String(params.department_name ?? '—')}
                  </td>
                  <td className='p-3'>
                    {t(op?.action?.replace('user.quota_', '') ?? '—')}
                  </td>
                  <td className='p-3'>
                    {typeof params.value === 'number'
                      ? formatQuota(params.value)
                      : String(params.quota ?? '—')}
                  </td>
                  <td className='p-3'>
                    {typeof params.before === 'number'
                      ? formatQuota(params.before)
                      : String(params.from ?? '—')}
                  </td>
                  <td className='p-3'>
                    {typeof params.after === 'number'
                      ? formatQuota(params.after)
                      : String(params.to ?? '—')}
                  </td>
                  <td className='max-w-64 p-3 break-words'>
                    {String(params.note ?? '')}
                  </td>
                </tr>
              )
            })}
            {query.isSuccess && query.data.items.length === 0 && (
              <tr>
                <td className='p-6 text-center' colSpan={9}>
                  {t('No records')}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <div className='flex items-center gap-3'>
        <Button disabled={page === 1} onClick={() => setPage(page - 1)}>
          {t('Previous')}
        </Button>
        <span>
          {page} · {query.data?.total ?? 0}
        </span>
        <Button
          disabled={page * 20 >= (query.data?.total ?? 0)}
          onClick={() => setPage(page + 1)}
        >
          {t('Next')}
        </Button>
      </div>
    </div>
  )
}
