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

export function CompanyBills(props: { admin?: boolean; userId?: number }) {
  const { t } = useTranslation()
  const [source, setSource] = useState('department_quota')
  const [page, setPage] = useState(1)
  const [model, setModel] = useState('')
  const query = useQuery({
    queryKey: ['company-bills', props.admin, props.userId, source, page, model],
    queryFn: () =>
      loadCompanyData<Page<CompanyLog>>(
        props.admin ? 'company-quota/bills' : 'company-bills',
        {
          source,
          p: page,
          page_size: 20,
          model,
          ...(props.userId ? { user_id: props.userId } : {}),
        }
      ),
  })
  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap gap-2'>
        {(['department_quota', 'wallet'] as const).map((value) => (
          <Button
            key={value}
            variant={source === value ? 'default' : 'outline'}
            onClick={() => {
              setSource(value)
              setPage(1)
            }}
          >
            {value === 'wallet' ? t('Wallet bills') : t('Company quota bills')}
          </Button>
        ))}
        <Input
          className='w-56'
          aria-label={t('Model filter')}
          placeholder={t('Model filter')}
          value={model}
          onChange={(event) => {
            setModel(event.target.value)
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
            <tr className='bg-muted/30 border-b'>
              {[
                t('Time'),
                t('Username'),
                t('Department'),
                t('Model'),
                t('Tokens'),
                t('Amount'),
                t('Request ID'),
              ].map((label) => (
                <th key={label} className='p-3 whitespace-nowrap'>
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {(query.data?.items ?? []).map((log) => (
              <tr key={`${log.id}:${log.request_id}`} className='border-b'>
                <td className='p-3 whitespace-nowrap'>
                  {new Date(log.created_at * 1000).toLocaleString()}
                </td>
                <td className='p-3'>{log.username}</td>
                <td className='p-3'>
                  {String(
                    parseBillingOther(log).department_name || t('Unassigned')
                  )}
                </td>
                <td className='p-3'>{log.model_name}</td>
                <td className='p-3'>
                  {parseBillingOther(log).token_data_available === false
                    ? '-'
                    : log.prompt_tokens + log.completion_tokens}
                </td>
                <td className='p-3'>{formatQuota(log.quota)}</td>
                <td className='p-3 text-xs'>{log.request_id}</td>
              </tr>
            ))}
            {query.isSuccess && query.data.items.length === 0 && (
              <tr>
                <td colSpan={7} className='p-6 text-center'>
                  {t('No records')}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <div className='flex items-center gap-3'>
        <Button
          variant='outline'
          disabled={page === 1}
          onClick={() => setPage(page - 1)}
        >
          {t('Previous')}
        </Button>
        <span>
          {page} · {query.data?.total ?? 0}
        </span>
        <Button
          variant='outline'
          disabled={page * 20 >= (query.data?.total ?? 0)}
          onClick={() => setPage(page + 1)}
        >
          {t('Next')}
        </Button>
      </div>
    </div>
  )
}
