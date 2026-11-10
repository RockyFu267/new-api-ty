import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { millions, yuan, type Report } from './api'

function unitPrices(row: Report['details'][number]) {
  if (row.actual) {
    const rates = row.actual.snapshot.rates
    return `${rates.input} / ${rates.cache} / ${rates.output}`
  }
  return row.peak ? '2 / 0.04 / 8' : '1 / 0.02 / 4'
}

export function BillUsers(props: {
  report: Report
  selected: string
  onSelect: (id: string) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='overflow-auto rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            {[
              t('User'),
              t('Requests'),
              t('Input'),
              t('Cache read'),
              t('Output'),
              t('Peak cost'),
              t('Off-peak cost'),
              t('Total (actual + historical simulation)'),
              t('Estimated records'),
            ].map((label) => (
              <TableHead key={label}>{label}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.report.users.map((user) => (
            <TableRow
              key={user.user_id}
              className={
                props.selected === String(user.user_id) ? 'bg-muted' : ''
              }
            >
              <TableCell>
                <Button
                  variant='link'
                  onClick={() => props.onSelect(String(user.user_id))}
                >
                  {user.username} · #{user.user_id}
                </Button>
              </TableCell>
              <TableCell>{user.requests.toLocaleString()}</TableCell>
              <TableCell>{millions(user.input)}</TableCell>
              <TableCell>{millions(user.cache_read)}</TableCell>
              <TableCell>{millions(user.output)}</TableCell>
              <TableCell>{yuan(user.peak_units)}</TableCell>
              <TableCell>{yuan(user.off_peak_units)}</TableCell>
              <TableCell className='font-semibold'>
                {yuan(user.peak_units + user.off_peak_units)}
              </TableCell>
              <TableCell>
                {user.estimated} / {user.requests}
              </TableCell>
            </TableRow>
          ))}
          {props.report.users.length === 0 && (
            <TableRow>
              <TableCell colSpan={9} className='py-10 text-center'>
                {t('No users used this model in the selected period')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}

export function BillDetails(props: {
  report: Report
  page: number
  onPage: (page: number) => void
}) {
  const { t } = useTranslation()
  return (
    <section className='space-y-3'>
      <div className='flex items-center justify-between'>
        <h2 className='text-lg font-semibold'>
          {t('Billing details')} · {props.report.detail_total.toLocaleString()}
        </h2>
        <div className='flex items-center gap-3'>
          <Button
            variant='outline'
            disabled={props.page <= 1}
            onClick={() => props.onPage(props.page - 1)}
          >
            {t('Previous')}
          </Button>
          <span>
            {props.page} /{' '}
            {Math.max(1, Math.ceil(props.report.detail_total / 50))}
          </span>
          <Button
            variant='outline'
            disabled={props.page * 50 >= props.report.detail_total}
            onClick={() => props.onPage(props.page + 1)}
          >
            {t('Next')}
          </Button>
        </div>
      </div>
      <div className='overflow-auto rounded-xl border'>
        <Table>
          <TableHeader>
            <TableRow>
              {[
                t('Beijing time'),
                t('User'),
                t('Billing period'),
                t('Uncached input'),
                t('Cache read'),
                t('Output'),
                t('Unit prices (CNY/M)'),
                t('Cost / billing source'),
                t('Data quality'),
              ].map((label) => (
                <TableHead key={label}>{label}</TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.report.details.map((row) => {
              let quality = t('Reported usage')
              if (row.estimated) quality = t('Estimated')
              if (row.unpriced) quality = t('Unpriced')
              return (
                <TableRow key={row.id}>
                  <TableCell className='whitespace-nowrap'>
                    <div>
                      {new Intl.DateTimeFormat('zh-CN', {
                        timeZone: 'Asia/Shanghai',
                        year: 'numeric',
                        month: '2-digit',
                        day: '2-digit',
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                        hour12: false,
                      }).format(new Date(row.time * 1000))}
                    </div>
                    <div
                      className='text-muted-foreground max-w-56 truncate text-xs'
                      title={row.request_id}
                    >
                      {row.request_id || `#${row.id}`}
                    </div>
                  </TableCell>
                  <TableCell>{row.username}</TableCell>
                  <TableCell>{row.peak ? t('Peak') : t('Off-peak')}</TableCell>
                  <TableCell>
                    {millions(Math.max(0, row.input - row.cache_read))}
                  </TableCell>
                  <TableCell>{millions(row.cache_read)}</TableCell>
                  <TableCell>{millions(row.output)}</TableCell>
                  <TableCell className='whitespace-nowrap'>
                    {unitPrices(row)}
                  </TableCell>
                  <TableCell>
                    {row.unpriced ? '—' : yuan(row.cost_units)}
                    <div className='text-muted-foreground text-xs'>
                      {row.actual
                        ? `${t('Actual billing')} · v${row.actual.snapshot.rule.version} · CNY/USD ${row.actual.snapshot.rule.fx} · $${(row.actual.quota / 500000).toFixed(6)}`
                        : t('Historical simulation')}
                    </div>
                  </TableCell>
                  <TableCell>{quality}</TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
    </section>
  )
}
