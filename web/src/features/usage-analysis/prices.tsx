import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { beijingDay, createPrice, type Price } from './api'

const priceSchema = z.object({
  local_model: z.string().trim().min(1).max(255),
  reference_model: z.string().trim().min(1).max(255),
  effective_day: z.string().regex(/^\d{4}-\d{2}-\d{2}$/),
  input_price: z.number().min(0).max(1_000_000),
  output_price: z.number().min(0).max(1_000_000),
})

export function ComparisonPrices(props: { prices: Price[] }) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const root = (useAuthStore((s) => s.auth.user?.role) ?? 0) >= ROLE.SUPER_ADMIN
  const form = useForm<z.infer<typeof priceSchema>>({
    resolver: zodResolver(priceSchema),
    defaultValues: {
      local_model: '',
      reference_model: '',
      effective_day: beijingDay(),
      input_price: 0,
      output_price: 0,
    },
  })
  const save = useMutation({
    mutationFn: createPrice,
    onSuccess: () => client.invalidateQueries({ queryKey: ['usage-analysis'] }),
  })
  return (
    <div className='space-y-4'>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Comparison prices are USD per million tokens. New versions take effect today or later and cannot overwrite history. Configured models are classified as local models.'
        )}
      </p>
      {root && (
        <form
          onSubmit={form.handleSubmit((values) => save.mutate(values))}
          className='grid gap-3 rounded-lg border p-4 md:grid-cols-3'
        >
          {(
            [
              'local_model',
              'reference_model',
              'effective_day',
              'input_price',
              'output_price',
            ] as const
          ).map((key) => {
            const labels = {
              local_model: 'Local model name',
              reference_model: 'Commercial comparison model',
              effective_day: 'Effective date',
              input_price: 'Input USD per million tokens',
              output_price: 'Output USD per million tokens',
            }
            const numeric = key === 'input_price' || key === 'output_price'
            let type = 'text'
            let min: number | string | undefined
            if (numeric) {
              type = 'number'
              min = 0
            }
            if (key === 'effective_day') {
              type = 'date'
              min = beijingDay()
            }
            return (
              <label key={key} className='space-y-1 text-sm'>
                <span>{t(labels[key])}</span>
                <Input
                  type={type}
                  step={numeric ? 'any' : undefined}
                  min={min}
                  {...form.register(key, { valueAsNumber: numeric })}
                />
                {form.formState.errors[key] && (
                  <span className='text-destructive'>{t('Invalid value')}</span>
                )}
              </label>
            )
          })}
          <Button type='submit' disabled={save.isPending}>
            {t('Add price version')}
          </Button>
          {save.isError && (
            <p role='alert' className='text-destructive'>
              {t('Unable to save comparison price')}: {save.error.message}
            </p>
          )}
          {save.isSuccess && <p role='status'>{t('Saved successfully')}</p>}
        </form>
      )}
      <div className='overflow-x-auto rounded-lg border'>
        <table className='w-full text-left text-sm'>
          <thead>
            <tr>
              {[
                'Local model name',
                'Commercial comparison model',
                'Effective date',
                'Input USD per million tokens',
                'Output USD per million tokens',
              ].map((key) => (
                <th key={key} className='p-3'>
                  {t(key)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {props.prices.map((p) => (
              <tr key={p.id} className='border-t'>
                <td className='p-3'>{p.local_model}</td>
                <td className='p-3'>{p.reference_model}</td>
                <td className='p-3'>{p.effective_day}</td>
                <td className='p-3'>{p.input_price}</td>
                <td className='p-3'>{p.output_price}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
