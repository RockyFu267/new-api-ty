import { createFileRoute } from '@tanstack/react-router'

import { DeepSeekBillDemo } from '@/features/deepseek-bill-demo'

export const Route = createFileRoute('/_authenticated/deepseek-bill-demo/')({
  component: DeepSeekBillDemo,
})
