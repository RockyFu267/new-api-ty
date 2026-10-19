import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import type { Bucket } from './api'
import { formatTokenMillions } from './format'

export function UsageTable(props: {
  rows: Bucket[]
  dimension?: string
  savings?: boolean
  onSelect?: (id: string) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='overflow-x-auto rounded-lg border'>
      <table className='w-full text-left text-sm tabular-nums'>
        <thead className='bg-muted'>
          <tr>
            {[
              'Name',
              'Requests',
              'Input tokens',
              'Output tokens',
              'Cache read tokens',
              'Cache write tokens',
              'Cache reporting requests',
              'Measured requests',
              'Estimated requests',
              'Unknown usage requests',
            ].map((key) => (
              <th key={key} className='p-3 whitespace-nowrap'>
                {t(key)}
              </th>
            ))}
            {props.savings && (
              <th className='p-3 whitespace-nowrap'>
                {t('Estimated avoided commercial API expenditure')}
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {props.rows.map((row) => {
            let name = row.name
            let id = row.name
            if (
              props.dimension === 'user' ||
              props.dimension === 'department'
            ) {
              const index = row.name.indexOf('|')
              id = row.name.slice(0, index)
              name = row.name.slice(index + 1)
            }
            if (name === 'historical_unknown') {
              name = t('Historical department unknown')
            }
            if (name === 'unassigned') name = t('Unassigned department')
            return (
              <tr key={row.name} className='border-t'>
                <td className='p-3'>
                  {props.onSelect ? (
                    <Button variant='link' onClick={() => props.onSelect?.(id)}>
                      {name}
                    </Button>
                  ) : (
                    name
                  )}
                </td>
                {(
                  [
                    'requests',
                    'input',
                    'output',
                    'cache_read',
                    'cache_write',
                    'cache_reported',
                    'measured',
                    'estimated',
                    'unknown',
                  ] as const
                ).map((key) => {
                  const tokens =
                    key === 'input' ||
                    key === 'output' ||
                    key === 'cache_read' ||
                    key === 'cache_write'
                  let display = row[key].toLocaleString()
                  if (tokens) display = formatTokenMillions(row[key])
                  const missingCache =
                    (key === 'cache_read' || key === 'cache_write') &&
                    row.cache_reported === 0
                  if (missingCache) {
                    display = t('Not provided')
                  }
                  return (
                    <td
                      key={key}
                      className='p-3 whitespace-nowrap'
                      title={
                        tokens && !missingCache
                          ? `${row[key].toLocaleString()} tokens`
                          : undefined
                      }
                    >
                      {display}
                    </td>
                  )
                })}
                {props.savings && (
                  <td className='p-3 whitespace-nowrap'>
                    USD {row.avoided_usd.toFixed(4)}
                  </td>
                )}
              </tr>
            )
          })}
          {props.rows.length === 0 && (
            <tr>
              <td colSpan={props.savings ? 11 : 10} className='p-6 text-center'>
                {t('No data')}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}
