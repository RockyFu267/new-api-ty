import { useTranslation } from 'react-i18next'

import type { Report } from './api'

export function BillingReportStatus({ status }: { status: Report['status'] }) {
  const { t } = useTranslation()
  if (!status) return null

  let message: string
  if (!status.configured) {
    message = t(
      'Background billing reports are not configured yet. No live log scan will be started.'
    )
  } else if (!status.worker_running) {
    message = t(
      'Background reporting is paused. Showing the last generated bill.'
    )
  } else if (!status.history_complete) {
    message = t(
      'Billing history is being prepared in the background. AI requests do not wait for it.'
    )
  } else {
    message = t(
      'Bills are generated in the background and may be delayed. Refresh only reads generated results.'
    )
  }
  const progress =
    status.source_through > 0
      ? Math.min(
          100,
          Math.max(
            0,
            Math.floor((status.imported_through / status.source_through) * 100)
          )
        )
      : 0

  return (
    <section role='status' className='bg-muted rounded-xl p-4 text-sm'>
      <p>{message}</p>
      {status.configured && !status.history_complete && (
        <p>
          {t('Approximate history preparation progress')}: {progress}%
        </p>
      )}
      {status.last_sync > 0 && (
        <p>
          <span>{t('Report updated at')}</span>:{' '}
          {new Intl.DateTimeFormat('zh-CN', {
            timeZone: 'Asia/Shanghai',
            dateStyle: 'short',
            timeStyle: 'medium',
          }).format(new Date(status.last_sync * 1000))}
        </p>
      )}
    </section>
  )
}
