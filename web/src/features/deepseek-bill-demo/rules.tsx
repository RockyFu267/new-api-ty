import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { api } from '@/lib/api'

type Rates = { input: number; cache: number; output: number }
type Rule = {
  version: number
  created_by: number
  enabled: boolean
  effective_at: number
  fx: number
  peak: Rates
  off_peak: Rates
  windows: { start: number; end: number }[]
  holidays: string[]
}
type Store = { revision: number; rules: Rule[] }
type Config = {
  store: Store
  defaults: Rule
  active: { rule: Rule } | null
  model: string
}
type Fields = {
  enabled: boolean
  fx: number
  peak: Rates
  off_peak: Rates
  effective: string
  windows: string
  holidays: string
}

async function getConfig(): Promise<Config> {
  const response = await api.get('/api/user/deepseek-billing/rules')
  if (!response.data.success) throw new Error(response.data.message)
  return response.data.data as Config
}

export function DeepSeekPricingNotice() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['deepseek-billing-rules'],
    queryFn: getConfig,
  })
  if (!query.data?.active) return null
  return (
    <p className='rounded-lg border border-amber-300 p-3 text-sm'>
      {t(
        'deepseek-v4.1-flash uses dedicated DeepSeek billing. Ordinary model prices and group multipliers do not apply.'
      )}{' '}
      <Link className='underline' to='/deepseek-bill-demo'>
        {t('DeepSeek pricing rules')}
      </Link>
    </p>
  )
}

function clock(minute: number) {
  return `${String(Math.floor(minute / 60)).padStart(2, '0')}:${String(minute % 60).padStart(2, '0')}`
}
function parseWindows(value: string) {
  return value.split(',').map((part) => {
    const match = part.trim().match(/^(\d{2}):(\d{2})-(\d{2}):(\d{2})$/)
    if (
      !match ||
      Number(match[1]) > 23 ||
      Number(match[2]) > 59 ||
      Number(match[3]) > 24 ||
      Number(match[4]) > 59
    ) {
      throw new Error('Invalid peak windows')
    }
    return {
      start: Number(match[1]) * 60 + Number(match[2]),
      end: Number(match[3]) * 60 + Number(match[4]),
    }
  })
}

export function DeepSeekRules() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['deepseek-billing-rules'],
    queryFn: getConfig,
    refetchInterval: 30000,
  })
  if (query.isPending) return <p role='status'>{t('Loading')}</p>
  if (query.isError) return <p role='alert'>{query.error.message}</p>
  return <RulesEditor key={query.data.store.revision} config={query.data} />
}

function RulesEditor({ config }: { config: Config }) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const rule = config.store.rules.at(-1) ?? config.defaults
  const form = useForm<Fields>({
    defaultValues: {
      enabled: rule.enabled,
      fx: rule.fx,
      peak: rule.peak,
      off_peak: rule.off_peak,
      effective: '',
      windows: rule.windows
        .map((window) => `${clock(window.start)}-${clock(window.end)}`)
        .join(','),
      holidays: rule.holidays.join('\n'),
    },
  })
  const save = useMutation({
    mutationFn: async (values: Fields) => {
      const effective = values.effective
        ? new Date(`${values.effective}+08:00`).getTime() / 1000
        : 0
      const response = await api.put('/api/user/deepseek-billing/rules', {
        revision: config.store.revision,
        rule: {
          enabled: values.enabled,
          fx: values.fx,
          peak: values.peak,
          off_peak: values.off_peak,
          effective_at: effective,
          windows: parseWindows(values.windows),
          holidays: values.holidays.split(/[\s,]+/).filter(Boolean),
        },
      })
      if (!response.data.success) throw new Error(response.data.message)
      return response.data.data as Store
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ['deepseek-billing-rules'] })
    },
  })
  return (
    <section className='space-y-4 rounded-xl border p-4'>
      <h2 className='text-lg font-semibold'>{t('DeepSeek pricing rules')}</h2>
      <p className='text-sm'>
        {config.model} ·{' '}
        {config.active
          ? `${t('Dedicated billing active')} · v${config.active.rule.version}`
          : t('Ordinary model pricing is active')}
      </p>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Dedicated prices replace ordinary model prices and group multipliers. Existing department quotas and wallet fallback remain shared. Historical usage is never charged again.'
        )}
      </p>
      <form
        className='space-y-4'
        onSubmit={form.handleSubmit((values) => save.mutate(values))}
      >
        <label className='flex items-center gap-2 text-sm'>
          <input type='checkbox' {...form.register('enabled')} />
          {t('Enable dedicated DeepSeek billing')}
        </label>
        <div className='grid gap-4 sm:grid-cols-2'>
          <label className='space-y-1 text-sm'>
            {t('CNY per USD for quota conversion')}
            <Input
              type='number'
              step='any'
              min='.01'
              max='1000'
              required
              {...form.register('fx', {
                valueAsNumber: true,
                required: true,
                min: 0.01,
              })}
            />
          </label>
          <label className='space-y-1 text-sm'>
            {t('Effective time (Beijing; blank means now)')}
            <Input type='datetime-local' {...form.register('effective')} />
          </label>
        </div>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Unit prices are CNY per million tokens. FX is a configured accounting rate, not an automatically fetched market rate.'
          )}
        </p>
        <div className='grid gap-4 sm:grid-cols-2'>
          {(['peak', 'off_peak'] as const).map((period) => (
            <fieldset key={period} className='space-y-2 rounded-lg border p-3'>
              <legend>{period === 'peak' ? t('Peak') : t('Off-peak')}</legend>
              {(['input', 'cache', 'output'] as const).map((kind) => (
                <label key={kind} className='block space-y-1 text-sm'>
                  {t(
                    {
                      input: 'Uncached input',
                      cache: 'Cache read',
                      output: 'Output',
                    }[kind]
                  )}
                  <Input
                    type='number'
                    step='any'
                    min='.000001'
                    required
                    {...form.register(`${period}.${kind}`, {
                      valueAsNumber: true,
                      required: true,
                      min: 0.000001,
                    })}
                  />
                </label>
              ))}
            </fieldset>
          ))}
        </div>
        <label className='block space-y-1 text-sm'>
          {t('Weekday peak windows (Beijing)')}
          <Input
            required
            {...form.register('windows')}
            placeholder='09:00-12:00,14:00-18:00'
          />
        </label>
        <label className='block space-y-1 text-sm'>
          {t('Off-peak holiday dates (one YYYY-MM-DD per line)')}
          <textarea
            rows={5}
            className='bg-background block w-full rounded-md border p-2'
            {...form.register('holidays')}
          />
        </label>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Weekends are off-peak. New versions apply only to new requests after the effective time; running requests keep their original rule and FX.'
          )}
        </p>
        {save.isError && (
          <p role='alert' className='text-destructive'>
            {save.error.message}
          </p>
        )}
        <Button type='submit' disabled={save.isPending}>
          {t('Publish new pricing version')}
        </Button>
      </form>
      {config.store.rules.length > 0 && (
        <div className='overflow-auto'>
          <table className='w-full text-left text-sm'>
            <thead>
              <tr>
                <th>{t('Version')}</th>
                <th>{t('Effective time')}</th>
                <th>{t('Status')}</th>
                <th>CNY/USD</th>
                <th>{t('Peak')}</th>
                <th>{t('Off-peak')}</th>
              </tr>
            </thead>
            <tbody>
              {[...config.store.rules].reverse().map((version) => (
                <tr key={version.version} className='border-t'>
                  <td className='p-2'>v{version.version}</td>
                  <td>
                    {new Date(version.effective_at * 1000).toLocaleString(
                      'zh-CN',
                      { timeZone: 'Asia/Shanghai' }
                    )}
                  </td>
                  <td>{version.enabled ? t('Enabled') : t('Disabled')}</td>
                  <td>{version.fx}</td>
                  <td>
                    {version.peak.input} / {version.peak.cache} /{' '}
                    {version.peak.output}
                  </td>
                  <td>
                    {version.off_peak.input} / {version.off_peak.cache} /{' '}
                    {version.off_peak.output}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
