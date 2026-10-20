import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

export function ChatContextSection(props: {
  title: string
  count: number
  children: ReactNode
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(true)
  return (
    <section className='shrink-0 border-b'>
      <Button
        type='button'
        variant='ghost'
        className='w-full justify-between'
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <span>
          {props.title} · {props.count}
        </span>
        <span>{open ? t('Collapse') : t('Expand')}</span>
      </Button>
      <div hidden={!open}>{props.children}</div>
    </section>
  )
}
