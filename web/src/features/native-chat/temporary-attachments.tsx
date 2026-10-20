import { nanoid } from 'nanoid'
import { useRef, useState, type ClipboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'

export type TemporaryAttachment = {
  id: string
  name: string
  text?: string
  image?: string
}
export const ATTACHMENT_ACCEPT =
  '.log,.txt,.md,image/png,image/jpeg,image/webp,image/gif'

export async function readTemporaryAttachment(
  file: File
): Promise<TemporaryAttachment> {
  const attachment = { id: nanoid(), name: file.name }
  if (file.size > 5 * 1024 * 1024) throw new Error('Attachment exceeds 5 MB')
  const buffer = await new Promise<ArrayBuffer>((resolve, reject) => {
    const reader = new FileReader()
    reader.addEventListener(
      'load',
      () => resolve(reader.result as ArrayBuffer),
      { once: true }
    )
    reader.addEventListener(
      'error',
      () => reject(new Error('Unable to read attachment')),
      { once: true }
    )
    reader.readAsArrayBuffer(file)
  })
  const bytes = new Uint8Array(buffer)
  if (/\.(log|txt|md)$/i.test(file.name)) {
    let text: string
    try {
      text = new TextDecoder('utf-8', { fatal: true }).decode(bytes)
    } catch {
      throw new Error('Use UTF-8 text files up to 100,000 characters')
    }
    if (text.includes('\0') || text.length > 100000) {
      throw new Error('Use UTF-8 text files up to 100,000 characters')
    }
    return { ...attachment, text }
  }
  let mime = ''
  if (
    bytes[0] === 137 &&
    bytes[1] === 80 &&
    bytes[2] === 78 &&
    bytes[3] === 71 &&
    bytes[4] === 13 &&
    bytes[5] === 10 &&
    bytes[6] === 26 &&
    bytes[7] === 10
  ) {
    mime = 'image/png'
  }
  if (bytes[0] === 255 && bytes[1] === 216 && bytes[2] === 255) {
    mime = 'image/jpeg'
  }
  if (/^GIF8[79]a$/.test(new TextDecoder().decode(bytes.slice(0, 6)))) {
    mime = 'image/gif'
  }
  if (
    new TextDecoder().decode(bytes.slice(0, 4)) === 'RIFF' &&
    new TextDecoder().decode(bytes.slice(8, 12)) === 'WEBP'
  ) {
    mime = 'image/webp'
  }
  if (!mime) {
    throw new Error('Only LOG, TXT, MD, PNG, JPEG, WebP and GIF are supported')
  }
  const image = await new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.addEventListener('load', () => resolve(String(reader.result)), {
      once: true,
    })
    reader.addEventListener(
      'error',
      () => reject(new Error('Unable to read attachment')),
      { once: true }
    )
    reader.readAsDataURL(new Blob([bytes], { type: mime }))
  })
  return { ...attachment, image }
}

export function useTemporaryAttachments() {
  const { t } = useTranslation()
  const input = useRef<HTMLInputElement>(null)
  const [items, setItems] = useState<TemporaryAttachment[]>([])
  const [reading, setReading] = useState(false)
  const generation = useRef(0)
  const busy = useRef(false)
  const clear = () => {
    generation.current++
    setItems([])
  }
  const add = async (files: File[]) => {
    if (busy.current) return
    if (files.length + items.length > 6) {
      toast.error(t('Maximum 6 attachments per message'))
      return
    }
    busy.current = true
    setReading(true)
    const current = generation.current
    try {
      const next = await Promise.all(files.map(readTemporaryAttachment))
      if (current === generation.current) {
        setItems((previous) => [...previous, ...next])
      }
    } catch (error) {
      toast.error(t('Unable to add attachment'), {
        description: error instanceof Error ? t(error.message) : '',
      })
    } finally {
      busy.current = false
      setReading(false)
    }
  }
  const onPaste = (event: ClipboardEvent<HTMLTextAreaElement>) => {
    const files = [...event.clipboardData.files]
    if (!files.length) return
    event.preventDefault()
    void add(files)
  }
  const panel = (
    <div className='space-y-2 px-3'>
      <input
        ref={input}
        type='file'
        multiple
        accept={ATTACHMENT_ACCEPT}
        className='hidden'
        aria-label={t('Temporary attachments')}
        onChange={(event) => {
          void add([...(event.target.files ?? [])])
          event.target.value = ''
        }}
      />
      <p className='text-muted-foreground text-xs'>
        {t(
          'Temporary attachments: LOG/TXT/MD (UTF-8), PNG/JPEG/WebP/GIF. Up to 6 files, 5 MB each. Images require a vision model.'
        )}
      </p>
      {reading && <p role='status'>{t('Reading attachments')}</p>}
      {items.map((item) => (
        <div
          key={item.id}
          className='flex items-center gap-2 rounded border p-2'
        >
          {item.image && (
            <img
              src={item.image}
              alt={item.name}
              className='size-14 object-contain'
            />
          )}
          <span className='min-w-0 flex-1 truncate'>{item.name}</span>
          <Button
            type='button'
            size='sm'
            variant='ghost'
            aria-label={`${t('Remove attachment')}: ${item.name}`}
            onClick={() =>
              setItems((previous) =>
                previous.filter((value) => value.id !== item.id)
              )
            }
          >
            {t('Remove')}
          </Button>
        </div>
      ))}
    </div>
  )
  return {
    items,
    reading,
    clear,
    onPaste,
    panel,
    open: () => input.current?.click(),
  }
}
