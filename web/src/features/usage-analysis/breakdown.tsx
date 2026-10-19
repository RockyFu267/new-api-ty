import { useNavigate } from '@tanstack/react-router'
import { useState, type Dispatch, type SetStateAction } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import type { Report, Filters } from './api'
import { UsageTable } from './usage-table'

export function UsageBreakdown(props: {
  data: Report
  filters: Filters
  savings: boolean
  onChange: Dispatch<SetStateAction<Filters>>
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [selectedUser, setSelectedUser] = useState<{
    id: string
    name: string
  } | null>(null)
  const data = props.data
  const filters = props.filters
  const selectedUsername =
    selectedUser?.id === filters.user_id ? selectedUser.name : ''
  const setFilters = props.onChange
  function filter(key: keyof Filters, value: string) {
    props.onChange({ ...filters, [key]: value, page: 1 })
  }
  function drilldown(id: string) {
    if (filters.dimension === 'department') {
      setFilters((previous) => ({
        ...previous,
        department_id: id,
        dimension: 'user',
        page: 1,
      }))
    }
    if (filters.dimension === 'user') {
      const row = data?.rows.find((r) => r.name.startsWith(`${id}|`))
      setSelectedUser({
        id,
        name: row?.name.slice(row.name.indexOf('|') + 1) ?? '',
      })
      setFilters((previous) => ({
        ...previous,
        user_id: id,
        dimension: 'model',
        page: 1,
      }))
    }
    if (filters.dimension === 'model') {
      props.onChange({ ...filters, model: id, page: 1 })
    }
  }
  return (
    <>
      <div className='flex gap-2'>
        {['department', 'user', 'model'].map((dimension) => (
          <Button
            key={dimension}
            variant={filters.dimension === dimension ? 'default' : 'outline'}
            onClick={() => filter('dimension', dimension)}
          >
            {t(
              {
                department: 'By department',
                user: 'By user',
                model: 'By model',
              }[dimension] ?? dimension
            )}
          </Button>
        ))}
      </div>
      <UsageTable
        rows={data.rows}
        dimension={filters.dimension}
        onSelect={drilldown}
        savings={props.savings && filters.dimension === 'model'}
      />
      {selectedUsername && filters.user_id && (
        <Button
          variant='outline'
          onClick={() =>
            void navigate({
              to: '/usage-logs/$section',
              params: { section: 'common' },
              search: {
                username: selectedUsername,
                model: filters.model,
                type: ['2'],
                startTime: new Date(
                  `${filters.start}T00:00:00+08:00`
                ).getTime(),
                endTime: new Date(`${filters.end}T23:59:59+08:00`).getTime(),
              },
            })
          }
        >
          {t('View request logs')}
        </Button>
      )}
      <div className='flex items-center gap-3'>
        <Button
          variant='outline'
          disabled={filters.page === 1}
          onClick={() => setFilters((f) => ({ ...f, page: f.page - 1 }))}
        >
          {t('Previous')}
        </Button>
        <span>
          {filters.page} / {Math.max(1, Math.ceil(data.row_count / 20))}
        </span>
        <Button
          variant='outline'
          disabled={filters.page * 20 >= data.row_count}
          onClick={() => setFilters((f) => ({ ...f, page: f.page + 1 }))}
        >
          {t('Next')}
        </Button>
      </div>
    </>
  )
}
