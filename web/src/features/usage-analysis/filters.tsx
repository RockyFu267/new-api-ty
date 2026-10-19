import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { ComboboxInput } from '@/components/ui/combobox-input'
import { Input } from '@/components/ui/input'

import { beijingDay, getFilterOptions, type Filters } from './api'

export function UsageFilters(props: {
  filters: Filters
  onApply: (filters: Filters) => void
  onReset: (filters: Filters) => void
}) {
  const { t } = useTranslation()
  const today = beijingDay()
  const [filters, setFilters] = useState(props.filters)
  function filter(key: keyof Filters, value: string) {
    setFilters((previous) => {
      const next = { ...previous, [key]: value, page: 1 }
      if (key === 'department_id' || key === 'department_basis') {
        next.user_id = ''
        next.model = ''
        next.dimension = next.department_id ? 'user' : 'department'
        if (value === 'current' && next.department_id === '-1') {
          next.department_id = ''
        }
      }
      if (key === 'user_id') next.model = ''
      return next
    })
  }
  const options = useQuery({
    queryKey: [
      'usage-analysis-options',
      filters.start,
      filters.end,
      filters.department_id,
      filters.department_basis,
      filters.user_id,
      filters.source,
      filters.local,
    ],
    queryFn: () => getFilterOptions(filters),
    placeholderData: keepPreviousData,
  })
  return (
    <form
      onSubmit={(event) => {
        event.preventDefault()
        props.onApply({ ...filters, page: 1 })
      }}
      className='grid gap-3 rounded-lg border p-4 sm:grid-cols-2 lg:grid-cols-4'
    >
      <label className='space-y-1 text-sm'>
        {t('Start date')}
        <Input
          type='date'
          max={today}
          value={filters.start}
          onChange={(e) => filter('start', e.target.value)}
        />
      </label>
      <label className='space-y-1 text-sm'>
        {t('End date')}
        <Input
          type='date'
          max={today}
          value={filters.end}
          onChange={(e) => filter('end', e.target.value)}
        />
      </label>
      <label className='space-y-1 text-sm'>
        {t('Department reporting basis')}
        <select
          className='bg-background h-9 w-full rounded-md border px-2'
          value={filters.department_basis ?? 'historical'}
          onChange={(event) => filter('department_basis', event.target.value)}
        >
          <option value='current'>
            {t('Current department members (including historical usage)')}
          </option>
          <option value='historical'>{t('Department at request time')}</option>
        </select>
      </label>
      <label className='space-y-1 text-sm'>
        {t('Department')}
        <select
          className='bg-background h-9 w-full rounded-md border px-2'
          value={filters.department_id}
          onChange={(e) => filter('department_id', e.target.value)}
        >
          <option value=''>{t('All departments')}</option>
          <option value='0'>{t('Unassigned department')}</option>
          {filters.department_basis !== 'current' && (
            <option value='-1'>{t('Historical department unknown')}</option>
          )}
          {options.data?.departments.map((d) => (
            <option key={d.value} value={d.value}>
              {d.label}
            </option>
          ))}
        </select>
      </label>
      <label className='space-y-1 text-sm'>
        {t('Token data source')}
        <select
          className='bg-background h-9 w-full rounded-md border px-2'
          value={filters.source ?? ''}
          onChange={(e) => filter('source', e.target.value)}
        >
          <option value=''>{t('All data sources')}</option>
          <option value='measured'>{t('Measured')}</option>
          <option value='estimated'>{t('Estimated')}</option>
          <option value='unknown'>{t('Unknown')}</option>
        </select>
      </label>
      <div className='space-y-1 text-sm'>
        <label htmlFor='usage-user'>{t('User')}</label>
        <ComboboxInput
          id='usage-user'
          value={filters.user_id}
          options={[
            { value: '', label: t('All users') },
            ...(options.data?.users ?? []),
          ]}
          onValueChange={(value) => filter('user_id', value)}
          placeholder={t('Search by name, account or ID')}
        />
      </div>
      <div className='space-y-1 text-sm'>
        <label htmlFor='usage-model'>{t('Model filter')}</label>
        <ComboboxInput
          id='usage-model'
          value={filters.model}
          options={[
            { value: '', label: t('All models') },
            ...(options.data?.models ?? []),
          ]}
          onValueChange={(value) => filter('model', value)}
          placeholder={t('Search and select a model')}
        />
      </div>
      <label className='space-y-1 text-sm'>
        {t('Model source')}
        <select
          className='bg-background h-9 w-full rounded-md border px-2'
          value={filters.local}
          onChange={(e) => filter('local', e.target.value)}
        >
          <option value=''>{t('All models')}</option>
          <option value='local'>{t('Local models')}</option>
          <option value='commercial'>{t('Other models')}</option>
        </select>
      </label>
      <Button
        type='submit'
        disabled={!filters.start || !filters.end || filters.start > filters.end}
      >
        {t('Search usage')}
      </Button>
      <Button
        type='button'
        variant='outline'
        onClick={() => {
          const cleared: Filters = {
            start: `${today.slice(0, 7)}-01`,
            end: today,
            department_id: '',
            department_basis: 'current',
            user_id: '',
            model: '',
            local: '',
            dimension: 'department',
            page: 1,
          }
          setFilters(cleared)
          props.onReset(cleared)
        }}
      >
        {t('Reset filters')}
      </Button>
      <p className='text-muted-foreground text-sm sm:col-span-2 lg:col-span-4'>
        {t('Choose filters, then click Search.')}{' '}
        {t(
          'Current department mode groups historical usage by current members without changing saved history. Request-time mode keeps historical unknown departments separate.'
        )}
      </p>
      {options.isError && (
        <div role='alert' className='sm:col-span-2 lg:col-span-4'>
          {t('Unable to load filter options')}{' '}
          <Button
            type='button'
            variant='outline'
            onClick={() => void options.refetch()}
          >
            {t('Retry')}
          </Button>
        </div>
      )}
    </form>
  )
}
