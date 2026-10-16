import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { getDepartments } from '@/features/users/api'
import { getCurrencyLabel } from '@/lib/currency'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'

import {
  loadCompanyData,
  saveCompanyData,
  type Page,
  type QuotaRule,
  type QuotaUsage,
} from './api'
import { CompanyBills } from './bills'
import { QuotaUsageCard } from './usage'

export function CompanyQuota() {
  const { t } = useTranslation()
  const client = useQueryClient()
  const [tab, setTab] = useState('rules')
  const [editor, setEditor] = useState<QuotaRule | null>(null)
  const [values, setValues] = useState({ daily: '', weekly: '', monthly: '' })
  const [department, setDepartment] = useState('')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<QuotaUsage | null>(null)
  const config = useQuery({
    queryKey: ['company-quota-config'],
    queryFn: () =>
      loadCompanyData<{ enabled: boolean }>('company-quota/config'),
  })
  const departments = useQuery({
    queryKey: ['departments'],
    queryFn: getDepartments,
  })
  const rules = useQuery({
    queryKey: ['company-quota-rules'],
    queryFn: () => loadCompanyData<QuotaRule[]>('company-quota/rules'),
  })
  const users = useQuery({
    queryKey: ['company-quota-users', department, keyword, page],
    queryFn: () =>
      loadCompanyData<Page<QuotaUsage>>('company-quota/users', {
        department_id: department,
        keyword,
        p: page,
        page_size: 20,
      }),
    enabled: tab === 'users',
  })
  const toggle = useMutation({
    mutationFn: (enabled: boolean) =>
      saveCompanyData('company-quota/config', { enabled }),
    onSuccess: () =>
      client.invalidateQueries({ queryKey: ['company-quota-config'] }),
  })
  const save = useMutation({
    mutationFn: () => {
      if (!editor) throw new Error('No department selected')
      const limits = Object.fromEntries(
        Object.entries(values).map(([period, value]) => [
          period,
          value.trim() ? parseQuotaFromDollars(Number(value)) : 0,
        ])
      )
      return saveCompanyData('company-quota/rules', { ...editor, ...limits })
    },
    onSuccess: async () => {
      setEditor(null)
      await Promise.all([
        client.invalidateQueries({ queryKey: ['company-quota-rules'] }),
        client.invalidateQueries({ queryKey: ['company-quota-users'] }),
        client.invalidateQueries({ queryKey: ['company-quota-self'] }),
      ])
    },
  })
  const openRule = (id: number) => {
    const rule = rules.data?.find((item) => item.department_id === id) ?? {
      department_id: id,
      enabled: false,
      daily: 0,
      weekly: 0,
      monthly: 0,
    }
    setEditor(rule)
    setValues({
      daily: rule.daily ? String(quotaUnitsToDollars(rule.daily)) : '',
      weekly: rule.weekly ? String(quotaUnitsToDollars(rule.weekly)) : '',
      monthly: rule.monthly ? String(quotaUnitsToDollars(rule.monthly)) : '',
    })
  }
  return (
    <div className='space-y-5 p-6'>
      <h1 className='text-2xl font-semibold'>{t('Department quotas')}</h1>
      <div className='flex items-center gap-3 rounded-lg border p-4'>
        <Switch
          aria-label={t('Enable enterprise quota mode')}
          checked={config.data?.enabled ?? false}
          disabled={toggle.isPending || !config.isSuccess}
          onCheckedChange={(enabled) => toggle.mutate(enabled)}
        />
        <div>
          <p>{t('Enable enterprise quota mode')}</p>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Company quota first, wallet fallback. Turning this off restores existing billing preferences.'
            )}
          </p>
        </div>
      </div>
      <p className='text-muted-foreground text-sm'>
        {t(
          'In-flight requests may exceed the estimate; actual consumption is always recorded.'
        )}
      </p>
      <div className='flex gap-2'>
        {['rules', 'users'].map((value) => (
          <Button
            key={value}
            variant={tab === value ? 'default' : 'outline'}
            onClick={() => setTab(value)}
          >
            {value === 'rules' ? t('Department rules') : t('User usage')}
          </Button>
        ))}
      </div>
      {config.isError && (
        <Button onClick={() => void config.refetch()}>{t('Retry')}</Button>
      )}
      {tab === 'rules' && (
        <div className='space-y-3'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Limits apply to each person, not a shared department pool. Blank periods are unlimited; no configured periods means no company quota.'
            )}
          </p>
          {departments.isError || rules.isError ? (
            <Button
              onClick={() => {
                void departments.refetch()
                void rules.refetch()
              }}
            >
              {t('Retry')}
            </Button>
          ) : null}
          {(departments.data ?? []).map((item) => {
            const rule = rules.data?.find(
              (entry) => entry.department_id === item.id
            )
            return (
              <div
                key={item.id}
                className='flex flex-wrap items-center gap-4 rounded-lg border p-4'
              >
                <span className='flex-1 font-medium'>
                  {item.name} · {item.user_count}
                </span>
                <span>
                  {rule?.enabled ? t('Enabled') : t('Not configured')}
                </span>
                {(['daily', 'weekly', 'monthly'] as const).map((period) => (
                  <span key={period}>
                    {t(period)}:{' '}
                    {rule?.[period] ? formatQuota(rule[period]) : '—'}
                  </span>
                ))}
                <Button
                  variant='outline'
                  disabled={!rules.isSuccess}
                  onClick={() => openRule(item.id)}
                >
                  {t('Edit')}
                </Button>
              </div>
            )
          })}
          {departments.isSuccess && departments.data.length === 0 && (
            <p>{t('Create departments in user management first.')}</p>
          )}
        </div>
      )}
      {tab === 'users' && (
        <div className='space-y-3'>
          <div className='flex flex-wrap gap-2'>
            <select
              className='bg-background rounded-md border p-2'
              aria-label={t('Department filter')}
              value={department}
              onChange={(event) => {
                setDepartment(event.target.value)
                setPage(1)
              }}
            >
              <option value=''>{t('All departments')}</option>
              <option value='0'>{t('Unassigned')}</option>
              {(departments.data ?? []).map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
            <Input
              className='w-60'
              aria-label={t('User filter')}
              placeholder={t('User filter')}
              value={keyword}
              onChange={(event) => {
                setKeyword(event.target.value)
                setPage(1)
              }}
            />
          </div>
          {users.isError && (
            <Button onClick={() => void users.refetch()}>{t('Retry')}</Button>
          )}
          {(users.data?.items ?? []).map((item) => (
            <div key={item.user_id} className='space-y-2'>
              <Button variant='outline' onClick={() => setSelected(item)}>
                {item.username} · {t('View bills')}
              </Button>
              <QuotaUsageCard
                usage={item}
                enabled={config.data?.enabled ?? false}
              />
            </div>
          ))}
          {users.isSuccess && users.data.items.length === 0 && (
            <p>{t('No records')}</p>
          )}
          <div className='flex gap-3'>
            <Button disabled={page === 1} onClick={() => setPage(page - 1)}>
              {t('Previous')}
            </Button>
            <span>
              {page} · {users.data?.total ?? 0}
            </span>
            <Button
              disabled={page * 20 >= (users.data?.total ?? 0)}
              onClick={() => setPage(page + 1)}
            >
              {t('Next')}
            </Button>
          </div>
        </div>
      )}
      <Dialog
        open={editor !== null}
        onOpenChange={(open) => {
          if (!open && !save.isPending) setEditor(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Department quota rule')}</DialogTitle>
            <DialogDescription>
              {t(
                'Rules apply immediately. Existing consumption is preserved. Reset at Beijing midnight, Sunday, and the first day of each month.'
              )}
            </DialogDescription>
          </DialogHeader>
          <form
            className='space-y-4'
            onSubmit={(event) => {
              event.preventDefault()
              save.mutate()
            }}
          >
            <div className='flex items-center gap-2'>
              <Switch
                aria-label={t('Enable department quota')}
                checked={editor?.enabled ?? false}
                onCheckedChange={(enabled) =>
                  setEditor((current) =>
                    current ? { ...current, enabled } : null
                  )
                }
              />
              <span>{t('Enable department quota')}</span>
            </div>
            {(['daily', 'weekly', 'monthly'] as const).map((period) => (
              <label key={period} className='block space-y-1'>
                <span>
                  {t(period)} ({getCurrencyLabel()})
                </span>
                <Input
                  type='number'
                  min='0.000001'
                  step='any'
                  value={values[period]}
                  placeholder={t('Blank means no limit')}
                  onChange={(event) =>
                    setValues((current) => ({
                      ...current,
                      [period]: event.target.value,
                    }))
                  }
                />
              </label>
            ))}
            <Button
              type='submit'
              disabled={
                save.isPending ||
                (editor?.enabled &&
                  !Object.values(values).some((value) => Number(value) > 0))
              }
            >
              {t('Save')}
            </Button>
          </form>
        </DialogContent>
      </Dialog>
      <Dialog
        open={selected !== null}
        onOpenChange={(open) => {
          if (!open) setSelected(null)
        }}
      >
        <DialogContent className='max-h-[85vh] overflow-auto sm:max-w-5xl'>
          <DialogHeader>
            <DialogTitle>
              {selected?.username} · {t('Bills')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'Company quota and wallet consumption are recorded separately.'
              )}
            </DialogDescription>
          </DialogHeader>
          {selected && <CompanyBills admin userId={selected.user_id} />}
        </DialogContent>
      </Dialog>
    </div>
  )
}
