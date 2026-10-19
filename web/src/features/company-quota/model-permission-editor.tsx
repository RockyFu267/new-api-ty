import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'

import { saveCompanyData } from './api'

export function ModelPermissionEditor(props: {
  department: { id: number; name: string }
  initial: string[]
  catalog: string[]
  onClose: () => void
}) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const [selected, setSelected] = useState(props.initial)
  const [search, setSearch] = useState('')
  const all = [...new Set([...props.catalog, ...props.initial])].sort()
  const visible = all.filter((name) =>
    name.toLowerCase().includes(search.trim().toLowerCase())
  )
  const save = useMutation({
    mutationFn: () =>
      saveCompanyData('company-quota/models', {
        department_id: props.department.id,
        models: selected,
      }),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: ['department-model-policies'] }),
        client.invalidateQueries({ queryKey: ['user-models'] }),
        client.invalidateQueries({ queryKey: ['native-chat-channel-models'] }),
      ])
      props.onClose()
    },
  })
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) props.onClose()
      }}
    >
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>
            {props.department.name} · {t('Model permissions')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Only selected models are allowed. Saving an empty list denies every model. Administrators follow the same policy.'
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
          <Input
            disabled={save.isPending}
            aria-label={t('Search models')}
            placeholder={t('Search models')}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
          <div className='flex flex-wrap items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              disabled={save.isPending}
              onClick={() =>
                setSelected([...new Set([...selected, ...visible])])
              }
            >
              {t('Select search results')}
            </Button>
            <Button
              type='button'
              variant='outline'
              disabled={save.isPending}
              onClick={() => setSelected([])}
            >
              {t('Clear all models')}
            </Button>
            <span className='text-sm'>
              {t('Authorized models')}: {selected.length}
            </span>
          </div>
          <div className='max-h-80 space-y-2 overflow-y-auto rounded-lg border p-3'>
            {visible.map((name) => (
              <label
                key={name}
                className='flex items-center gap-3 text-sm break-all'
              >
                <Checkbox
                  disabled={save.isPending}
                  checked={selected.includes(name)}
                  onCheckedChange={(checked) =>
                    setSelected((previous) =>
                      checked
                        ? [...new Set([...previous, name])]
                        : previous.filter((model) => model !== name)
                    )
                  }
                />
                <span>
                  {name}
                  {!props.catalog.includes(name) && (
                    <span className='text-muted-foreground'>
                      {' '}
                      · {t('Not currently configured in channels')}
                    </span>
                  )}
                </span>
              </label>
            ))}
            {visible.length === 0 && <p>{t('No matching models')}</p>}
          </div>
          {selected.length === 0 && (
            <p role='status' className='text-destructive text-sm'>
              {t('This department will be unable to call any model.')}
            </p>
          )}
          {save.isError && (
            <p role='alert' className='text-destructive'>
              {t('Unable to save model permissions')}: {save.error.message}
            </p>
          )}
          <Button type='submit' disabled={save.isPending}>
            {t('Save model permissions')}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
